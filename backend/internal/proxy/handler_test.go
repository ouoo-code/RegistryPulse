package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

func TestHandlerStreamsWithoutCaching(t *testing.T) {
	var requests atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", "sha256:test")
		_, _ = io.WriteString(w, "payload-")
	}))
	defer upstream.Close()

	manager, err := NewRouteManager(Config{CategoryID: "dockerhub", StaticUpstreams: []string{upstream.URL}, AllowPrivateTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(manager, nil).Routes())
	defer server.Close()

	for range 2 {
		response, requestErr := http.Get(server.URL + "/v2/library/alpine/manifests/latest")
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "payload-" {
			t.Fatalf("response = %d %q, err=%v", response.StatusCode, body, readErr)
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("upstream requests = %d, want 2", requests.Load())
	}
}

func TestHandlerRedirectsRegistryTraffic(t *testing.T) {
	var upstreamRequests atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	manager, err := NewRouteManager(Config{CategoryID: "dockerhub", StaticUpstreams: []string{upstream.URL}, AllowPrivateTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.ApplyRuntimeConfig(RuntimeConfig{Enabled: true, TransportMode: TransportModeRedirect, ListenPort: 10800, CategoryID: "dockerhub", RouteMaxAgeMinutes: 120, FailureCooldownSeconds: 30, MaxConcurrent: 64, MaxRangeMB: 256, MaxManifestMB: 8}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(manager, nil).Routes())
	defer server.Close()

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(server.URL + "/v2/library/alpine/blobs/sha256:test?x=1")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusTemporaryRedirect)
	}
	if got, want := response.Header.Get("Location"), upstream.URL+"/v2/library/alpine/blobs/sha256:test?x=1"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
	if upstreamRequests.Load() != 0 {
		t.Fatalf("upstream requests = %d, want 0", upstreamRequests.Load())
	}
}

func TestHandlerFailsOverBeforeWritingHeaders(t *testing.T) {
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer failed.Close()
	success := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "2")
		_, _ = io.WriteString(w, "ok")
	}))
	defer success.Close()

	manager, err := NewRouteManager(Config{CategoryID: "dockerhub", StaticUpstreams: []string{failed.URL, success.URL}, AllowPrivateTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(manager, nil).Routes())
	defer server.Close()
	response, err := http.Get(server.URL + "/v2/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("response = %d %q", response.StatusCode, body)
	}
}

func TestHandlerRejectsUnknownLengthManifestOverLimit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		_, _ = io.WriteString(w, "12345")
	}))
	defer upstream.Close()

	manager, err := NewRouteManager(Config{
		CategoryID:          "dockerhub",
		StaticUpstreams:     []string{upstream.URL},
		AllowPrivateTargets: true,
		MaxManifestBytes:    4,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(manager, nil).Routes())
	defer server.Close()

	response, err := http.Get(server.URL + "/v2/library/alpine/manifests/latest")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestHandlerAddsRequestIDAndMetrics(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	manager, err := NewRouteManager(Config{CategoryID: "dockerhub", StaticUpstreams: []string{upstream.URL}, AllowPrivateTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(manager, nil).Routes())
	defer server.Close()

	response, err := http.Get(server.URL + "/v2/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.Header.Get("X-RegistryPulse-Request-ID") == "" {
		t.Fatal("request id header is missing")
	}

	metrics, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer metrics.Body.Close()
	data, _ := io.ReadAll(metrics.Body)
	for _, name := range []string{
		"registrypulse_proxy_bytes_forwarded_total",
		"registrypulse_proxy_responses_total{class=\"2xx\"}",
	} {
		if !strings.Contains(string(data), name) {
			t.Fatalf("metrics missing %q: %s", name, data)
		}
	}
}

func TestHandlerAddsRequestIDToRejectedRegistryRequest(t *testing.T) {
	manager, err := NewRouteManager(Config{CategoryID: "dockerhub", AllowPrivateTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.ApplyRuntimeConfig(RuntimeConfig{
		Enabled:                false,
		TransportMode:          TransportModeForward,
		ListenPort:             10800,
		CategoryID:             "dockerhub",
		RouteMaxAgeMinutes:     120,
		FailureCooldownSeconds: 30,
		MaxConcurrent:          64,
		MaxRangeMB:             256,
		MaxManifestMB:          8,
	}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(manager, nil).Routes())
	defer server.Close()

	response, err := http.Get(server.URL + "/v2/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusServiceUnavailable)
	}
	if response.Header.Get("X-RegistryPulse-Request-ID") == "" {
		t.Fatal("request id header is missing on rejected request")
	}
}

func TestHandlerRejectsMutationAndStripsAuthorization(t *testing.T) {
	var authHeader atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader.Store(r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()
	manager, err := NewRouteManager(Config{CategoryID: "dockerhub", StaticUpstreams: []string{upstream.URL}, AllowPrivateTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(manager, nil).Routes())
	defer server.Close()

	request, _ := http.NewRequest(http.MethodGet, server.URL+"/v2/", nil)
	request.Header.Set("Authorization", "Bearer client-secret")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if got, _ := authHeader.Load().(string); got != "" {
		t.Fatalf("upstream received Authorization %q", got)
	}

	request, _ = http.NewRequest(http.MethodPost, server.URL+"/v2/", strings.NewReader("push"))
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", response.StatusCode)
	}
}

func TestHandlerBindsBearerTokenToChallengeSource(t *testing.T) {
	var requests atomic.Int64
	var received atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		received.Store(r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="https://auth.example.com/token",service="registry.example.com",scope="repository:library/alpine:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "manifest")
	}))
	defer upstream.Close()
	manager, err := NewRouteManager(Config{CategoryID: "dockerhub", StaticUpstreams: []string{upstream.URL}, AllowPrivateTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(manager, nil).Routes())
	defer server.Close()
	client := &http.Client{}
	response, err := client.Get(server.URL + "/v2/library/alpine/manifests/latest")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("challenge status = %d", response.StatusCode)
	}

	request, _ := http.NewRequest(http.MethodGet, server.URL+"/v2/library/alpine/manifests/latest", nil)
	request.Header.Set("Authorization", "Bearer short-lived-token")
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "manifest" {
		t.Fatalf("authenticated response = %d %q", response.StatusCode, body)
	}
	if requests.Load() != 2 || received.Load() != "Bearer short-lived-token" {
		t.Fatalf("requests=%d authorization=%q", requests.Load(), received.Load())
	}
}

func TestBuildSnapshotNeverContainsCredentials(t *testing.T) {
	snapshot := BuildSnapshot([]domain.Source{{ID: "source-1", CategoryID: "dockerhub", BaseURL: "https://registry.example.com", Name: "Example", Enabled: true, Status: "online"}})
	if len(snapshot.Candidates) != 1 || snapshot.Candidates[0].BaseURL != "https://registry.example.com" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestBuildSnapshotPrefersOnlineOverDegraded(t *testing.T) {
	snapshot := BuildSnapshot([]domain.Source{
		{ID: "degraded", CategoryID: "dockerhub", BaseURL: "https://degraded.example.com", Name: "Recommended degraded", Enabled: true, Status: "degraded", IsRecommended: true},
		{ID: "online", CategoryID: "dockerhub", BaseURL: "https://online.example.com", Name: "Online", Enabled: true, Status: "online"},
	})
	if len(snapshot.Candidates) != 2 || snapshot.Candidates[0].SourceID != "online" {
		t.Fatalf("candidate order = %#v, want online first", snapshot.Candidates)
	}
}
