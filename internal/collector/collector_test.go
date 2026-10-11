package collector

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVersionFromOutput(t *testing.T) {
	got := versionFromOutput("configure arguments: --with-compat\nnginx version: openresty/1.27.1.2\n")
	if got != "openresty/1.27.1.2" {
		t.Fatalf("unexpected version: %q", got)
	}
}

func TestProcessMatchesConfiguredExecutable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc executable validation is Linux-specific")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !processMatches(os.Getpid(), executable) {
		t.Fatalf("current process did not match %s", executable)
	}
	wrong := filepath.Join(t.TempDir(), "different")
	if err := os.WriteFile(wrong, []byte("not executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if processMatches(os.Getpid(), wrong) {
		t.Fatal("unrelated executable matched current process")
	}
}
