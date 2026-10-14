package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	NodeID       string        `json:"nodeId"`
	ControlPlane string        `json:"controlPlaneUrl"`
	ClientCert   string        `json:"clientCertFile"`
	ClientKey    string        `json:"clientKeyFile"`
	ServerCA     string        `json:"serverCAFile"`
	AgentToken   string        `json:"agentToken,omitempty"`
	PollInterval time.Duration `json:"-"`
	HTTPTimeout  time.Duration `json:"-"`
	NginxBinary  string        `json:"nginxBinary"`
	NginxConfig  string        `json:"nginxConfig"`
	ReloadHelper string        `json:"reloadHelper"`
	StateDir     string        `json:"stateDir"`
	PIDFile      string        `json:"pidFile"`
}

type fileConfig struct {
	NodeID              string `json:"nodeId"`
	ControlPlane        string `json:"controlPlaneUrl"`
	ClientCert          string `json:"clientCertFile"`
	ClientKey           string `json:"clientKeyFile"`
	ServerCA            string `json:"serverCAFile"`
	AgentToken          string `json:"agentToken"`
	PollIntervalSeconds int    `json:"pollIntervalSeconds"`
	HTTPTimeoutSeconds  int    `json:"httpTimeoutSeconds"`
	NginxBinary         string `json:"nginxBinary"`
	NginxConfig         string `json:"nginxConfig"`
	ReloadHelper        string `json:"reloadHelper"`
	StateDir            string `json:"stateDir"`
	PIDFile             string `json:"pidFile"`
}

func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	var file fileConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	cfg = Config{NodeID: file.NodeID, ControlPlane: file.ControlPlane, ClientCert: file.ClientCert, ClientKey: file.ClientKey, ServerCA: file.ServerCA, AgentToken: file.AgentToken, PollInterval: time.Duration(file.PollIntervalSeconds) * time.Second, HTTPTimeout: time.Duration(file.HTTPTimeoutSeconds) * time.Second, NginxBinary: file.NginxBinary, NginxConfig: file.NginxConfig, ReloadHelper: file.ReloadHelper, StateDir: file.StateDir, PIDFile: file.PIDFile}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 15 * time.Second
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 10 * time.Second
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.NodeID == "" {
		return errors.New("nodeId is required")
	}
	u, err := url.Parse(c.ControlPlane)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("controlPlaneUrl must be an absolute https URL")
	}
	if c.ServerCA == "" || (c.AgentToken == "" && (c.ClientCert == "" || c.ClientKey == "")) {
		return errors.New("server CA and either an Agent token or a complete client certificate/key pair are required")
	}
	if (c.ClientCert == "") != (c.ClientKey == "") {
		return errors.New("client certificate and key must be configured together")
	}
	if c.PollInterval < time.Second || c.PollInterval > 5*time.Minute {
		return errors.New("pollInterval must be between 1s and 5m")
	}
	if c.HTTPTimeout < time.Second || c.HTTPTimeout > time.Minute {
		return errors.New("httpTimeout must be between 1s and 1m")
	}
	if c.NginxBinary == "" {
		return errors.New("nginxBinary is required")
	}
	if !filepath.IsAbs(c.NginxBinary) {
		return errors.New("nginxBinary must be an absolute path")
	}
	if c.NginxConfig == "" {
		return errors.New("nginxConfig is required")
	}
	if !filepath.IsAbs(c.NginxConfig) {
		return errors.New("nginxConfig must be an absolute path")
	}
	if c.PIDFile != "" && !filepath.IsAbs(c.PIDFile) {
		return errors.New("pidFile must be an absolute path")
	}
	if c.ReloadHelper != "" && !filepath.IsAbs(c.ReloadHelper) {
		return errors.New("reloadHelper must be an absolute path")
	}
	if c.StateDir == "" || !filepath.IsAbs(c.StateDir) {
		return errors.New("stateDir must be an absolute path")
	}
	return nil
}
