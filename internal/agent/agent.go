package agent

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"orp-node-agent/internal/collector"
	"orp-node-agent/internal/config"
	"orp-node-agent/internal/controlplane"
	"orp-node-agent/internal/operations"
)

type Agent struct {
	config    config.Config
	client    *controlplane.Client
	collector collector.Collector
	reloader  operations.Reloader
	results   *resultStore
	logger    *slog.Logger
}

func New(cfg config.Config, logger *slog.Logger) (*Agent, error) {
	client, err := controlplane.NewWithToken(cfg.ControlPlane, cfg.NodeID, cfg.ClientCert, cfg.ClientKey, cfg.ServerCA, cfg.AgentToken, cfg.HTTPTimeout)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	results, err := newResultStore(cfg.StateDir)
	if err != nil {
		return nil, err
	}
	return &Agent{config: cfg, client: client, collector: collector.Collector{NginxBinary: cfg.NginxBinary, NginxConfig: cfg.NginxConfig, PIDFile: cfg.PIDFile}, reloader: operations.Reloader{Binary: cfg.NginxBinary, Config: cfg.NginxConfig, ReloadHelper: cfg.ReloadHelper}, results: results, logger: logger}, nil
}

func (a *Agent) Run(ctx context.Context) error {
	if err := a.cycle(ctx); err != nil {
		a.logger.Warn("agent cycle failed", "error", err)
	}
	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.cycle(ctx); err != nil {
				a.logger.Warn("agent cycle failed", "error", err)
			}
		}
	}
}

func (a *Agent) cycle(ctx context.Context) error {
	var firstErr error
	state := a.collector.Collect(ctx)
	if err := a.client.Heartbeat(ctx, state); err != nil {
		firstErr = err
		a.logger.Warn("heartbeat failed", "error", err)
	}
	task, err := a.client.NextTask(ctx)
	if err != nil {
		if firstErr == nil {
			firstErr = err
		}
		return firstErr
	}
	if task == nil {
		return firstErr
	}
	if task.ID == "" {
		return errors.New("control plane returned a task without an ID")
	}
	cached, cacheErr := a.results.Load(task.ID)
	if cacheErr != nil {
		return cacheErr
	}
	if cached != nil {
		if cached.NodeID != a.config.NodeID {
			return errors.New("cached task result belongs to another node")
		}
		if err := a.client.CompleteTask(ctx, *cached); err != nil {
			return err
		}
		a.logger.Info("cached task result redelivered", "task_id", task.ID, "status", cached.Status)
		return firstErr
	}
	result := a.reloader.Execute(ctx, *task, a.config.NodeID)
	if err := a.results.Save(result); err != nil {
		return err
	}
	if err := a.client.CompleteTask(ctx, result); err != nil {
		if firstErr == nil {
			firstErr = err
		}
		a.logger.Error("task result delivery failed", "task_id", task.ID, "error", err)
		return firstErr
	}
	a.logger.Info("task completed", "task_id", task.ID, "status", result.Status)
	return firstErr
}
