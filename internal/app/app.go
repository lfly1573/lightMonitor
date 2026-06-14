package app

import (
	"context"
	"fmt"
	"log"
	"os"

	"lightmonitor/internal/application/core"
	"lightmonitor/internal/application/install"
	"lightmonitor/internal/application/runtime"
	"lightmonitor/internal/config"
	"lightmonitor/internal/infrastructure/database/sqlite"
	httpinfra "lightmonitor/internal/infrastructure/http"
)

func Run(ctx context.Context, cfg config.Config) error {
	if err := ensureRuntimeDirs(cfg); err != nil {
		return err
	}

	db, err := sqlite.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()

	installRepo := sqlite.NewInstallRepository(db)
	installService := install.NewService(installRepo)
	store := sqlite.NewStore(db)
	coreService := core.NewService(store)
	scheduler := runtime.NewScheduler(coreService, store)
	scheduler.Start(ctx)

	server := httpinfra.NewServer(httpinfra.Dependencies{
		InstallService: installService,
		CoreService:    coreService,
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("lightMonitor listening on http://127.0.0.1%s", addr)
	return server.Run(addr)
}

func ensureRuntimeDirs(cfg config.Config) error {
	for _, dir := range []string{cfg.DataDir, cfg.LogDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
