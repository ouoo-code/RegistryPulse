package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"
)

type Channel struct {
	Type    string
	Name    string
	Enabled bool
	Config  map[string]any
}

type Event struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Status  string `json:"status,omitempty"`
}

func Send(ctx context.Context, channel Channel, event Event) error {
	switch strings.ToLower(strings.TrimSpace(channel.Type)) {
	case "gotify":
		return sendGotify(ctx, channel.Config, event)
	case "webhook":
		return sendWebhook(ctx, channel.Config, event)
	case "smtp", "email":
		return sendSMTP(channel.Config, event)
	default:
		return fmt.Errorf("unsupported notification type %q", channel.Type)
	}
}

func stringValue(config map[string]any, key string) string {
	if value, ok := config[key].(string); ok {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "env:") {
			return strings.TrimSpace(os.Getenv(strings.TrimPrefix(value, "env:")))
		}
		return value
	}
	if envName, ok := config[key+"_env"].(string); ok {
		return strings.TrimSpace(os.Getenv(strings.TrimSpace(envName)))
	}
	return ""
}
func client() *http.Client { return &http.Client{Timeout: 10 * time.Second} }

func sendGotify(ctx context.Context, config map[string]any, event Event) error {
	base, token := stringValue(config, "url"), stringValue(config, "token")
	if base == "" || token == "" {
		return fmt.Errorf("gotify url and token are required")
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + "/message")
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return fmt.Errorf("invalid gotify url")
	}
	body, _ := json.Marshal(map[string]any{"title": event.Title, "message": event.Message, "priority": 5})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gotify-Key", token)
	res, err := client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("gotify returned %d", res.StatusCode)
	}
	return nil
}

func sendWebhook(ctx context.Context, config map[string]any, event Event) error {
	target, secret := stringValue(config, "url"), stringValue(config, "secret")
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return fmt.Errorf("invalid webhook url")
	}
	body, _ := json.Marshal(event)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		req.Header.Set("X-Signature-SHA256", hex.EncodeToString(mac.Sum(nil)))
	}
	res, err := client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", res.StatusCode)
	}
	return nil
}

func sendSMTP(config map[string]any, event Event) error {
	host, port, username, password, from, to := stringValue(config, "host"), stringValue(config, "port"), stringValue(config, "username"), stringValue(config, "password"), stringValue(config, "from"), stringValue(config, "to")
	if host == "" || port == "" || from == "" || to == "" {
		return fmt.Errorf("smtp host, port, from and to are required")
	}
	auth := smtp.Auth(nil)
	if username != "" {
		auth = smtp.PlainAuth("", username, password, strings.Split(host, ":")[0])
	}
	message := []byte("Subject: " + event.Title + "\r\n\r\n" + event.Message + "\r\n")
	return smtp.SendMail(host+":"+port, auth, from, []string{to}, message)
}
