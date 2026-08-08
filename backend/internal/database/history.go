package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/incident"
	"github.com/ouoo-code/RegistryPulse/internal/probe"
)

// RecordProbe persists the probe sample, current source state, and the
// incident transition atomically. It implements the scheduler.ResultRecorder
// contract without making the scheduler depend on PostgreSQL.
func (s *Store) RecordProbe(ctx context.Context, source domain.Source, result probe.Result, transition incident.Transition) error {
	if s == nil || s.DB == nil {
		return errors.New("database store is nil")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin probe record: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	checkedAt := result.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	status := transition.To
	if status == "" {
		status = result.Status
	}
	resolvedIPs, _ := json.Marshal(result.ResolvedIPs)
	var probeID int64
	if err = tx.QueryRowContext(ctx, `
		INSERT INTO probe_results
		(source_id,status,dns_duration_ms,tcp_duration_ms,tls_duration_ms,registry_duration_ms,manifest_duration_ms,blob_duration_ms,blob_bytes,error,error_stage,dns_success,resolved_ips,tcp_success,remote_ip,remote_port,tls_success,tls_version,tls_cipher,certificate_subject,certificate_issuer,certificate_days_remaining,registry_api_success,registry_api_status,manifest_success,manifest_status,manifest_content_type,manifest_digest,blob_success,blob_status,blob_ttfb_ms,blob_speed_bps,checked_at,certificate_not_before,certificate_not_after,registry_api_version,manifest_size,blob_range_supported,dns_error,tcp_error,tls_error,registry_api_error,manifest_error,blob_error)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38,$39,$40,$41,$42,$43,$44) RETURNING id`,
		source.ID, status, result.DNSMS, result.TCPMS, result.TLSMS,
		result.RegistryMS, result.ManifestMS, result.BlobMS, result.BlobBytes,
		result.Error, result.ErrorStage, result.DNSSuccess, string(resolvedIPs), result.TCPSuccess, result.RemoteIP, result.RemotePort, result.TLSSuccess, result.TLSVersion, result.TLSCipher, result.CertificateSubject, result.CertificateIssuer, result.CertificateDaysRemaining, result.RegistrySuccess, result.RegistryStatus, result.ManifestSuccess, result.ManifestStatus, result.ManifestContentType, result.ManifestDigest, result.BlobSuccess, result.BlobStatus, result.BlobTTFBMS, result.BlobSpeedBPS, checkedAt, nullTime(result.CertificateNotBefore), nullTime(result.CertificateNotAfter), result.RegistryAPIVersion, result.ManifestSize, result.BlobRangeSupported, result.DNSError, result.TCPError, result.TLSError, result.RegistryAPIError, result.ManifestError, result.BlobError).Scan(&probeID); err != nil {
		return fmt.Errorf("insert probe result: %w", err)
	}
	stages := []struct {
		name     string
		duration int64
		success  bool
		errText  string
	}{
		{"dns", result.DNSMS, result.DNSSuccess, stageError(result, "dns")},
		{"tcp", result.TCPMS, result.TCPSuccess, stageError(result, "tcp")},
		{"tls", result.TLSMS, result.TLSSuccess, stageError(result, "tls")},
		{"registry", result.RegistryMS, result.RegistrySuccess, stageError(result, "registry")},
		{"manifest", result.ManifestMS, result.ManifestSuccess, stageError(result, "manifest")},
		{"blob", result.BlobMS, result.BlobSuccess, stageError(result, "blob")},
	}
	for _, stage := range stages {
		stageStatus := "success"
		if !stage.success {
			stageStatus = "failure"
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO probe_stage_results(probe_result_id,stage,status,duration_ms,error) VALUES($1,$2,$3,$4,$5)`, probeID, stage.name, stageStatus, stage.duration, stage.errText); err != nil {
			return fmt.Errorf("insert %s stage result: %w", stage.name, err)
		}
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE registry_sources
		SET status=$1,response_ms=$2,last_checked=$3,error=$4,updated_at=now()
		WHERE id=$5`, status, result.RegistryMS+result.ManifestMS+result.BlobMS,
		checkedAt, result.Error, source.ID); err != nil {
		return fmt.Errorf("update source status: %w", err)
	}
	if err = saveTransitionTx(ctx, tx, transition, checkedAt); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit probe record: %w", err)
	}
	return nil
}

func stageError(result probe.Result, stage string) string {
	switch stage {
	case "dns":
		return result.DNSError
	case "tcp":
		return result.TCPError
	case "tls":
		return result.TLSError
	case "registry":
		return result.RegistryAPIError
	case "manifest":
		return result.ManifestError
	case "blob":
		return result.BlobError
	}
	if result.ErrorStage == stage {
		return result.Error
	}
	return ""
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

// SaveTransition persists only the incident portion of a transition. It is
// useful when probe samples are written by an existing recorder.
func (s *Store) SaveTransition(ctx context.Context, transition incident.Transition) error {
	if s == nil || s.DB == nil {
		return errors.New("database store is nil")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin incident record: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err = saveTransitionTx(ctx, tx, transition, transition.At); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit incident record: %w", err)
	}
	return nil
}

func saveTransitionTx(ctx context.Context, tx *sql.Tx, transition incident.Transition, fallback time.Time) error {
	if transition.SourceID == "" || transition.Event == "" {
		return nil
	}
	at := transition.At
	if at.IsZero() {
		at = fallback
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	var incidentID string
	var currentStatus string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text,status FROM incidents
		WHERE source_id=$1 AND resolved_at IS NULL
		ORDER BY started_at DESC LIMIT 1 FOR UPDATE`, transition.SourceID).Scan(&incidentID, &currentStatus)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("lock active incident: %w", err)
	}
	hasActive := err == nil
	if transition.To == incident.Offline || transition.To == incident.Degraded {
		status := "open"
		if transition.To == incident.Degraded {
			status = "degraded"
		}
		if !hasActive {
			err = tx.QueryRowContext(ctx, `
				INSERT INTO incidents(source_id,status,started_at,last_error)
				VALUES($1,$2,$3,$4) RETURNING id::text`,
				transition.SourceID, status, at, transition.Message).Scan(&incidentID)
			if err != nil {
				return fmt.Errorf("open incident: %w", err)
			}
		} else if currentStatus != status || transition.Message != "" {
			if _, err = tx.ExecContext(ctx, `UPDATE incidents SET status=$1,last_error=$2 WHERE id=$3`, status, transition.Message, incidentID); err != nil {
				return fmt.Errorf("update incident: %w", err)
			}
		}
	} else if transition.To == incident.Online && hasActive {
		if _, err = tx.ExecContext(ctx, `UPDATE incidents SET status='resolved',resolved_at=$1,last_error=$2 WHERE id=$3`, at, transition.Message, incidentID); err != nil {
			return fmt.Errorf("resolve incident: %w", err)
		}
	}
	if incidentID == "" {
		return nil
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO incident_events(incident_id,source_id,from_status,to_status,event_type,message,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, incidentID, transition.SourceID,
		transition.From, transition.To, transition.Event, transition.Message, at); err != nil {
		return fmt.Errorf("insert incident event: %w", err)
	}
	return nil
}

// Incidents returns newest incidents for a source. An empty sourceID returns
// incidents for all sources.
func (s *Store) Incidents(ctx context.Context, sourceID string, limit int) ([]incident.Incident, error) {
	limit = historyLimit(limit)
	query := `SELECT id::text,source_id::text,status,started_at,resolved_at,last_error FROM incidents`
	args := []any{}
	if sourceID != "" {
		query += ` WHERE source_id=$1`
		args = append(args, sourceID)
	}
	query += fmt.Sprintf(` ORDER BY started_at DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query incidents: %w", err)
	}
	defer rows.Close()
	out := []incident.Incident{}
	for rows.Next() {
		var v incident.Incident
		var resolved sql.NullTime
		if err = rows.Scan(&v.ID, &v.SourceID, &v.Status, &v.StartedAt, &resolved, &v.LastError); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		if resolved.Valid {
			v.ResolvedAt = &resolved.Time
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) IncidentEvents(ctx context.Context, incidentID string, limit int) ([]incident.Event, error) {
	limit = historyLimit(limit)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id,incident_id::text,source_id::text,from_status,to_status,event_type,message,created_at
		FROM incident_events WHERE incident_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2`, incidentID, limit)
	if err != nil {
		return nil, fmt.Errorf("query incident events: %w", err)
	}
	defer rows.Close()
	out := []incident.Event{}
	for rows.Next() {
		var v incident.Event
		if err = rows.Scan(&v.ID, &v.IncidentID, &v.SourceID, &v.From, &v.To, &v.EventType, &v.Message, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan incident event: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func historyLimit(limit int) int {
	if limit <= 0 || limit > 500 {
		return 100
	}
	return limit
}
