package incident

import (
	"context"
	"sync"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/probe"
)

const (
	Online      = "online"
	Degraded    = "degraded"
	Offline     = "offline"
	Maintenance = "maintenance"
	Unknown     = "unknown"
)

type Config struct {
	FailureThreshold, RecoveryThreshold, SlowThreshold int
	SlowAfter                                          time.Duration
}

func (c Config) withDefaults() Config {
	if c.FailureThreshold < 1 {
		c.FailureThreshold = 3
	}
	if c.RecoveryThreshold < 1 {
		c.RecoveryThreshold = 2
	}
	if c.SlowThreshold < 1 {
		c.SlowThreshold = 3
	}
	if c.SlowAfter <= 0 {
		c.SlowAfter = 2 * time.Second
	}
	return c
}

type Transition struct {
	SourceID, From, To, Event, Message string
	At                                 time.Time
	Suspected                          bool
}

// Incident is the durable representation of an outage/degraded period.  It is
// deliberately independent from PostgreSQL so the state machine can be used
// with an in-memory store, a database, or another history sink.
type Incident struct {
	ID         string     `json:"id"`
	SourceID   string     `json:"source_id"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
}

type Event struct {
	ID         int64     `json:"id"`
	IncidentID string    `json:"incident_id"`
	SourceID   string    `json:"source_id"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	EventType  string    `json:"event_type"`
	Message    string    `json:"message,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// HistoryStore is the persistence boundary for incident state. Implementations
// should make SaveTransition idempotent for a retried probe result.
type HistoryStore interface {
	SaveTransition(context.Context, Transition) error
}

type state struct {
	status                    string
	failures, successes, slow int
	maintenance               bool
}
type Machine struct {
	mu     sync.Mutex
	cfg    Config
	states map[string]state
}

func New(cfg Config) *Machine {
	return &Machine{cfg: cfg.withDefaults(), states: make(map[string]state)}
}

// Seed restores the last durable status before the worker starts probing.
// Debounce counters intentionally start at zero; the last published status,
// however, must not regress to Unknown merely because the worker restarted.
func (m *Machine) Seed(sourceID, status string, maintenance bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if status == "" {
		status = Unknown
	}
	m.states[sourceID] = state{
		status:      status,
		maintenance: maintenance || status == Maintenance,
	}
}

func (m *Machine) SetMaintenance(sourceID string, enabled bool) Transition {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states[sourceID]
	old := s.status
	s.maintenance = enabled
	if enabled {
		s.status = Maintenance
	} else if old == Maintenance {
		s.status = Unknown
		s.failures, s.successes, s.slow = 0, 0, 0
	}
	m.states[sourceID] = s
	return Transition{SourceID: sourceID, From: old, To: s.status, Event: "maintenance_changed", At: time.Now().UTC()}
}
func (m *Machine) Status(sourceID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states[sourceID]
	if s.status == "" {
		return Unknown
	}
	return s.status
}

func (m *Machine) Observe(sourceID string, r probe.Result) Transition {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := r.CheckedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	s := m.states[sourceID]
	if s.status == "" {
		s.status = Unknown
	}
	old := s.status
	if s.maintenance {
		return Transition{SourceID: sourceID, From: old, To: Maintenance, Event: "maintenance_ignored_probe", At: now}
	}
	slow := r.Status == Online && (r.RegistryMS+r.ManifestMS+r.BlobMS) >= m.cfg.SlowAfter.Milliseconds()
	if r.Status == Online {
		s.failures = 0
		s.successes++
		if slow {
			s.slow++
		} else {
			s.slow = 0
		}
	} else {
		s.successes = 0
		s.failures++
		s.slow = 0
	}
	suspected := false
	event := "probe_succeeded"
	message := r.Error
	switch {
	case r.Status != Online && s.failures >= m.cfg.FailureThreshold:
		s.status = Offline
		if old != Offline {
			event = "incident_opened"
		} else {
			// A still-open incident is not a new incident. Keep recording the
			// probe result, but do not append another identical history event.
			event = ""
		}
	case r.Status != Online:
		suspected = true
		if old != s.status {
			event = "probe_failed"
		} else {
			event = ""
		}
	case slow && s.slow >= m.cfg.SlowThreshold:
		s.status = Degraded
		if old != Degraded {
			event = "degraded_detected"
		} else {
			event = ""
		}
	case r.Status == Online && s.successes >= m.cfg.RecoveryThreshold:
		s.status = Online
		event = "incident_resolved"
	case r.Status == Online:
		suspected = true
		if old != s.status {
			event = "recovery_pending"
		} else {
			event = ""
		}
	}
	if r.Status == Online && !r.CertificateNotAfter.IsZero() {
		daysRemaining := int(time.Until(r.CertificateNotAfter).Hours() / 24)
		switch {
		case daysRemaining <= 7:
			event = "certificate_expiring_critical"
			message = "TLS certificate expires within 7 days"
		case daysRemaining <= 30:
			event = "certificate_expiring"
			message = "TLS certificate expires within 30 days"
		}
	}
	m.states[sourceID] = s
	return Transition{SourceID: sourceID, From: old, To: s.status, Event: event, Message: message, At: now, Suspected: suspected}
}
