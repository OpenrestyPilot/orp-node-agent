package controlplane

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"orp-node-agent/internal/collector"
)

type Client struct {
	base   *url.URL
	http   *http.Client
	nodeID string
	token  string
}
type Task struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"nodeId"`
	Operation string    `json:"operation"`
	ExpiresAt time.Time `json:"expiresAt"`
	IssuedAt  time.Time `json:"issuedAt"`
}
type TaskResult struct {
	TaskID      string    `json:"taskId"`
	NodeID      string    `json:"nodeId"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
}

func New(baseURL, nodeID, certFile, keyFile, caFile string, timeout time.Duration) (*Client, error) {
	return NewWithToken(baseURL, nodeID, certFile, keyFile, caFile, "", timeout)
}

func NewWithToken(baseURL, nodeID, certFile, keyFile, caFile, token string, timeout time.Duration) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, err
	}
	if base.Scheme != "https" || base.Host == "" {
		return nil, errors.New("control plane URL must use https")
	}
	var certificates []tls.Certificate
	if certFile != "" || keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client identity: %w", err)
		}
		certificates = []tls.Certificate{cert}
	}
	caPEM, err := osReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read server CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("server CA file contains no certificates")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: certificates, RootCAs: roots}}
	return &Client{base: base, nodeID: nodeID, token: token, http: &http.Client{Transport: transport, Timeout: timeout}}, nil
}

// osReadFile is kept tiny to make the client constructor's filesystem boundary explicit.
var osReadFile = func(path string) ([]byte, error) { return os.ReadFile(path) }

func (c *Client) Heartbeat(ctx context.Context, state collector.Snapshot) error {
	return c.request(ctx, http.MethodPost, "/api/agent/v1/heartbeat", map[string]any{"nodeId": c.nodeID, "state": state}, nil)
}
func (c *Client) NextTask(ctx context.Context) (*Task, error) {
	var task Task
	err := c.request(ctx, http.MethodGet, "/api/agent/v1/tasks/next?nodeId="+url.QueryEscape(c.nodeID), nil, &task)
	if errors.Is(err, errNoTask) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}
func (c *Client) CompleteTask(ctx context.Context, result TaskResult) error {
	return c.request(ctx, http.MethodPost, "/api/agent/v1/tasks/"+url.PathEscape(result.TaskID)+"/result", result, nil)
}

var errNoTask = errors.New("no task")

func (c *Client) request(ctx context.Context, method, path string, body, out any) error {
	u, err := endpointURL(c.base, path)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent && method == http.MethodGet {
		return errNoTask
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("control plane returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode control-plane response: %w", err)
	}
	return nil
}

func endpointURL(base *url.URL, path string) (*url.URL, error) {
	endpoint, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	u := *base
	u.Path = strings.TrimRight(base.Path, "/") + endpoint.Path
	u.RawPath = strings.TrimRight(base.EscapedPath(), "/") + endpoint.EscapedPath()
	u.RawQuery = endpoint.RawQuery
	return &u, nil
}
