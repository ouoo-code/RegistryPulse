package database

import (
	"context"
	"fmt"
	"time"
)

type AggregatePoint struct {
	Bucket          time.Time `json:"bucket"`
	Samples         int64     `json:"samples"`
	OnlineSamples   int64     `json:"online_samples"`
	AverageDuration float64   `json:"avg_duration_ms"`
}

type SourceAggregates struct {
	Hourly []AggregatePoint `json:"hourly"`
	Daily  []AggregatePoint `json:"daily"`
}

// RollupProbeStats materializes recent raw samples into the hourly and daily
// tables. The upserts make it safe to run repeatedly after a restart.
func (s *Store) RollupProbeStats(ctx context.Context) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("database store is nil")
	}
	if _, err := s.DB.ExecContext(ctx, `
		INSERT INTO probe_hourly_stats(source_id,bucket,samples,online_samples,avg_duration_ms)
		SELECT source_id,date_trunc('hour',checked_at),count(*),count(*) FILTER (WHERE status='online'),avg(registry_duration_ms+manifest_duration_ms+blob_duration_ms)
		FROM probe_results WHERE checked_at >= date_trunc('hour',now())-interval '2 hours'
		GROUP BY source_id,date_trunc('hour',checked_at)
		ON CONFLICT(source_id,bucket) DO UPDATE SET samples=EXCLUDED.samples,online_samples=EXCLUDED.online_samples,avg_duration_ms=EXCLUDED.avg_duration_ms`); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO probe_daily_stats(source_id,bucket,samples,online_samples,avg_duration_ms)
		SELECT source_id,checked_at::date,count(*),count(*) FILTER (WHERE status='online'),avg(registry_duration_ms+manifest_duration_ms+blob_duration_ms)
		FROM probe_results WHERE checked_at >= current_date-2
		GROUP BY source_id,checked_at::date
		ON CONFLICT(source_id,bucket) DO UPDATE SET samples=EXCLUDED.samples,online_samples=EXCLUDED.online_samples,avg_duration_ms=EXCLUDED.avg_duration_ms`)
	return err
}

func (s *Store) SourceAggregates(ctx context.Context, sourceID string) (SourceAggregates, error) {
	if s == nil || s.DB == nil {
		return SourceAggregates{}, fmt.Errorf("database store is nil")
	}
	out := SourceAggregates{Hourly: []AggregatePoint{}, Daily: []AggregatePoint{}}
	rows, err := s.DB.QueryContext(ctx, `SELECT bucket,samples,online_samples,avg_duration_ms FROM probe_hourly_stats WHERE source_id=$1 AND bucket >= now()-interval '24 hours' ORDER BY bucket`, sourceID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var point AggregatePoint
		if err = rows.Scan(&point.Bucket, &point.Samples, &point.OnlineSamples, &point.AverageDuration); err != nil {
			rows.Close()
			return out, err
		}
		out.Hourly = append(out.Hourly, point)
	}
	if err = rows.Close(); err != nil {
		return out, err
	}
	rows, err = s.DB.QueryContext(ctx, `SELECT bucket::timestamptz,samples,online_samples,avg_duration_ms FROM probe_daily_stats WHERE source_id=$1 AND bucket >= current_date-29 ORDER BY bucket`, sourceID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var point AggregatePoint
		if err = rows.Scan(&point.Bucket, &point.Samples, &point.OnlineSamples, &point.AverageDuration); err != nil {
			rows.Close()
			return out, err
		}
		out.Daily = append(out.Daily, point)
	}
	return out, rows.Close()
}
