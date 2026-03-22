package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/knoniewicz/pularis/config"
	"github.com/knoniewicz/pularis/internal/redispub"
	"github.com/knoniewicz/pularis/manager"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
	slog.Info("pularis starting")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config := config.LoadConfig()
	if config.RedisAddr == "" {
		slog.Error("REDIS_ADDR environment variable is required but empty")
		stop()
		return
	}
	redisClient, err := redispub.New(config.RedisAddr)
	if err != nil {
		slog.Error("failed to create redis client", "error", err)
		stop()
		return
	}

	manager := manager.NewManager(ctx, redisClient, config)
	manager.Start()
	<-ctx.Done()
	slog.Info("pularis shutting down")
	manager.Close()
	slog.Info("pularis shutdown complete")
}
