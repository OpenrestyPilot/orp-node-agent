package collector

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Collector struct{ NginxBinary, NginxConfig, PIDFile string }

type Snapshot struct {
	CollectedAt          time.Time `json:"collectedAt"`
	Hostname             string    `json:"hostname"`
	OS                   string    `json:"os"`
	Arch                 string    `json:"arch"`
	OpenRestyVersion     string    `json:"openrestyVersion,omitempty"`
	OpenRestyRunning     bool      `json:"openrestyRunning"`
	OpenRestyPID         int       `json:"openrestyPid,omitempty"`
	ConfigReadable       bool      `json:"configReadable"`
	LoadAverage1m        float64   `json:"loadAverage1m,omitempty"`
	MemoryTotalBytes     uint64    `json:"memoryTotalBytes,omitempty"`
	MemoryAvailableBytes uint64    `json:"memoryAvailableBytes,omitempty"`
}

func (c Collector) Collect(ctx context.Context) Snapshot {
	s := Snapshot{CollectedAt: time.Now().UTC(), OS: runtime.GOOS, Arch: runtime.GOARCH}
	s.Hostname, _ = os.Hostname()
	if configFile, err := os.Open(c.NginxConfig); err == nil {
		s.ConfigReadable = true
		_ = configFile.Close()
	}
	versionCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(versionCtx, c.NginxBinary, "-V").CombinedOutput(); err == nil {
		s.OpenRestyVersion = versionFromOutput(string(out))
	}
	s.OpenRestyPID = readPID(c.PIDFile)
	s.OpenRestyRunning = s.OpenRestyPID > 0 && processMatches(s.OpenRestyPID, c.NginxBinary)
	s.LoadAverage1m, s.MemoryTotalBytes, s.MemoryAvailableBytes = linuxResources()
	return s
}

func versionFromOutput(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "openresty/") || strings.Contains(line, "nginx version:") {
			return strings.TrimPrefix(line, "nginx version: ")
		}
	}
	return ""
}

func readPID(path string) int {
	if path == "" {
		return 0
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	p, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return p
}

func processMatches(pid int, binary string) bool {
	if runtime.GOOS == "linux" {
		processPath := filepath.Join("/proc", strconv.Itoa(pid), "exe")
		actual, err := os.Readlink(processPath)
		if err == nil {
			actual = strings.TrimSuffix(actual, " (deleted)")
			configured, resolveErr := filepath.EvalSymlinks(binary)
			if resolveErr != nil {
				configured = binary
			}
			return filepath.Clean(actual) == filepath.Clean(configured)
		}
		// A least-privileged Agent cannot always read /proc/<pid>/exe for a
		// root-owned OpenResty master. /proc/<pid>/comm remains readable and
		// provides a bounded fallback using the configured executable name.
		comm, commErr := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
		if commErr != nil {
			return false
		}
		expected := filepath.Base(binary)
		if len(expected) > 15 {
			expected = expected[:15]
		}
		return strings.TrimSpace(string(comm)) == expected
	}
	if runtime.GOOS == "windows" {
		return false
	}
	err := exec.Command("kill", "-0", strconv.Itoa(pid)).Run()
	return err == nil
}

func linuxResources() (float64, uint64, uint64) {
	var load float64
	var total, available uint64
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 0 {
			load, _ = strconv.ParseFloat(fields[0], 64)
		}
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			n, e := strconv.ParseUint(fields[1], 10, 64)
			if e != nil {
				continue
			}
			switch strings.TrimSuffix(fields[0], ":") {
			case "MemTotal":
				total = n * 1024
			case "MemAvailable":
				available = n * 1024
			}
		}
	}
	return load, total, available
}
