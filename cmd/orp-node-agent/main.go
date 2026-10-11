package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"orp-node-agent/internal/agent"
	"orp-node-agent/internal/config"
)

func main() {
	configPath := flag.String("config", "/etc/orp-node-agent/config.json", "path to the agent JSON configuration")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	runner, err := agent.New(cfg, logger)
	if err != nil {
		logger.Error("agent initialization failed", "error", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := runner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("agent stopped", "error", err)
		os.Exit(1)
	}
}
