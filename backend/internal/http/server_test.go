package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

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
