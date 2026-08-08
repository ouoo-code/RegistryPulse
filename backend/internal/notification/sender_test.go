package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookSendsSignedJSON(t *testing.T) {
	const secret = "test-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var event Event
		if json.Unmarshal(body, &event) != nil || event.Title != "title" {
			t.Fatal("invalid webhook payload")
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		if r.Header.Get("X-Signature-SHA256") != hex.EncodeToString(mac.Sum(nil)) {
			t.Fatal("invalid signature")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if err := Send(context.Background(), Channel{Type: "webhook", Config: map[string]any{"url": server.URL, "secret": secret}}, Event{Title: "title", Message: "message"}); err != nil {
		t.Fatal(err)
	}
}

func TestUnsupportedNotificationType(t *testing.T) {
	if err := Send(context.Background(), Channel{Type: "telegram"}, Event{}); err == nil {
		t.Fatal("expected unsupported type error")
	}
}
