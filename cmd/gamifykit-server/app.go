package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	mem "gamifykit/adapters/memory"
	redisAdapter "gamifykit/adapters/redis"
	sqlxAdapter "gamifykit/adapters/sqlx"
	"gamifykit/api/httpapi"
	"gamifykit/config"
	"gamifykit/engine"
	"gamifykit/gamify"
	"gamifykit/realtime"
)

// App aggregates the assembled server components.
type App struct {
	Config  *config.Config
	Logger  *slog.Logger
	Hub     *realtime.Hub
	Service *engine.GamifyService
	Handler http.Handler
	Server  *http.Server
}

func provideConfig(ctx context.Context) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Environment == config.EnvProduction {
		if err := cfg.LoadSecretsFromEnv(ctx); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func provideLogger(cfg *config.Config) *slog.Logger {
	return setupLogging(cfg)
}

func provideHub() *realtime.Hub {
	return realtime.NewHub()
}

func provideStorage(ctx context.Context, cfg *config.Config) (engine.Storage, error) {
	return setupStorage(ctx, cfg)
}

func provideService(hub *realtime.Hub, storage engine.Storage) *engine.GamifyService {
	return gamify.New(
		gamify.WithRealtime(hub),
		gamify.WithStorage(storage),
		gamify.WithDispatchMode(engine.DispatchAsync),
	)
}

func provideHandler(svc *engine.GamifyService, hub *realtime.Hub, cfg *config.Config) http.Handler {
	return httpapi.NewMux(svc, hub, httpapi.Options{
		PathPrefix:       cfg.Server.PathPrefix,
		AllowCORSOrigin:  cfg.Server.CORSOrigin,
		APIKeys:          cfg.Security.APIKeys,
		RateLimitEnabled: cfg.Security.EnableRateLimit,
		RateLimitRPM:     cfg.Security.RateLimit.RequestsPerMinute,
		RateLimitBurst:   cfg.Security.RateLimit.BurstSize,
	})
}

func provideServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}
}

// setupLogging configures the logger based on configuration.
func setupLogging(cfg *config.Config) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: parseLogLevel(cfg.Logging.Level),
	}

	switch cfg.Logging.Format {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	if len(cfg.Logging.Attributes) > 0 {
		handler = handler.WithAttrs(convertAttributes(cfg.Logging.Attributes))
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// parseLogLevel converts string log level to slog.Level.
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// convertAttributes converts map[string]string to []slog.Attr.
func convertAttributes(attrs map[string]string) []slog.Attr {
	var result []slog.Attr
	for k, v := range attrs {
		result = append(result, slog.String(k, v))
	}
	return result
}

// setupStorage creates the appropriate storage adapter based on configuration.
func setupStorage(ctx context.Context, cfg *config.Config) (engine.Storage, error) {
	switch cfg.Storage.Adapter {
	case "memory":
		return mem.New(), nil
	case "redis":
		return redisAdapter.New(cfg.Storage.Redis)
	case "sql":
		return sqlxAdapter.New(cfg.Storage.SQL)
	case "file":
		return mem.New(), fmt.Errorf("file storage not yet implemented, using memory fallback")
	default:
		return mem.New(), fmt.Errorf("unknown storage adapter: %s", cfg.Storage.Adapter)
	}
}
