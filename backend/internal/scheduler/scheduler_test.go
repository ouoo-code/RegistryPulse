package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/incident"
	"github.com/ouoo-code/RegistryPulse/internal/probe"
)

func TestCronInterval(t *testing.T) {
	for _, test := range []struct {
		expression string
		want       time.Duration
	}{
		{"@every 5m", 5 * time.Minute},
		{"*/1 * * * *", time.Minute},
		{"0 * * * *", time.Hour},
		{"0 */6 * * *", 6 * time.Hour},
		{"0 0 * * *", 24 * time.Hour},
	} {
		got, ok := CronInterval(test.expression)
		if !ok || got != test.want {
			t.Fatalf("CronInterval(%q)=%v,%v want %v,true", test.expression, got, ok, test.want)
		}
	}
	if _, ok := CronInterval("not a cron"); ok {
		t.Fatal("invalid expression accepted")
	}
}

func TestDueKindSeparatesNormalRoundFromRetry(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	runner := New(Config{
		Interval: 30 * time.Minute,
		RetryIntervalProvider: func(context.Context) time.Duration {
			return 3 * time.Minute
		},
	}, nil, nil, nil, nil)

	runner.markNormalProbe("online", now.Add(-10*time.Minute))
	due, normal := runner.dueKind(domain.Source{
		ID:          "online",
		Status:      incident.Online,
		LastChecked: now.Add(-10 * time.Minute),
	}, now)
	if due || normal {
		t.Fatalf("healthy source became due early: due=%v normal=%v", due, normal)
	}

	runner.markNormalProbe("offline", now.Add(-4*time.Minute))
	due, normal = runner.dueKind(domain.Source{
		ID:          "offline",
		Status:      incident.Offline,
		LastChecked: now.Add(-4 * time.Minute),
	}, now)
	if !due || normal {
		t.Fatalf("offline source should be retry-due only: due=%v normal=%v", due, normal)
	}

	runner.markNormalProbe("normal-round", now.Add(-31*time.Minute))
	due, normal = runner.dueKind(domain.Source{
		ID:          "normal-round",
		Status:      incident.Offline,
		LastChecked: now.Add(-1 * time.Minute),
	}, now)
	if !due || !normal {
		t.Fatalf("normal round was not due: due=%v normal=%v", due, normal)
	}
}

func TestRunOnceSkipsWhenProbeServiceIsDisabled(t *testing.T) {
	recorder := &recordingRecorder{}
	runner := New(Config{
		EnabledProvider: func(context.Context) bool { return false },
	}, func(context.Context) ([]domain.Source, error) {
		t.Fatal("source provider should not be called while the probe service is disabled")
		return nil, nil
	}, recorder, incident.New(incident.Config{}), nil)

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned an error: %v", err)
	}
	if got := recorder.Count(); got != 0 {
		t.Fatalf("disabled probe service recorded %d results, want 0", got)
	}
}

type recordingRecorder struct {
	mu          sync.Mutex
	count       int
	transitions []incident.Transition
}

func (r *recordingRecorder) RecordProbe(_ context.Context, _ domain.Source, _ probe.Result, transition incident.Transition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.count++
	r.transitions = append(r.transitions, transition)
	return nil
}

func (r *recordingRecorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func TestStableRetryIsNotPersistedButNormalRoundIs(t *testing.T) {
	recorder := &recordingRecorder{}
	runner := New(Config{Interval: 30 * time.Minute}, nil, recorder, incident.New(incident.Config{}), nil)
	runner.probe = func(context.Context, string, time.Duration) probe.Result {
		return probe.Result{Status: incident.Offline, Error: "same failure", CheckedAt: time.Now().UTC()}
	}
	source := domain.Source{ID: "stable-offline", BaseURL: "https://registry.example.test", Status: incident.Offline}
	runner.machine.Seed(source.ID, incident.Offline, false)

	runner.runSource(context.Background(), source, false, &sync.Mutex{}, new(error))
	if got := recorder.Count(); got != 0 {
		t.Fatalf("stable abnormal retry persisted %d records, want 0", got)
	}

	runner.runSource(context.Background(), source, true, &sync.Mutex{}, new(error))
	if got := recorder.Count(); got != 1 {
		t.Fatalf("normal round persisted %d records, want 1", got)
	}
}
