package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	database "github.com/ouoo-code/RegistryPulse/internal/database"
)

func TestNotificationRuleAggregationAPI(t *testing.T) {
	dsn := os.Getenv("NOTIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set NOTIFICATION_TEST_DATABASE_URL to run PostgreSQL notification API tests")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("ADMIN_API_TOKEN", "notification-rule-test-token")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}

	var channelID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO notification_channels(type,name,enabled,config)
		VALUES('webhook',$1,true,'{}'::jsonb) RETURNING id::text`, "notification-api-test-"+time.Now().UTC().Format("20060102150405.000000000")).Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), `DELETE FROM notification_channels WHERE id=$1`, channelID)

	s := New(database.NewStore(db))
	postBody := []byte(`{"channel_id":"` + channelID + `","event_type":"incident_opened","enabled":true,"cooldown_seconds":10,"aggregation_seconds":42,"template":""}`)
	post := httptest.NewRequest(http.MethodPost, "/api/v1/admin/notification-rules", bytes.NewReader(postBody))
	post.Header.Set("Authorization", "Bearer notification-rule-test-token")
	postResult := httptest.NewRecorder()
	s.Routes().ServeHTTP(postResult, post)
	if postResult.Code != http.StatusCreated {
		t.Fatalf("POST status=%d body=%s", postResult.Code, postResult.Body.String())
	}
	var postResponse struct {
		Data struct {
			ID                 string `json:"id"`
			AggregationSeconds int    `json:"aggregation_seconds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(postResult.Body.Bytes(), &postResponse); err != nil {
		t.Fatal(err)
	}
	if postResponse.Data.ID == "" || postResponse.Data.AggregationSeconds != 42 {
		t.Fatalf("POST response = %+v", postResponse.Data)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/v1/admin/notification-rules", nil)
	get.Header.Set("Authorization", "Bearer notification-rule-test-token")
	getResult := httptest.NewRecorder()
	s.Routes().ServeHTTP(getResult, get)
	if getResult.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", getResult.Code, getResult.Body.String())
	}
	var getResponse struct {
		Data []struct {
			ID                 string `json:"id"`
			AggregationSeconds int    `json:"aggregation_seconds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(getResult.Body.Bytes(), &getResponse); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rule := range getResponse.Data {
		if rule.ID == postResponse.Data.ID {
			found = true
			if rule.AggregationSeconds != 42 {
				t.Fatalf("GET aggregation_seconds=%d, want 42", rule.AggregationSeconds)
			}
		}
	}
	if !found {
		t.Fatalf("created rule %q missing from GET response", postResponse.Data.ID)
	}

	putBody := []byte(`{"id":"` + postResponse.Data.ID + `","channel_id":"` + channelID + `","event_type":"incident_opened","enabled":true,"cooldown_seconds":10,"aggregation_seconds":-1,"template":""}`)
	put := httptest.NewRequest(http.MethodPut, "/api/v1/admin/notification-rules/"+postResponse.Data.ID, bytes.NewReader(putBody))
	put.Header.Set("Authorization", "Bearer notification-rule-test-token")
	putResult := httptest.NewRecorder()
	s.Routes().ServeHTTP(putResult, put)
	if putResult.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", putResult.Code, putResult.Body.String())
	}
	var putResponse struct {
		Data struct {
			AggregationSeconds int `json:"aggregation_seconds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(putResult.Body.Bytes(), &putResponse); err != nil {
		t.Fatal(err)
	}
	if putResponse.Data.AggregationSeconds != 0 {
		t.Fatalf("negative aggregation_seconds was not normalized: %+v", putResponse.Data)
	}
}
