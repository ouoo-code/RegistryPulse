package database

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/incident"
	"github.com/ouoo-code/RegistryPulse/internal/probe"
)

func TestNotificationSourceIDArg(t *testing.T) {
	if got := notificationSourceIDArg("  "); got != nil {
		t.Fatalf("blank source id = %#v, want nil", got)
	}
	if got := notificationSourceIDArg("source-id"); got != "source-id" {
		t.Fatalf("source id = %#v, want source-id", got)
	}
}

// TestNotifyTransitionAggregation is an opt-in PostgreSQL integration test.
// It uses a dedicated random channel/source and removes them on completion.
func TestNotifyTransitionAggregation(t *testing.T) {
	dsn := os.Getenv("NOTIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set NOTIFICATION_TEST_DATABASE_URL to run PostgreSQL notification integration tests")
	}
	t.Setenv("DATABASE_URL", dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}

	var sourceID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO registry_sources(category_id,name,base_url,registry_host,provider,region,tags,is_enabled,probe_mode)
		VALUES('dockerhub',$1,'https://example.invalid','example.invalid','test','test','[]'::jsonb,false,'registry')
		RETURNING id::text`, "notification-test-source-"+time.Now().UTC().Format("20060102150405.000000000")).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), `DELETE FROM registry_sources WHERE id=$1`, sourceID)

	var requests atomic.Int32
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()
	config, _ := json.Marshal(map[string]any{"url": webhook.URL})
	var channelID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO notification_channels(type,name,enabled,config)
		VALUES('webhook',$1,true,$2::jsonb) RETURNING id::text`, "notification-test-channel-"+time.Now().UTC().Format("20060102150405.000000000"), string(config)).Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), `DELETE FROM notification_channels WHERE id=$1`, channelID)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO notification_rules(channel_id,event_type,enabled,cooldown_seconds,aggregation_seconds,template)
		VALUES($1,'incident_opened',true,0,60,'')`, channelID); err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	source := domain.Source{ID: sourceID, Name: "aggregation-test"}
	result := probe.Result{Status: incident.Offline, CheckedAt: time.Now().UTC()}
	transition := incident.Transition{SourceID: sourceID, Event: "incident_opened", Message: "test"}
	store.NotifyTransition(ctx, source, result, transition)
	store.NotifyTransition(ctx, source, result, transition)

	if got := requests.Load(); got != 1 {
		t.Fatalf("webhook requests = %d, want 1", got)
	}
	rows, err := db.QueryContext(ctx, `SELECT status,COALESCE(source_id::text,''),error FROM notification_logs WHERE channel_id=$1 AND event_type='incident_opened' ORDER BY id`, channelID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var statuses, sourceIDs []string
	var reason string
	for rows.Next() {
		var status, sourceIDValue, errorValue string
		if err := rows.Scan(&status, &sourceIDValue, &errorValue); err != nil {
			t.Fatal(err)
		}
		statuses = append(statuses, status)
		sourceIDs = append(sourceIDs, sourceIDValue)
		if status == "coalesced" {
			reason = errorValue
		}
	}
	if len(statuses) != 2 || statuses[0] != "sent" || statuses[1] != "coalesced" {
		t.Fatalf("notification statuses = %#v, want [sent coalesced]", statuses)
	}
	if len(sourceIDs) != 2 || sourceIDs[0] != sourceID || sourceIDs[1] != sourceID {
		t.Fatalf("notification source ids = %#v, want %q for both rows", sourceIDs, sourceID)
	}
	if reason == "" {
		t.Fatal("coalesced log has no audit reason")
	}
}
