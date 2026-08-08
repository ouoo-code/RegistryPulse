package database

import (
	"context"
	"fmt"
	"time"
)

// RetentionPolicy controls historical data cleanup. Zero values use the
// conservative defaults and never remove active incidents.
type RetentionPolicy struct {
	ProbeResults      time.Duration
	ResolvedIncidents time.Duration
	IncidentEvents    time.Duration
}

func (p RetentionPolicy) withDefaults() RetentionPolicy {
	if p.ProbeResults <= 0 {
		p.ProbeResults = 30 * 24 * time.Hour
	}
	if p.ResolvedIncidents <= 0 {
		p.ResolvedIncidents = 180 * 24 * time.Hour
	}
	if p.IncidentEvents <= 0 {
		p.IncidentEvents = 180 * 24 * time.Hour
	}
	return p
}

type RetentionReport struct {
	ProbeResultsDeleted   int64     `json:"probe_results_deleted"`
	IncidentEventsDeleted int64     `json:"incident_events_deleted"`
	IncidentsDeleted      int64     `json:"incidents_deleted"`
	Before                time.Time `json:"before"`
	ResolvedBefore        time.Time `json:"resolved_before"`
	EventsBefore          time.Time `json:"events_before"`
}

// CleanupRetention removes old probe samples and resolved incident history in
// one transaction. Open incidents and their events are always retained. The
// now parameter makes scheduled cleanup deterministic and testable.
func (s *Store) CleanupRetention(ctx context.Context, now time.Time, policy RetentionPolicy) (RetentionReport, error) {
	if s == nil || s.DB == nil {
		return RetentionReport{}, fmt.Errorf("database store is nil")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	policy = policy.withDefaults()
	report := RetentionReport{
		Before:         now.Add(-policy.ProbeResults),
		ResolvedBefore: now.Add(-policy.ResolvedIncidents),
		EventsBefore:   now.Add(-policy.IncidentEvents),
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return report, fmt.Errorf("begin retention cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if result, err := tx.ExecContext(ctx, `DELETE FROM probe_results WHERE checked_at < $1`, report.Before); err != nil {
		return report, fmt.Errorf("delete probe results: %w", err)
	} else if report.ProbeResultsDeleted, err = result.RowsAffected(); err != nil {
		return report, fmt.Errorf("count deleted probe results: %w", err)
	}
	if result, err := tx.ExecContext(ctx, `
		DELETE FROM incident_events e
		WHERE e.created_at < $1
		AND EXISTS (
			SELECT 1 FROM incidents i
			WHERE i.id=e.incident_id AND i.resolved_at IS NOT NULL
		)`, report.EventsBefore); err != nil {
		return report, fmt.Errorf("delete incident events: %w", err)
	} else if report.IncidentEventsDeleted, err = result.RowsAffected(); err != nil {
		return report, fmt.Errorf("count deleted incident events: %w", err)
	}
	if result, err := tx.ExecContext(ctx, `DELETE FROM incidents WHERE resolved_at IS NOT NULL AND resolved_at < $1`, report.ResolvedBefore); err != nil {
		return report, fmt.Errorf("delete resolved incidents: %w", err)
	} else if report.IncidentsDeleted, err = result.RowsAffected(); err != nil {
		return report, fmt.Errorf("count deleted incidents: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return report, fmt.Errorf("commit retention cleanup: %w", err)
	}
	return report, nil
}
