package main

import (
	"log/slog"
	"os"

	"github.com/lepinkainen/filemanager/internal/config"
	"github.com/lepinkainen/filemanager/internal/server"
	"github.com/lepinkainen/filemanager/internal/server/handlers"
	"github.com/lepinkainen/humanlog"
)

// Set at build time.
var version = "dev"

func main() {
	logger := slog.New(humanlog.NewHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	// Set version for whoami endpoint
	handlers.Version = version

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
