package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// Registry contains process-local counters for the worker. It deliberately has
// no Prometheus dependency so the worker can be embedded in the API process or
// exposed through a small adapter later.
type Registry struct {
	probes, successes, failures, incidents, notifications, queue, active             atomic.Int64
	lockAttempts, lockAcquired, lockSkipped, lockErrors, resultRetries, resultErrors atomic.Int64
	durationNanos, httpRequests, httpDurationNanos                                   atomic.Int64
}

func New() *Registry              { return &Registry{} }
func (r *Registry) ProbeStarted() { r.probes.Add(1); r.active.Add(1) }
func (r *Registry) ProbeFinished(success bool, d time.Duration) {
	r.active.Add(-1)
	r.durationNanos.Add(d.Nanoseconds())
	if success {
		r.successes.Add(1)
	} else {
		r.failures.Add(1)
	}
}
func (r *Registry) SetQueueSize(n int) { r.queue.Store(int64(n)) }
func (r *Registry) Incident()          { r.incidents.Add(1) }
func (r *Registry) Notification()      { r.notifications.Add(1) }
func (r *Registry) LockAttempt()       { r.lockAttempts.Add(1) }
func (r *Registry) LockAcquired()      { r.lockAcquired.Add(1) }
func (r *Registry) LockSkipped()       { r.lockSkipped.Add(1) }
func (r *Registry) LockError()         { r.lockErrors.Add(1) }
func (r *Registry) ResultRetry()       { r.resultRetries.Add(1) }
func (r *Registry) ResultError()       { r.resultErrors.Add(1) }
func (r *Registry) HTTPRequestFinished(d time.Duration) {
	r.httpRequests.Add(1)
	r.httpDurationNanos.Add(d.Nanoseconds())
}

func (r *Registry) Active() int64 { return r.active.Load() }

// Prometheus returns a stable text exposition suitable for /metrics.
func (r *Registry) Prometheus() string {
	var b strings.Builder
	write := func(name string, value int64, help string) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
	}
	write("registry_monitor_probe_total", r.probes.Load(), "Total probe attempts.")
	write("registry_monitor_probe_success_total", r.successes.Load(), "Successful probe attempts.")
	write("registry_monitor_probe_failure_total", r.failures.Load(), "Failed probe attempts.")
	write("registry_monitor_incident_total", r.incidents.Load(), "Incident state transitions.")
	write("registry_monitor_notification_total", r.notifications.Load(), "Notifications emitted.")
	write("registry_monitor_lock_attempt_total", r.lockAttempts.Load(), "Distributed lock attempts.")
	write("registry_monitor_lock_acquired_total", r.lockAcquired.Load(), "Distributed locks acquired.")
	write("registry_monitor_lock_skipped_total", r.lockSkipped.Load(), "Runs or sources skipped because a lock was held.")
	write("registry_monitor_lock_error_total", r.lockErrors.Load(), "Distributed lock errors.")
	write("registry_monitor_result_retry_total", r.resultRetries.Load(), "Probe result persistence retries.")
	write("registry_monitor_result_error_total", r.resultErrors.Load(), "Probe result persistence failures.")
	fmt.Fprintf(&b, "registry_monitor_probe_queue_size %d\nregistry_monitor_active_workers %d\n", r.queue.Load(), r.active.Load())
	fmt.Fprintln(&b, "registry_monitor_registry_status{status=\"online\"} 0")
	fmt.Fprintf(&b, "registry_monitor_http_request_total %d\n", r.httpRequests.Load())
	fmt.Fprintf(&b, "registry_monitor_http_request_duration_seconds %.6f\n", float64(r.httpDurationNanos.Load())/float64(time.Second))
	count := r.probes.Load()
	avg := float64(0)
	if count > 0 {
		avg = float64(r.durationNanos.Load()) / float64(count) / float64(time.Second)
	}
	fmt.Fprintf(&b, "registry_monitor_probe_duration_seconds %.6f\n", avg)
	return b.String()
}

// Names is useful for smoke tests and adapters that need to validate the
// metrics contract without parsing exposition text.
func Names() []string {
	names := []string{"registry_monitor_probe_total", "registry_monitor_probe_success_total", "registry_monitor_probe_failure_total", "registry_monitor_probe_duration_seconds", "registry_monitor_probe_queue_size", "registry_monitor_active_workers", "registry_monitor_registry_status", "registry_monitor_incident_total", "registry_monitor_notification_total", "registry_monitor_lock_attempt_total", "registry_monitor_lock_acquired_total", "registry_monitor_lock_skipped_total", "registry_monitor_lock_error_total", "registry_monitor_result_retry_total", "registry_monitor_result_error_total", "registry_monitor_http_request_total", "registry_monitor_http_request_duration_seconds"}
	sort.Strings(names)
	return names
}
