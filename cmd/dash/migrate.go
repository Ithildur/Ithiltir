package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dash/internal/config"
	"dash/internal/infra"
	"dash/internal/migrate"
)

func runMigrate(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	var configPath string
	var debug bool
	fs.StringVar(&configPath, "config", "", "config file path (optional)")
	fs.BoolVar(&debug, "debug", false, "enable debug logging")
	_ = fs.Parse(args)

	initialLevel := os.Getenv("APP_LOG_LEVEL")
	initialFormat := os.Getenv("APP_LOG_FORMAT")
	if _, err := infra.InitLogger(initialLevel, initialFormat); err != nil {
		_, _ = infra.InitLogger("info", "text")
	}

	cfg, warnings, err := config.LoadForMigrateWithWarnings(configPath)
	if err != nil {
		infra.Log().Error("load config failed", err)
		os.Exit(1)
	}

	logLevel := cfg.App.LogLevel
	if debug {
		logLevel = "debug"
	}
	if _, err := infra.InitLogger(logLevel, cfg.App.LogFormat); err != nil {
		infra.Log().Error("init logger failed", err)
		os.Exit(1)
	}
	for _, w := range warnings {
		infra.Log().Warn(w.Msg, w.Err, w.Attrs...)
	}

	ctx := context.Background()
	db, err := infra.NewGORMTimescale(ctx, cfg.Database)
	if err != nil {
		infra.Log().Error("init timescale failed", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		infra.Log().Error("extract sql.DB failed", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	res, err := migrate.Run(ctx, db)
	if err != nil {
		infra.Log().Error("migrate failed", err)
		os.Exit(1)
	}
	if err := migrate.SyncRetentionPolicies(ctx, db, cfg.Database.EffectiveRetentionDays(), cfg.Database.EffectiveTrafficRetentionDays()); err != nil {
		infra.Log().Error("sync retention policies failed", err)
		os.Exit(1)
	}

	fmt.Printf("migrate: total=%d applied=%d skipped=%d\n", res.Total, res.Applied, res.Skipped)
}
