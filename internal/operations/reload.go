package operations

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"orp-node-agent/internal/controlplane"
)

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}
type execRunner struct{}

func (execRunner) Run(ctx context.Context, binary string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, binary, args...).CombinedOutput()
}

type Reloader struct {
	Binary, Config, ReloadHelper string
	Runner                       Runner
}

func (r Reloader) Execute(ctx context.Context, task controlplane.Task, nodeID string) controlplane.TaskResult {
	now := time.Now().UTC()
	result := controlplane.TaskResult{TaskID: task.ID, NodeID: nodeID, StartedAt: now}
	finish := func() { result.CompletedAt = time.Now().UTC() }
	if task.ID == "" || task.NodeID != nodeID {
		result.Status = "rejected"
		result.Error = "task identity does not match this node"
		finish()
		return result
	}
	if task.ExpiresAt.IsZero() || !task.ExpiresAt.After(now) {
		result.Status = "rejected"
		result.Error = "task is expired or has no expiry"
		finish()
		return result
	}
	if task.Operation != "reload" {
		result.Status = "rejected"
		result.Error = "operation is not allowlisted"
		finish()
		return result
	}
	if r.Binary == "" || r.Config == "" {
		result.Status = "failed"
		result.Error = "local OpenResty binary and config are not configured"
		finish()
		return result
	}
	operationDeadline := time.Now().Add(30 * time.Second)
	if task.ExpiresAt.Before(operationDeadline) {
		operationDeadline = task.ExpiresAt
	}
	opCtx, cancel := context.WithDeadline(ctx, operationDeadline)
	defer cancel()
	runner := r.Runner
	if runner == nil {
		runner = execRunner{}
	}
	if r.ReloadHelper != "" {
		output, err := runner.Run(opCtx, "/usr/bin/sudo", "-n", r.ReloadHelper)
		result.Output = bounded(string(output), 4096)
		if err != nil {
			result.Status = "failed"
			result.Error = "fixed OpenResty validation/reload helper failed: " + err.Error()
			finish()
			return result
		}
	} else {
		output, err := runner.Run(opCtx, r.Binary, "-t", "-c", r.Config)
		result.Output = bounded(string(output), 4096)
		if err != nil {
			result.Status = "failed"
			result.Error = "nginx configuration test failed: " + err.Error()
			finish()
			return result
		}
		output, err = runner.Run(opCtx, r.Binary, "-s", "reload", "-c", r.Config)
		if len(output) > 0 {
			result.Output = bounded(result.Output+"\n"+string(output), 4096)
		}
		if err != nil {
			result.Status = "failed"
			result.Error = "nginx reload failed: " + err.Error()
			finish()
			return result
		}
	}
	result.Status = "succeeded"
	finish()
	return result
}

func bounded(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		value = value[:max] + "…"
	}
	return value
}
