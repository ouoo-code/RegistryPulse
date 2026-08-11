package proxy

import (
	"bufio"
	"strconv"
	"strings"
	"time"
)

// MetricsSnapshot is the small, UI-safe subset of the proxy Prometheus
// metrics that is exposed through the authenticated admin API.
type MetricsSnapshot struct {
	Requests               int64     `json:"requests"`
	Successes              int64     `json:"successes"`
	UpstreamFailures       int64     `json:"upstream_failures"`
	Retries                int64     `json:"retries"`
	Redirects              int64     `json:"redirects"`
	ActiveRequests         int64     `json:"active_requests"`
	BytesForwarded         int64     `json:"bytes_forwarded"`
	Responses1xx           int64     `json:"responses_1xx"`
	Responses2xx           int64     `json:"responses_2xx"`
	Responses3xx           int64     `json:"responses_3xx"`
	Responses4xx           int64     `json:"responses_4xx"`
	Responses5xx           int64     `json:"responses_5xx"`
	AverageDurationSeconds float64   `json:"average_duration_seconds"`
	CollectedAt            time.Time `json:"collected_at"`
}

// ParsePrometheusMetrics parses the proxy's own text exposition format. It
// intentionally ignores unknown metrics so adding a metric to the proxy does
// not break the admin monitor.
func ParsePrometheusMetrics(body []byte) MetricsSnapshot {
	var snapshot MetricsSnapshot
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		if err != nil {
			continue
		}
		metric := fields[0]
		switch {
		case metric == "registrypulse_proxy_requests_total":
			snapshot.Requests = int64(value)
		case metric == "registrypulse_proxy_success_total":
			snapshot.Successes = int64(value)
		case metric == "registrypulse_proxy_upstream_failure_total":
			snapshot.UpstreamFailures = int64(value)
		case metric == "registrypulse_proxy_retries_total":
			snapshot.Retries = int64(value)
		case metric == "registrypulse_proxy_redirects_total":
			snapshot.Redirects = int64(value)
		case metric == "registrypulse_proxy_active_requests":
			snapshot.ActiveRequests = int64(value)
		case metric == "registrypulse_proxy_bytes_forwarded_total":
			snapshot.BytesForwarded = int64(value)
		case strings.HasPrefix(metric, "registrypulse_proxy_responses_total{"):
			switch {
			case strings.Contains(metric, `class="1xx"`):
				snapshot.Responses1xx = int64(value)
			case strings.Contains(metric, `class="2xx"`):
				snapshot.Responses2xx = int64(value)
			case strings.Contains(metric, `class="3xx"`):
				snapshot.Responses3xx = int64(value)
			case strings.Contains(metric, `class="4xx"`):
				snapshot.Responses4xx = int64(value)
			case strings.Contains(metric, `class="5xx"`):
				snapshot.Responses5xx = int64(value)
			}
		case metric == "registrypulse_proxy_request_duration_seconds":
			snapshot.AverageDurationSeconds = value
		}
	}
	return snapshot
}
