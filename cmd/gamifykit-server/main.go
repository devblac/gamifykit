package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx := context.Background()
	app, err := BuildApp(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize app: %v\n", err)
		os.Exit(1)
	}

	cfg := app.Config

	slog.Info("starting gamifykit server",
		"environment", cfg.Environment,
		"profile", cfg.Profile,
		"address", cfg.Server.Address,
		"storage_adapter", cfg.Storage.Adapter)

	srv := app.Server

	// Start server in a goroutine
	go func() {
		slog.Info("server listening", "address", cfg.Server.Address)
		if err := srv.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				return
			}
			slog.Error("failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server", "timeout", cfg.Server.ShutdownTimeout)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("error during server shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
