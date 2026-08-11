package proxy

import "testing"

func TestParsePrometheusMetrics(t *testing.T) {
	body := []byte(`# HELP registrypulse_proxy_requests_total Total proxy requests.
registrypulse_proxy_requests_total 12
registrypulse_proxy_success_total 8
registrypulse_proxy_upstream_failure_total 3
registrypulse_proxy_retries_total 2
registrypulse_proxy_redirects_total 1
registrypulse_proxy_active_requests 4
registrypulse_proxy_bytes_forwarded_total 1048576
registrypulse_proxy_responses_total{class="1xx"} 0
registrypulse_proxy_responses_total{class="2xx"} 8
registrypulse_proxy_responses_total{class="3xx"} 1
registrypulse_proxy_responses_total{class="4xx"} 2
registrypulse_proxy_responses_total{class="5xx"} 1
registrypulse_proxy_request_duration_seconds 0.125
`)

	got := ParsePrometheusMetrics(body)
	if got.Requests != 12 || got.Successes != 8 || got.UpstreamFailures != 3 || got.Retries != 2 || got.Redirects != 1 {
		t.Fatalf("unexpected counters: %+v", got)
	}
	if got.ActiveRequests != 4 || got.BytesForwarded != 1048576 {
		t.Fatalf("unexpected gauges: %+v", got)
	}
	if got.Responses2xx != 8 || got.Responses4xx != 2 || got.Responses5xx != 1 || got.AverageDurationSeconds != 0.125 {
		t.Fatalf("unexpected response metrics: %+v", got)
	}
}

func TestParsePrometheusMetricsIgnoresInvalidAndUnknownLines(t *testing.T) {
	got := ParsePrometheusMetrics([]byte("unknown_metric 9\nregistrypulse_proxy_requests_total nope\nmalformed\n"))
	if got.Requests != 0 {
		t.Fatalf("invalid values should be ignored: %+v", got)
	}
}
