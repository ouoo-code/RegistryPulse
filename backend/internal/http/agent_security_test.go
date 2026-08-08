package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAgentRequestSignatureRejectsReplay(t *testing.T) {
	s := &Server{replay: make(map[string]time.Time)}
	token, timestamp, nonce, body := "agent-secret", "1700000000", "nonce-1", `{"status":"online"}`
	key := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte(timestamp + "\n" + nonce + "\n"))
	_, _ = mac.Write([]byte(body))
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("X-Agent-Timestamp", strconvFormat(time.Now().Unix()))
	req.Header.Set("X-Agent-Nonce", nonce)
	req.Header.Set("X-Agent-Signature", hex.EncodeToString(mac.Sum(nil)))
	// Sign with the actual current timestamp used by the request.
	actual := req.Header.Get("X-Agent-Timestamp")
	mac = hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte(actual + "\n" + nonce + "\n"))
	_, _ = mac.Write([]byte(body))
	req.Header.Set("X-Agent-Signature", hex.EncodeToString(mac.Sum(nil)))
	if !s.verifyAgentRequest(req, token) {
		t.Fatal("valid signature rejected")
	}
	req2 := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req2.Header = req.Header.Clone()
	if s.verifyAgentRequest(req2, token) {
		t.Fatal("replayed nonce accepted")
	}
}

func strconvFormat(value int64) string { return fmt.Sprintf("%d", value) }
