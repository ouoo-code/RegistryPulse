package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckV2AcceptsUnauthorizedChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer srv.Close()
	client := Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	status, err := client.CheckV2(context.Background())
	if err != nil || status != http.StatusUnauthorized {
		t.Fatalf("status=%d err=%v", status, err)
	}
}
