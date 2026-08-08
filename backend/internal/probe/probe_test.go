package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/registry"
)

func TestValidateTargetBlocksPrivateAddresses(t *testing.T) {
	for _, target := range []string{"http://127.0.0.1:8080", "http://localhost", "http://169.254.169.254"} {
		if ValidateTarget(target, false) == nil {
			t.Fatalf("expected target to be blocked: %s", target)
		}
	}
}

func TestValidateTargetAllowsPublicShape(t *testing.T) {
	if err := ValidateTarget("https://registry.example.com", false); err != nil {
		t.Fatal(err)
	}
}

func TestRedirectTargetRevalidatesPrivateAddress(t *testing.T) {
	target, err := url.Parse("http://127.0.0.1:8080/v2/")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRedirectTarget(context.Background(), target, false); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("expected private redirect to be rejected, got %v", err)
	}
}

func TestRunHonorsProbeTimeout(t *testing.T) {
	t.Setenv("ALLOW_PRIVATE_TARGETS", "true")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	result := Run(context.Background(), server.URL, 20*time.Millisecond)
	lower := strings.ToLower(result.Error)
	if result.Error == "" || (!strings.Contains(lower, "deadline") && !strings.Contains(lower, "timeout")) {
		t.Fatalf("expected timeout error, got %+v", result)
	}
}

func TestDockerPullDisabledByDefault(t *testing.T) {
	t.Setenv("ENABLE_REAL_DOCKER_PULL", "false")
	result := RunDockerPull(context.Background(), time.Second, "library/hello-world:latest", 1<<20)
	if result.Status != "offline" || !strings.Contains(result.Error, "disabled") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestInvalidSOCKS5ProxyIsRejected(t *testing.T) {
	t.Setenv("ALLOW_PRIVATE_TARGETS", "true")
	t.Setenv("PROBE_SOCKS5_PROXY", "http://127.0.0.1:1080")
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	result := Run(context.Background(), srv.URL, 100*time.Millisecond)
	if !strings.Contains(result.Error, "invalid SOCKS5 proxy") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOCIProbeFlowUsesBearerManifestFallbackAndBlobRange(t *testing.T) {
	var tokenSeen, rangeSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/" && r.Header.Get("Authorization") == "":
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+"http://"+r.Host+`/token",service="test",scope="repository:library/alpine:pull")`)
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/token":
			tokenSeen = true
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "abc"})
		case r.URL.Path == "/v2/library/alpine/manifests/latest" && r.Method == http.MethodHead:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case r.URL.Path == "/v2/library/alpine/manifests/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{"schemaVersion": 2, "config": map[string]any{"digest": "sha256:test", "size": 10}})
		case strings.Contains(r.URL.Path, "/blobs/"):
			rangeSeen = r.Header.Get("Range") == "bytes=0-65535"
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(make([]byte, 128))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := registry.Client{BaseURL: srv.URL, UserAgent: "test"}
	if _, err := client.CheckV2(context.Background()); err != nil {
		t.Fatal(err)
	}
	manifest, err := client.Manifest(context.Background(), "library/alpine", "latest")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Manifest.Config.Digest == "" {
		t.Fatal("manifest config digest missing")
	}
	if _, bytesRead, _, err := client.BlobRange(context.Background(), "library/alpine", manifest.Manifest.Config.Digest, 64<<10); err != nil {
		t.Fatal(err)
	} else if bytesRead != 128 {
		t.Fatalf("expected bounded response, got %d bytes", bytesRead)
	}
	if !tokenSeen || !rangeSeen {
		t.Fatalf("tokenSeen=%v rangeSeen=%v", tokenSeen, rangeSeen)
	}
}
