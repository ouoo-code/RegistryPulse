package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/probe"
)

// probe-agent registers independently, leases tasks, executes OCI probes, and
// reports results back to the API.
func main() {
	apiURL := flag.String("api", os.Getenv("API_URL"), "monitor API base URL")
	name := flag.String("name", os.Getenv("PROBE_AGENT_NAME"), "agent name")
	region := flag.String("region", os.Getenv("PROBE_AGENT_REGION"), "agent region")
	version := flag.String("version", os.Getenv("PROBE_AGENT_VERSION"), "agent version")
	interval := flag.Duration("interval", envDuration("PROBE_AGENT_HEARTBEAT", 30*time.Second), "heartbeat interval")
	flag.Parse()
	if strings.TrimSpace(*apiURL) == "" || strings.TrimSpace(*name) == "" || strings.TrimSpace(*version) == "" {
		slog.Error("api, name and version are required")
		os.Exit(2)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	ctx := context.Background()
	token, err := register(ctx, client, strings.TrimRight(*apiURL, "/"), *name, *region, *version)
	if err != nil {
		slog.Error("agent registration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("probe-agent registered", "name", *name)
	for {
		if err := heartbeat(ctx, client, strings.TrimRight(*apiURL, "/"), token, *version); err != nil {
			slog.Warn("heartbeat failed", "error", err)
		}
		if err := poll(ctx, client, strings.TrimRight(*apiURL, "/"), token); err != nil {
			slog.Warn("task poll failed", "error", err)
		}
		time.Sleep(*interval)
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
func requestJSON(ctx context.Context, client *http.Client, method, url, token string, body any, out any) error {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := strconv.FormatInt(time.Now().UnixNano(), 10)
		key := sha256.Sum256([]byte(token))
		mac := hmac.New(sha256.New, key[:])
		_, _ = mac.Write([]byte(timestamp + "\n" + nonce + "\n"))
		_, _ = mac.Write(payload)
		req.Header.Set("X-Agent-Timestamp", timestamp)
		req.Header.Set("X-Agent-Nonce", nonce)
		req.Header.Set("X-Agent-Signature", hex.EncodeToString(mac.Sum(nil)))
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return &httpError{status: res.StatusCode}
	}
	return json.NewDecoder(res.Body).Decode(out)
}

type httpError struct{ status int }

func (e *httpError) Error() string {
	return "agent API returned HTTP " + string(rune(e.status/100+'0')) + string(rune(e.status%100/10+'0')) + string(rune(e.status%10+'0'))
}
func register(ctx context.Context, client *http.Client, base, name, region, version string) (string, error) {
	var response struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	err := requestJSON(ctx, client, http.MethodPost, base+"/api/v1/agent/register", "", map[string]any{"name": name, "region": region, "version": version, "capabilities": []string{"oci"}}, &response)
	return response.Data.Token, err
}
func heartbeat(ctx context.Context, client *http.Client, base, token, version string) error {
	var response any
	return requestJSON(ctx, client, http.MethodPost, base+"/api/v1/agent/heartbeat", token, map[string]any{"status": "online", "version": version}, &response)
}
func poll(ctx context.Context, client *http.Client, base, token string) error {
	var response struct {
		Data struct {
			Tasks []domain.ProbeTask `json:"tasks"`
		} `json:"data"`
	}
	if err := requestJSON(ctx, client, http.MethodPost, base+"/api/v1/agent/tasks/poll", token, map[string]any{"limit": 10}, &response); err != nil {
		return err
	}
	for _, task := range response.Data.Tasks {
		if err := executeTask(ctx, client, base, token, task); err != nil {
			slog.Warn("probe task failed", "task", task.ID, "error", err)
		}
	}
	return nil
}

func executeTask(ctx context.Context, client *http.Client, base, token string, task domain.ProbeTask) error {
	var target string
	if value, ok := task.Payload["target_url"].(string); ok {
		target = value
	}
	if target == "" {
		if value, ok := task.Payload["url"].(string); ok {
			target = value
		}
	}
	if target == "" {
		return submitFailure(ctx, client, base, token, task.ID, "task payload has no target_url")
	}
	if err := postTaskAction(ctx, client, base, token, task.ID, "start", nil, nil); err != nil {
		return err
	}
	result := probe.Run(ctx, target, 15*time.Second)
	return postTaskAction(ctx, client, base, token, task.ID, "result", map[string]any{"source_id": task.SourceID, "status": result.Status, "dns_duration_ms": result.DNSMS, "tcp_duration_ms": result.TCPMS, "tls_duration_ms": result.TLSMS, "registry_api_duration_ms": result.RegistryMS, "manifest_duration_ms": result.ManifestMS, "blob_duration_ms": result.BlobMS, "blob_bytes": result.BlobBytes, "error": result.Error}, nil)
}

func submitFailure(ctx context.Context, client *http.Client, base, token, taskID, message string) error {
	return postTaskAction(ctx, client, base, token, taskID, "fail", map[string]any{"error": message}, nil)
}
func postTaskAction(ctx context.Context, client *http.Client, base, token, taskID, action string, body any, out any) error {
	if body == nil {
		body = map[string]any{}
	}
	return requestJSON(ctx, client, http.MethodPost, base+"/api/v1/agent/tasks/"+taskID+"/"+action, token, body, &out)
}
