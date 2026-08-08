package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var safeImageReference = regexp.MustCompile("^[a-z0-9][a-z0-9._/-]*:[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$")
var pullSlots = make(chan struct{}, 2)

// RunDockerPull is an optional, bounded Docker Engine API probe. It never
// invokes a shell and refuses to run unless explicitly enabled.
func RunDockerPull(ctx context.Context, timeout time.Duration, image string, maxBytes int64) Result {
	result := Result{Status: "offline", CheckedAt: time.Now().UTC()}
	if !strings.EqualFold(os.Getenv("ENABLE_REAL_DOCKER_PULL"), "true") {
		result.Error = "docker pull disabled"
		return result
	}
	if image == "" {
		image = "library/hello-world:latest"
	}
	if !safeImageReference.MatchString(image) || strings.Contains(image, "..") {
		result.Error = "invalid test image reference"
		return result
	}
	if maxBytes <= 0 {
		maxBytes = 64 << 20
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case pullSlots <- struct{}{}:
		defer func() { <-pullSlots }()
	case <-ctx.Done():
		result.Error = ctx.Err().Error()
		return result
	}
	client, err := dockerEngineClient()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	pullURL := "http://docker/images/create?fromImage=" + url.QueryEscape(image)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, pullURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		result.Error = "docker pull: " + err.Error()
		return result
	}
	stream, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || readErr != nil {
		result.Error = fmt.Sprintf("docker pull returned %d", resp.StatusCode)
		if len(stream) > 0 {
			result.Error += ": " + strings.TrimSpace(string(stream))
		}
		return result
	}
	defer removeDockerImage(ctx, client, image)
	inspect, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/images/"+url.PathEscape(image)+"/json", nil)
	info, err := client.Do(inspect)
	if err != nil {
		result.Error = "docker inspect: " + err.Error()
		return result
	}
	var metadata struct{ Size int64 }
	_ = json.NewDecoder(io.LimitReader(info.Body, 1<<20)).Decode(&metadata)
	info.Body.Close()
	if metadata.Size > maxBytes {
		result.Error = fmt.Sprintf("docker image exceeds %d bytes", maxBytes)
		return result
	}
	result.Status = "online"
	return result
}

func dockerEngineClient() (*http.Client, error) {
	host := strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid DOCKER_HOST")
	}
	switch u.Scheme {
	case "unix":
		return &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", u.Path)
		}}}, nil
	case "tcp":
		return &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", u.Host)
		}}}, nil
	default:
		return nil, fmt.Errorf("DOCKER_HOST must use unix or tcp")
	}
}

func removeDockerImage(ctx context.Context, client *http.Client, image string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, "http://docker/images/"+url.PathEscape(image)+"?force=true", nil)
	if req != nil {
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}
}
