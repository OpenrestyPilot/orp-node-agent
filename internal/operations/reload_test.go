package operations

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"orp-node-agent/internal/controlplane"
)

type fakeRunner struct {
	calls  [][]string
	failAt int
}

func (f *fakeRunner) Run(_ context.Context, binary string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{binary}, args...))
	if f.failAt == len(f.calls) {
		return []byte("invalid directive"), errors.New("exit status 1")
	}
	return []byte("ok"), nil
}

func TestReloadValidatesThenReloadsFixedTarget(t *testing.T) {
	runner := &fakeRunner{}
	reloader := Reloader{Binary: "/usr/sbin/nginx", Config: "/etc/openresty/nginx.conf", Runner: runner}
	task := controlplane.Task{ID: "task-1", NodeID: "node-a", Operation: "reload", ExpiresAt: time.Now().Add(time.Minute)}
	result := reloader.Execute(context.Background(), task, "node-a")
	if result.Status != "succeeded" {
		t.Fatalf("unexpected result: %+v", result)
	}
	want := [][]string{{"/usr/sbin/nginx", "-t", "-c", "/etc/openresty/nginx.conf"}, {"/usr/sbin/nginx", "-s", "reload", "-c", "/etc/openresty/nginx.conf"}}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("commands differ: %#v", runner.calls)
	}
}

func TestReloadRejectsWrongNodeExpiredAndUnknownOperation(t *testing.T) {
	tests := []struct {
		name string
		task controlplane.Task
	}{
		{"wrong node", controlplane.Task{ID: "a", NodeID: "other", Operation: "reload", ExpiresAt: time.Now().Add(time.Minute)}},
		{"expired", controlplane.Task{ID: "b", NodeID: "node-a", Operation: "reload", ExpiresAt: time.Now().Add(-time.Minute)}},
		{"unknown operation", controlplane.Task{ID: "c", NodeID: "node-a", Operation: "shell", ExpiresAt: time.Now().Add(time.Minute)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{}
			got := (Reloader{Binary: "/nginx", Config: "/nginx.conf", Runner: runner}).Execute(context.Background(), tt.task, "node-a")
			if got.Status != "rejected" {
				t.Fatalf("expected rejection: %+v", got)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("rejected task executed commands: %#v", runner.calls)
			}
		})
	}
}

func TestReloadDoesNotReloadWhenConfigTestFails(t *testing.T) {
	runner := &fakeRunner{failAt: 1}
	task := controlplane.Task{ID: "t", NodeID: "node-a", Operation: "reload", ExpiresAt: time.Now().Add(time.Minute)}
	got := (Reloader{Binary: "/nginx", Config: "/nginx.conf", Runner: runner}).Execute(context.Background(), task, "node-a")
	if got.Status != "failed" || len(runner.calls) != 1 {
		t.Fatalf("unexpected result: %+v calls=%v", got, runner.calls)
	}
}

func TestReloadUsesFixedPrivilegedHelperWithoutTaskArguments(t *testing.T) {
	runner := &fakeRunner{}
	task := controlplane.Task{ID: "t", NodeID: "node-a", Operation: "reload", ExpiresAt: time.Now().Add(time.Minute)}
	got := (Reloader{Binary: "/nginx", Config: "/nginx.conf", ReloadHelper: "/usr/local/sbin/orp-openresty-reload", Runner: runner}).Execute(context.Background(), task, "node-a")
	if got.Status != "succeeded" {
		t.Fatalf("unexpected result: %+v", got)
	}
	want := []string{"/usr/bin/sudo", "-n", "/usr/local/sbin/orp-openresty-reload"}
	if len(runner.calls) != 1 || !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("helper invocation can vary: %#v", runner.calls)
	}
}

func TestHelperFailureIsReportedWithoutFallbackCommands(t *testing.T) {
	runner := &fakeRunner{failAt: 1}
	task := controlplane.Task{ID: "t", NodeID: "node-a", Operation: "reload", ExpiresAt: time.Now().Add(time.Minute)}
	got := (Reloader{Binary: "/nginx", Config: "/nginx.conf", ReloadHelper: "/usr/local/sbin/orp-openresty-reload", Runner: runner}).Execute(context.Background(), task, "node-a")
	if got.Status != "failed" || len(runner.calls) != 1 {
		t.Fatalf("unexpected helper fallback: result=%+v calls=%v", got, runner.calls)
	}
}
