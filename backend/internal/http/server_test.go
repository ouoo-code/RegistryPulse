package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/probe"
)

func TestAdminTransientProbeUsesUnsavedConfiguration(t *testing.T) {
	t.Setenv("ADMIN_API_TOKEN", "test-token")
	t.Setenv("ALLOW_PRIVATE_TARGETS", "true")
	t.Setenv("ALLOW_INSECURE_HTTP", "true")
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	s := New(domain.NewMemoryStore())
	body := bytes.NewBufferString(`{"base_url":"` + target.URL + `","probe_mode":"http","request_timeout_seconds":1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/sources/test", body)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	s.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	var response struct {
		Data probe.Result `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Status != "online" {
		t.Fatalf("unexpected result: %+v", response.Data)
	}
}

func TestPublicSourcesPaginationAndMetadata(t *testing.T) {
	s := New(domain.NewMemoryStore())
	req := httptest.NewRequest("GET", "/api/v1/public/sources?page=1&page_size=1&sort=name&order=asc", nil)
	res := httptest.NewRecorder()
	s.Routes().ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("status=%d", res.Code)
	}
	var body struct {
		Data []domain.Source `json:"data"`
		Meta struct {
			Count    int `json:"count"`
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
			Pages    int `json:"pages"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Meta.Count != 1 || body.Meta.PageSize != 1 {
		t.Fatalf("unexpected pagination: %+v", body)
	}
}

func TestCategoryDisplaySlugKeepsStableIDFiltering(t *testing.T) {
	s := New(domain.NewMemoryStore())
	for _, path := range []string{"/api/v1/public/categories/DockerHub", "/api/v1/public/sources?category=DockerHub"} {
		req := httptest.NewRequest("GET", path, nil)
		res := httptest.NewRecorder()
		s.Routes().ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("path=%s status=%d body=%s", path, res.Code, res.Body.String())
		}
	}
}

func TestUnknownSourceSubrouteReturnsSingleError(t *testing.T) {
	s := New(domain.NewMemoryStore())
	for _, path := range []string{
		"/api/v1/public/sources/00000000-0000-0000-0000-000000000000/history",
		"/api/v1/public/sources/00000000-0000-0000-0000-000000000000/aggregates",
		"/api/v1/public/sources/00000000-0000-0000-0000-000000000000/incidents",
	} {
		res := httptest.NewRecorder()
		s.Routes().ServeHTTP(res, httptest.NewRequest("GET", path, nil))
		if res.Code != 404 {
			t.Fatalf("path=%s status=%d body=%s", path, res.Code, res.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("path=%s returned malformed JSON: %v; body=%s", path, err, res.Body.String())
		}
		if body["success"] != false {
			t.Fatalf("path=%s unexpected body=%s", path, res.Body.String())
		}
	}
}

func TestPublicRateLimit(t *testing.T) {
	old := os.Getenv("PUBLIC_API_RATE_LIMIT")
	defer os.Setenv("PUBLIC_API_RATE_LIMIT", old)
	_ = os.Setenv("PUBLIC_API_RATE_LIMIT", "1")
	s := New(domain.NewMemoryStore())
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/v1/public/summary", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		res := httptest.NewRecorder()
		s.Routes().ServeHTTP(res, req)
		if i == 0 && res.Code != 200 {
			t.Fatalf("first status=%d", res.Code)
		}
		if i == 1 && res.Code != 429 {
			t.Fatalf("second status=%d", res.Code)
		}
	}
}

func TestValidateSourceInputRequiresHTTPSByDefault(t *testing.T) {
	t.Setenv("ALLOW_PRIVATE_TARGETS", "true")
	t.Setenv("ALLOW_INSECURE_HTTP", "false")
	if err := validateSourceInput(domain.SourceInput{Name: "local", BaseURL: "http://127.0.0.1:8080"}); err == nil {
		t.Fatal("expected insecure HTTP source to be rejected")
	}
	t.Setenv("ALLOW_INSECURE_HTTP", "true")
	if err := validateSourceInput(domain.SourceInput{Name: "local", BaseURL: "http://127.0.0.1:8080"}); err != nil {
		t.Fatalf("expected explicitly allowed HTTP source, got %v", err)
	}
}

func TestSourceCreateDoesNotEchoUnknownCredentialFields(t *testing.T) {
	t.Setenv("ADMIN_API_TOKEN", "credential-echo-test")
	s := New(domain.NewMemoryStore())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/sources", bytes.NewBufferString(`{"name":"credential boundary","base_url":"https://registry.example","category_id":"dockerhub","username":"alice","password":"plain-secret","credential":"another-secret"}`))
	req.Header.Set("Authorization", "Bearer credential-echo-test")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	s.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "plain-secret") || strings.Contains(res.Body.String(), "another-secret") || strings.Contains(res.Body.String(), `"password"`) {
		t.Fatalf("source response echoed credential material: %s", res.Body.String())
	}
}
