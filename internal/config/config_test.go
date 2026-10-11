package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaultsIntervalsAndValidatesMTLS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.json")
	body := `{"nodeId":"node-a","controlPlaneUrl":"https://control.example","clientCertFile":"/etc/agent/client.crt","clientKeyFile":"/etc/agent/client.key","serverCAFile":"/etc/agent/ca.crt","nginxBinary":"/usr/sbin/nginx","nginxConfig":"/etc/openresty/nginx.conf","stateDir":"/var/lib/orp-node-agent"}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PollInterval != 15*time.Second || cfg.HTTPTimeout != 10*time.Second {
		t.Fatalf("unexpected defaults: %s / %s", cfg.PollInterval, cfg.HTTPTimeout)
	}
}

func TestValidateRejectsHTTPAndRelativeExecutable(t *testing.T) {
	cfg := Config{NodeID: "node-a", ControlPlane: "http://control.example", ClientCert: "cert", ClientKey: "key", ServerCA: "ca", NginxBinary: "nginx", NginxConfig: "/etc/nginx.conf", StateDir: "/var/lib/orp-node-agent"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected insecure URL to be rejected")
	}
	cfg.ControlPlane = "https://control.example"
	cfg.NginxBinary = "/usr/sbin/nginx"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected relative config paths to be rejected")
	}
}

func TestLoadRejectsUnknownConfigKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.json")
	body := `{"nodeId":"node-a","controlPlaneUrl":"https://control.example","clientCertFile":"/cert","clientKeyFile":"/key","serverCAFile":"/ca","nginxBinary":"/nginx","nginxConfig":"/nginx.conf","stateDir":"/state","nginxCommand":"arbitrary"}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown config key to fail")
	}
}
