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
	"dash/internal/dashupdate"
	"dash/internal/infra"
	"dash/internal/infra/cachekeys"
	"dash/internal/migrate"
	"dash/internal/notify"
	"dash/internal/store"
	themefs "dash/internal/theme"
	trafficservice "dash/internal/traffic"
	transporthttp "dash/internal/transport/http"
	httpapi "dash/internal/transport/http/api"
	"dash/internal/version"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	authstore "github.com/Ithildur/EiluneKit/auth/store"
	"github.com/Ithildur/EiluneKit/auth/store/redissession"
	kitmigration "github.com/Ithildur/EiluneKit/postgres/migration"
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
	if len(os.Args) > 1 && os.Args[1] == "update" {
		os.Exit(runUpdate(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "check-redis" {
		os.Exit(runRedisCheck(os.Args[2:], os.Stdout, os.Stderr))
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

	cfg, err := config.LoadRuntime("", !noRedis)
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
	adminPassword := cfg.Auth.Password
	if err := config.ValidateAdminPassword(adminPassword); err != nil {
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
	if err := kitmigration.RequireCurrent(ctx, migrate.New(sqlDB, "")); err != nil {
		infra.Fatal("validate database schema failed", err)
	}
	notifyKeyPath, err := config.NotifyConfigKeyPath()
	if err != nil {
		infra.Fatal("resolve notification config key path failed", err)
	}
	configCipher, err := notify.LoadConfigCipher(notifyKeyPath)
	if err != nil {
		infra.Fatal("load notification config key failed", err, slog.String("path", notifyKeyPath))
	}
	if err := migrate.CheckNotifyConfigs(ctx, db, configCipher); err != nil {
		infra.Fatal("validate encrypted notification configs failed", err)
	}
	if err := migrate.SyncRetentionPolicies(
		ctx,
		db,
		cfg.Database.EffectiveMetricsRawRetentionDays(),
		cfg.Database.EffectiveRetentionDays(),
		cfg.Database.EffectiveTrafficRetentionDays(),
	); err != nil {
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
		checkTimeout := cfg.Redis.DialTimeoutDur + cfg.Redis.ReadTimeoutDur
		redisVersion, checkErr := infra.CheckRedis(ctx, redisClient, checkTimeout)
		if checkErr != nil {
			infra.Fatal("validate redis failed",
				checkErr,
				slog.String("addr", cfg.Redis.Addr),
				slog.Int("db", cfg.Redis.DB))
		}
		recommended, compareErr := version.Compare(redisVersion, infra.RedisRecommendedVersion)
		if compareErr != nil {
			infra.Fatal("compare redis version failed", compareErr, slog.String("version", redisVersion))
		}
		if recommended < 0 {
			logger.Warn("redis connected below recommended version",
				nil,
				slog.String("version", redisVersion),
				slog.String("recommended", infra.RedisRecommendedVersion))
		} else {
			logger.Info("redis connected", nil, slog.String("version", redisVersion))
		}
		defer redisClient.Close()
	} else {
		logger.Warn("redis disabled by startup flag", nil)
	}

	appLocation := cfg.App.EffectiveLocation()
	st := store.New(db, redisClient, appLocation, configCipher)
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
		tokenStore = redissession.New(redisClient, redissession.Options{
			Prefix:       cachekeys.RedisKeyAuthTokenPrefix,
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
	trafficRuntime, err := trafficservice.NewRuntime(
		ctx,
		st.Traffic,
		appLocation,
		cfg.Database.EffectiveRetentionDays(),
		cfg.Database.EffectiveTrafficRetentionDays(),
	)
	if err != nil {
		infra.Fatal("init traffic runtime failed", err)
	}
	dashUpdateRunner := dashupdate.NewRunner()
	alertService, err := alert.NewService(st.Alert, st.Front, alert.MessageConfig{
		Language: cfg.App.EffectiveLanguage(),
		Location: appLocation,
	}, cfg.App.EffectiveNodeOfflineThreshold())
	if err != nil {
		infra.Fatal("init alert service failed", err)
	}
	dashUpdateService := dashupdate.NewService(st.System, alertService, dashUpdateRunner, cfg.App.EffectiveLanguage())
	deps := httpapi.Dependencies{
		Stores:         st,
		Auth:           jwtAuth,
		Theme:          themeStore,
		TrafficRebuild: trafficRuntime.RebuildRunner(),
		DashUpdate:     dashUpdateRunner,
	}

	srv, err := transporthttp.NewHTTPServer(cfg, deps)
	if err != nil {
		infra.Fatal("init http server failed", err)
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return srv.Run(groupCtx) })
	group.Go(func() error { return alertService.Run(groupCtx) })
	group.Go(func() error { return trafficRuntime.Run(groupCtx) })
	group.Go(func() error { return dashUpdateService.Run(groupCtx) })

	runErr := group.Wait()
	trafficRuntime.Stop()
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		infra.Fatal("runtime failed", runErr)
	}
}
