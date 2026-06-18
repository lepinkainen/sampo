package main

import (
	"log/slog"
	"os"

	"github.com/lepinkainen/humanlog"
	"github.com/lepinkainen/sampo/internal/config"
	"github.com/lepinkainen/sampo/internal/server"
	"github.com/lepinkainen/sampo/internal/server/handlers"
)

// Set at build time.
var (
	version   = "dev"
	gitHash   = "unknown"
	buildTime = "unknown"
)

func main() {
	logger := slog.New(humanlog.NewHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	// Set build metadata for whoami endpoint.
	handlers.Version = version
	handlers.GitHash = gitHash
	handlers.BuildTime = buildTime

	cfg, err := config.Load()
	if err != nil {
		logger.Error("loading config", "error", err)
		os.Exit(1)
	}

	frontendDir := os.DirFS("frontend/build")

	srv, err := server.New(cfg, frontendDir, logger)
	if err != nil {
		logger.Error("creating server", "error", err)
		os.Exit(1)
	}

	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
