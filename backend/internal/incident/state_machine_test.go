package incident

import (
	"testing"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/probe"
)

func observe(machine *Machine, status string, at time.Time) Transition {
	return machine.Observe("source-1", probe.Result{Status: status, CheckedAt: at})
}

func TestMachineDebouncesFailuresAndRecovers(t *testing.T) {
	machine := New(Config{FailureThreshold: 3, RecoveryThreshold: 2})
	base := time.Unix(100, 0)
	if transition := observe(machine, "offline", base); transition.To != Unknown || !transition.Suspected {
		t.Fatalf("first failure should remain suspected, got %+v", transition)
	}
	if transition := observe(machine, "offline", base.Add(time.Second)); transition.To != Unknown {
		t.Fatalf("second failure should remain unknown, got %+v", transition)
	}
	if transition := observe(machine, "offline", base.Add(2*time.Second)); transition.To != Offline || transition.Event != "incident_opened" {
		t.Fatalf("third failure should open incident, got %+v", transition)
	}
	if transition := observe(machine, "offline", base.Add(3*time.Second)); transition.To != Offline || transition.Event != "" {
		t.Fatalf("continued failure should not reopen incident, got %+v", transition)
	}
	if transition := observe(machine, "online", base.Add(3*time.Second)); transition.To != Offline {
		t.Fatalf("first recovery should be pending, got %+v", transition)
	}
	if transition := observe(machine, "online", base.Add(4*time.Second)); transition.To != Online || transition.Event != "incident_resolved" {
		t.Fatalf("second recovery should resolve incident, got %+v", transition)
	}
}

func TestMachineSeedPreservesDurableStatusAfterRestart(t *testing.T) {
	machine := New(Config{RecoveryThreshold: 2})
	machine.Seed("source-1", Online, false)
	if got := machine.Observe("source-1", probe.Result{Status: Online, CheckedAt: time.Unix(300, 0)}); got.To != Online {
		t.Fatalf("seeded online status regressed after first probe: %+v", got)
	}
	machine.Seed("source-2", Maintenance, true)
	if got := machine.Observe("source-2", probe.Result{Status: Online, CheckedAt: time.Unix(300, 0)}); got.To != Maintenance {
		t.Fatalf("seeded maintenance status was overwritten: %+v", got)
	}
}

func TestMachineDegradedAndMaintenance(t *testing.T) {
	machine := New(Config{SlowThreshold: 2, SlowAfter: time.Second})
	base := time.Unix(200, 0)
	slow := probe.Result{Status: "online", RegistryMS: 2_000, CheckedAt: base}
	if got := machine.Observe("source-2", slow); got.To != Unknown {
		t.Fatalf("first slow result: %+v", got)
	}
	if got := machine.Observe("source-2", slow); got.To != Degraded || got.Event != "degraded_detected" {
		t.Fatalf("second slow result: %+v", got)
	}
	if got := machine.Observe("source-2", slow); got.To != Degraded || got.Event != "" {
		t.Fatalf("continued slow result should not repeat event: %+v", got)
	}
	machine.SetMaintenance("source-2", true)
	if got := machine.Observe("source-2", probe.Result{Status: "offline", CheckedAt: base.Add(time.Second)}); got.To != Maintenance {
		t.Fatalf("maintenance was overwritten: %+v", got)
	}
	machine.SetMaintenance("source-2", false)
	if machine.Status("source-2") != Unknown {
		t.Fatalf("maintenance exit should reset state")
	}
}

func TestCertificateExpiryProducesAlertEvent(t *testing.T) {
	machine := New(Config{FailureThreshold: 1, RecoveryThreshold: 1})
	result := probe.Result{Status: Online, CheckedAt: time.Now().UTC(), CertificateNotAfter: time.Now().Add(6 * 24 * time.Hour)}
	transition := machine.Observe("tls-source", result)
	if transition.Event != "certificate_expiring_critical" {
		t.Fatalf("event = %q, want critical certificate alert", transition.Event)
	}
	if transition.To != Online {
		t.Fatalf("status = %q, want online", transition.To)
	}
}
