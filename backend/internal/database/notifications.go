package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/incident"
	"github.com/ouoo-code/RegistryPulse/internal/notification"
	"github.com/ouoo-code/RegistryPulse/internal/probe"
)

type notificationExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func notificationSourceIDArg(id string) any {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	return id
}

func notificationLogExists(ctx context.Context, exec notificationExecutor, channelID, eventType string, seconds int, statuses ...string) (bool, error) {
	if seconds <= 0 || len(statuses) == 0 {
		return false, nil
	}
	// The status list is deliberately assembled from constants below rather
	// than user input, so the query remains parameterized for all values.
	statusFilter := "'sent'"
	if len(statuses) > 1 {
		statusFilter = "'sent','coalesced'"
	}
	var found bool
	err := exec.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM notification_logs
			WHERE channel_id=$1 AND event_type=$2 AND status IN (%s)
			  AND created_at > now() - make_interval(secs => $3)
		)`, statusFilter), channelID, eventType, seconds).Scan(&found)
	return found, err
}

func (s *Store) NotifyTransition(ctx context.Context, source domain.Source, result probe.Result, transition incident.Transition) {
	if s == nil || s.DB == nil || (transition.Event != "incident_opened" && transition.Event != "incident_resolved" && transition.Event != "degraded_detected" && transition.Event != "certificate_expiring" && transition.Event != "certificate_expiring_critical") {
		return
	}
	// A channel without an explicit rule keeps the backwards-compatible
	// "send all transitions" behavior. Once a rule exists for an event, only
	// enabled rules are eligible; cooldown and aggregation are evaluated below
	// so an aggregation suppression can be recorded even when both windows
	// overlap.
	rows, err := s.DB.QueryContext(ctx, `
		SELECT c.id::text,c.type,c.name,c.enabled,c.config,
		       COALESCE(r.cooldown_seconds,0),COALESCE(r.aggregation_seconds,0),COALESCE(r.template,'')
		FROM notification_channels c
		LEFT JOIN notification_rules r
		  ON r.channel_id=c.id AND r.event_type=$1
		WHERE c.enabled=true AND (r.id IS NULL OR r.enabled=true)`, transition.Event)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, typ, name string
		var enabled bool
		var raw []byte
		var cooldownSeconds, aggregationSeconds int
		var template string
		if rows.Scan(&id, &typ, &name, &enabled, &raw, &cooldownSeconds, &aggregationSeconds, &template) != nil {
			continue
		}
		var config map[string]any
		if json.Unmarshal(raw, &config) != nil {
			continue
		}
		event := notification.Event{Title: fmt.Sprintf("Registry %s: %s", source.Name, transition.Event), Message: transition.Message, Status: result.Status}
		if strings.TrimSpace(template) != "" {
			event.Message = renderNotificationTemplate(template, source.Name, transition.Event, transition.Message, result.Status)
		}
		channel := notification.Channel{Type: typ, Name: name, Enabled: enabled, Config: config}
		var tx *sql.Tx
		var exec notificationExecutor = s.DB
		if aggregationSeconds > 0 {
			// Serialize the check-and-log decision across API/worker instances.
			// The lock is held only for this channel/event transaction and keeps
			// concurrent transitions from both sending during one window.
			tx, err = s.DB.BeginTx(ctx, nil)
			if err != nil {
				continue
			}
			exec = tx
			if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, id+":"+transition.Event); err != nil {
				_ = tx.Rollback()
				continue
			}
		}
		rollback := func() {
			if tx != nil {
				_ = tx.Rollback()
			}
		}
		commit := func() {
			if tx != nil {
				_ = tx.Commit()
			}
		}

		if aggregationSeconds > 0 {
			active, checkErr := notificationLogExists(ctx, exec, id, transition.Event, aggregationSeconds, "sent", "coalesced")
			if checkErr != nil {
				rollback()
				continue
			}
			if active {
				_, _ = exec.ExecContext(ctx, `
					INSERT INTO notification_logs(channel_id,source_id,event_type,status,attempts,error)
					VALUES($1,$2,$3,'coalesced',0,$4)`, id, notificationSourceIDArg(source.ID), transition.Event, fmt.Sprintf("suppressed by aggregation window (%d seconds)", aggregationSeconds))
				commit()
				continue
			}
		}
		if cooldownSeconds > 0 {
			active, checkErr := notificationLogExists(ctx, exec, id, transition.Event, cooldownSeconds, "sent")
			if checkErr != nil {
				rollback()
				continue
			}
			if active {
				rollback()
				continue
			}
		}
		var sendErr error
		attempts := 0
		for attempts = 1; attempts <= 3; attempts++ {
			sendErr = notification.Send(ctx, channel, event)
			if sendErr == nil {
				break
			}
			if attempts < 3 {
				select {
				case <-ctx.Done():
					break
				case <-time.After(time.Duration(1<<(attempts-1)) * 100 * time.Millisecond):
				}
			}
		}
		if sendErr != nil {
			_, _ = exec.ExecContext(ctx, `
				INSERT INTO notification_logs(channel_id,source_id,event_type,status,attempts,error)
				VALUES($1,$2,$3,'failed',$4,$5)`, id, notificationSourceIDArg(source.ID), transition.Event, attempts, sendErr.Error())
			commit()
			continue
		}
		_, _ = exec.ExecContext(ctx, `
			INSERT INTO notification_logs(channel_id,source_id,event_type,status,attempts)
			VALUES($1,$2,$3,'sent',$4)`, id, notificationSourceIDArg(source.ID), transition.Event, attempts)
		commit()
	}
}

func renderNotificationTemplate(template, sourceName, eventType, message, status string) string {
	values := map[string]string{"{source_name}": sourceName, "{event}": eventType, "{message}": message, "{status}": status}
	for placeholder, value := range values {
		template = strings.ReplaceAll(template, placeholder, value)
	}
	return template
}
