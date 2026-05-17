package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"dash/internal/alert"
	"dash/internal/config"
	"dash/internal/infra"
	"dash/internal/migrate"
	"dash/internal/store"
	themefs "dash/internal/theme"
	trafficservice "dash/internal/traffic"
	transporthttp "dash/internal/transport/http"
	httpapi "dash/internal/transport/http/api"
	"dash/internal/version"
	authhttp "github.com/Ithildur/EiluneKit/auth/http"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	authstore "github.com/Ithildur/EiluneKit/auth/store"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version.CurrentString())
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrate(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "pack-theme" {
		runPackTheme(os.Args[2:])
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var debug bool
	var noRedis bool
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.BoolVar(&noRedis, "no-redis", false, "run without redis")
	flag.Parse()

	initialLevel := os.Getenv("APP_LOG_LEVEL")
	initialFormat := os.Getenv("APP_LOG_FORMAT")
	if _, err := infra.InitLogger(initialLevel, initialFormat); err != nil {
		_, _ = infra.InitLogger("info", "text")
		infra.Log().Warn("init logger from env failed", err)
	}

	cfg, warnings, err := config.LoadWithWarnings("")
	if err != nil {
		infra.Fatal("load config failed", err)
	}

	logLevel := cfg.App.LogLevel
	if debug {
		logLevel = "debug"
	}
	if _, err := infra.InitLogger(logLevel, cfg.App.LogFormat); err != nil {
		infra.Fatal("init logger failed", err)
	}
	logger := infra.Log()
	for _, w := range warnings {
		logger.Warn(w.Msg, w.Err, w.Attrs...)
	}

	adminPassword := cfg.Auth.Password
	if err := authhttp.ValidateStaticPassword(adminPassword); err != nil {
		infra.Fatal("admin password is invalid for admin login", err, slog.String("env", config.EnvAdminPassword))
	}

	if debug {
		logger.Info("debug logging enabled", nil)
		if dumped, err := yaml.Marshal(config.RedactedForLog(cfg)); err == nil {
			logger.Debug("config after env overrides", nil, slog.String("config", string(dumped)))
		} else {
			logger.Warn("config dump failed", err)
		}
	}

	db, err := infra.NewGORMTimescale(ctx, cfg.Database)
	if err != nil {
		infra.Fatal("init timescale failed",
			err,
			slog.String("host", cfg.Database.Host),
			slog.Int("port", cfg.Database.Port),
			slog.String("database", cfg.Database.Name))
	}
	sqlDB, err := db.DB()
	if err != nil {
		infra.Fatal("extract sql.DB failed", err)
	}
	defer sqlDB.Close()
	if err := migrate.SyncRetentionPolicies(ctx, db, cfg.Database.EffectiveRetentionDays(), cfg.Database.EffectiveTrafficRetentionDays()); err != nil {
		infra.Fatal("sync retention policies failed", err)
	}

	var redisClient = (*redis.Client)(nil)
	if !noRedis {
		redisClient, err = infra.NewRedisClient(cfg.Redis)
		if err != nil {
			infra.Fatal("init redis failed",
				err,
				slog.String("addr", cfg.Redis.Addr),
				slog.Int("db", cfg.Redis.DB))
		}
		pingCtx, pingCancel := context.WithTimeout(ctx, config.RedisFetchTimeout)
		if err := infra.PingRedis(pingCtx, redisClient); err != nil {
			pingCancel()
			infra.Fatal("ping redis failed",
				err,
				slog.String("addr", cfg.Redis.Addr),
				slog.Int("db", cfg.Redis.DB))
		}
		pingCancel()
		defer redisClient.Close()
	} else {
		logger.Warn("redis disabled by startup flag", nil)
	}

	st := store.New(db, redisClient)
	if err := st.Validate(); err != nil {
		infra.Fatal("init store failed", err)
	}
	themeRoot, err := config.ThemeRootDir()
	if err != nil {
		infra.Fatal("resolve theme root failed", err)
	}
	themeStore, err := themefs.NewStore(themeRoot)
	if err != nil {
		infra.Fatal("init theme store failed", err, slog.String("root", themeRoot))
	}
	if _, err := st.Node.EnsureDefaultGroup(ctx); err != nil {
		infra.Fatal("ensure default group failed", err)
	}
	if _, err := infra.WithPGReadTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.Node.RebuildServerCache(c)
	}); err != nil {
		infra.Fatal("rebuild server cache failed", err)
	}
	var tokenStore authstore.SessionStore
	if redisClient != nil {
		tokenStore = authstore.NewRedisStore(redisClient, authstore.RedisOptions{
			Prefix:       "auth:jwt:",
			ReadTimeout:  cfg.Redis.ReadTimeoutDur,
			WriteTimeout: cfg.Redis.WriteTimeoutDur,
		})
	} else {
		tokenStore = authstore.NewMemoryStore()
	}
	jwtOpts := authjwt.DefaultManagerOptions()
	jwtOpts.Issuer = "dash"
	jwtOpts.Audience = "dash_front"
	jwtAuth, err := authjwt.NewWithOptions(cfg.Auth.JWTSigningKey, tokenStore, jwtOpts)
	if err != nil {
		infra.Fatal("init jwt auth failed",
			err,
			slog.Bool("signing_key_set", cfg.Auth.JWTSigningKey != ""))
	}
	deps := httpapi.Dependencies{Stores: st, Auth: jwtAuth, Theme: themeStore}

	srv, err := transporthttp.NewHTTPServer(cfg, deps)
	if err != nil {
		infra.Fatal("init http server failed", err)
	}
	alertService := alert.NewService(st.Alert, st.Front, alert.WithMessageConfig(alert.MessageConfig{
		Language: cfg.App.EffectiveLanguage(),
		Location: cfg.App.EffectiveLocation(),
	}))
	trafficService := trafficservice.NewService(st.Traffic, cfg.App.EffectiveLocation(), cfg.Database.EffectiveTrafficRetentionDays())

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return srv.Run(groupCtx) })
	group.Go(func() error { return alertService.Run(groupCtx) })
	group.Go(func() error { return trafficService.Run(groupCtx) })

	if err := group.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		infra.Fatal("runtime failed", err)
	}
}
