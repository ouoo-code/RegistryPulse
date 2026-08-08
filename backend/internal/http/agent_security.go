package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) verifyAgentRequest(r *http.Request, token string) bool {
	timestamp := strings.TrimSpace(r.Header.Get("X-Agent-Timestamp"))
	nonce := strings.TrimSpace(r.Header.Get("X-Agent-Nonce"))
	signature := strings.TrimSpace(r.Header.Get("X-Agent-Signature"))
	if timestamp == "" || nonce == "" || signature == "" {
		return false
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || abs64(time.Now().Unix()-seconds) > 300 {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		return false
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	key := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte(timestamp + "\n" + nonce + "\n"))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return false
	}
	s.replayMu.Lock()
	defer s.replayMu.Unlock()
	for k, at := range s.replay {
		if time.Since(at) > 10*time.Minute {
			delete(s.replay, k)
		}
	}
	keyed := token + ":" + nonce
	if _, exists := s.replay[keyed]; exists {
		return false
	}
	s.replay[keyed] = time.Now()
	return true
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
