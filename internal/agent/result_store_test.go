package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"orp-node-agent/internal/controlplane"
)

func TestResultStoreRoundTripAndDoesNotUseTaskIDAsPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	store, err := newResultStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := controlplane.TaskResult{TaskID: "../../outside", NodeID: "node-a", Status: "succeeded", CompletedAt: time.Now().UTC()}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(want.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.TaskID != want.TaskID || got.Status != want.Status {
		t.Fatalf("unexpected cached result: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "outside")); !os.IsNotExist(err) {
		t.Fatalf("task ID escaped result directory: %v", err)
	}
	info, err := os.Stat(store.path(want.TaskID))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("result file permissions = %o", info.Mode().Perm())
	}
}
