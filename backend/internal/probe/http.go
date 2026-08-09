package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"time"
)

// RunHTTP checks an ordinary HTTP(S) endpoint. It is intended for categories
// whose published endpoint is a web/API gateway rather than an OCI registry.
// A 401/403/404 still proves that the endpoint is reachable, while 5xx is
// reported as unavailable.
func RunHTTP(ctx context.Context, raw string, timeout time.Duration) Result {
	result := Result{Status: "offline", CheckedAt: time.Now().UTC()}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	allowPrivate := strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_PRIVATE_TARGETS")), "true")
	if err := ValidateTarget(raw, allowPrivate); err != nil {
		result.ErrorStage, result.Error = "http", err.Error()
		return result
	}
	u, _ := url.Parse(raw)
	dnsStart := time.Now()
	ips, err := resolveHost(ctx, u.Hostname(), allowPrivate)
	result.DNSMS = time.Since(dnsStart).Milliseconds()
	if err != nil || len(ips) == 0 {
		result.ErrorStage = "dns"
		result.Error = "dns: " + errString(err)
		result.DNSError = result.Error
		return result
	}
	result.DNSSuccess = true
	for _, ip := range ips {
		result.ResolvedIPs = append(result.ResolvedIPs, ip.String())
	}
	tcpStart := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(ips[0].String(), port(u)))
	result.TCPMS = time.Since(tcpStart).Milliseconds()
	if err != nil {
		result.ErrorStage, result.Error = "tcp", "tcp: "+err.Error()
		result.TCPError = result.Error
		return result
	}
	result.TCPSuccess = true
	result.RemoteIP, result.RemotePort = ips[0].String(), portNumber(u)
	_ = conn.Close()

	var tlsState tls.ConnectionState
	var tlsStart, tlsDone time.Time
	trace := &httptrace.ClientTrace{
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(state tls.ConnectionState, handshakeErr error) {
			tlsDone = time.Now()
			tlsState = state
			if handshakeErr == nil && state.Version != 0 {
				result.TLSSuccess = true
				result.TLSVersion = tlsVersionName(state.Version)
				result.TLSCipher = tls.CipherSuiteName(state.CipherSuite)
				certificateDetails(&result, state.PeerCertificates)
			}
		},
	}
	transport := &http.Transport{
		Proxy:             nil,
		ForceAttemptHTTP2: true,
		TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname()},
		DialContext: func(c context.Context, network, address string) (net.Conn, error) {
			_, service, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				return nil, splitErr
			}
			return (&net.Dialer{}).DialContext(c, network, net.JoinHostPort(ips[0].String(), service))
		},
	}
	client := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	start := time.Now()
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, strings.TrimRight(raw, "/")+"/", nil)
	if err != nil {
		result.ErrorStage, result.Error = "http", err.Error()
		return result
	}
	req.Header.Set("User-Agent", "registrypulse/1.0")
	resp, err := client.Do(req)
	result.RegistryMS = time.Since(start).Milliseconds()
	if !tlsStart.IsZero() && !tlsDone.IsZero() {
		result.TLSMS = tlsDone.Sub(tlsStart).Milliseconds()
	}
	if err != nil {
		result.ErrorStage, result.Error = "http", "http: "+err.Error()
		if !result.TLSSuccess && strings.EqualFold(u.Scheme, "https") {
			result.TLSError = result.Error
		}
		return result
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	result.RegistryStatus = resp.StatusCode
	if tlsState.Version != 0 {
		result.TLSSuccess = true
	}
	if resp.StatusCode >= 500 {
		result.ErrorStage = "http"
		result.Error = fmt.Sprintf("http returned %d", resp.StatusCode)
		return result
	}
	result.Status = "online"
	return result
}

func portNumber(u *url.URL) int {
	if value := u.Port(); value != "" {
		var port int
		_, _ = fmt.Sscanf(value, "%d", &port)
		return port
	}
	if u.Scheme == "http" {
		return 80
	}
	return 443
}
