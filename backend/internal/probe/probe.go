package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/registry"
	socksproxy "golang.org/x/net/proxy"
)

type Result struct {
	Status                   string    `json:"status"`
	DNSMS                    int64     `json:"dns_duration_ms"`
	TCPMS                    int64     `json:"tcp_duration_ms"`
	TLSMS                    int64     `json:"tls_duration_ms"`
	RegistryMS               int64     `json:"registry_api_duration_ms"`
	ManifestMS               int64     `json:"manifest_duration_ms"`
	BlobMS                   int64     `json:"blob_duration_ms"`
	BlobBytes                int64     `json:"blob_bytes"`
	Error                    string    `json:"error,omitempty"`
	ErrorStage               string    `json:"error_stage,omitempty"`
	DNSSuccess               bool      `json:"dns_success"`
	ResolvedIPs              []string  `json:"resolved_ips,omitempty"`
	TCPSuccess               bool      `json:"tcp_success"`
	RemoteIP                 string    `json:"remote_ip,omitempty"`
	RemotePort               int       `json:"remote_port,omitempty"`
	TLSSuccess               bool      `json:"tls_success"`
	TLSVersion               string    `json:"tls_version,omitempty"`
	TLSCipher                string    `json:"tls_cipher,omitempty"`
	CertificateSubject       string    `json:"certificate_subject,omitempty"`
	CertificateIssuer        string    `json:"certificate_issuer,omitempty"`
	CertificateNotBefore     time.Time `json:"certificate_not_before,omitempty"`
	CertificateNotAfter      time.Time `json:"certificate_not_after,omitempty"`
	CertificateDaysRemaining int       `json:"certificate_days_remaining,omitempty"`
	RegistrySuccess          bool      `json:"registry_api_success"`
	RegistryStatus           int       `json:"registry_api_status"`
	RegistryAPIVersion       string    `json:"registry_api_version,omitempty"`
	ManifestSuccess          bool      `json:"manifest_success"`
	ManifestStatus           int       `json:"manifest_status"`
	ManifestContentType      string    `json:"manifest_content_type,omitempty"`
	ManifestDigest           string    `json:"manifest_digest,omitempty"`
	ManifestSize             int64     `json:"manifest_size"`
	BlobSuccess              bool      `json:"blob_success"`
	BlobStatus               int       `json:"blob_status"`
	BlobRangeSupported       bool      `json:"blob_range_supported"`
	BlobTTFBMS               int64     `json:"blob_ttfb_ms"`
	BlobSpeedBPS             int64     `json:"blob_speed_bps"`
	DNSError                 string    `json:"dns_error,omitempty"`
	TCPError                 string    `json:"tcp_error,omitempty"`
	TLSError                 string    `json:"tls_error,omitempty"`
	RegistryAPIError         string    `json:"registry_api_error,omitempty"`
	ManifestError            string    `json:"manifest_error,omitempty"`
	BlobError                string    `json:"blob_error,omitempty"`
	CheckedAt                time.Time `json:"checked_at"`
}

// Options controls the repository and bounded blob sample used by a source.
// Zero values intentionally retain the safe global defaults.
type Options struct {
	TestRepository    string
	TestTag           string
	DownloadTestBytes int64
	SkipBlob          bool
}

const (
	ModeRegistry   = "registry"
	ModeManifest   = "manifest"
	ModeHTTP       = "http"
	ModeDockerPull = "docker_pull"
)

func SupportedModes() []string {
	return []string{ModeRegistry, ModeManifest, ModeHTTP, ModeDockerPull}
}

func ValidateTarget(raw string, allowPrivate bool) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return fmt.Errorf("invalid target URL")
	}
	if !allowPrivate && isPrivateHost(u.Hostname()) {
		return fmt.Errorf("private target blocked")
	}
	return nil
}
func isPrivateHost(host string) bool {
	h := strings.ToLower(host)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || h == "metadata.google.internal" {
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func Run(ctx context.Context, raw string, timeout time.Duration) Result {
	return RunWithOptions(ctx, raw, timeout, Options{})
}

func RunWithOptions(ctx context.Context, raw string, timeout time.Duration, options Options) Result {
	result := Result{Status: "offline", CheckedAt: time.Now().UTC()}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if strings.TrimSpace(options.TestRepository) == "" {
		options.TestRepository = "library/alpine"
	}
	if strings.TrimSpace(options.TestTag) == "" {
		options.TestTag = "latest"
	}
	if options.DownloadTestBytes <= 0 || options.DownloadTestBytes > 64<<20 {
		options.DownloadTestBytes = 2 << 20
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	allowPrivate := strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_PRIVATE_TARGETS")), "true")
	if err := ValidateTarget(raw, allowPrivate); err != nil {
		result.Error = err.Error()
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
		result.ErrorStage = "tcp"
		result.Error = "tcp: " + err.Error()
		result.TCPError = result.Error
		return result
	}
	result.TCPSuccess = true
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		result.RemoteIP, result.RemotePort = tcpAddr.IP.String(), tcpAddr.Port
	}
	_ = conn.Close()
	var tlsState tls.ConnectionState
	var traceStage string
	var blobFirstByte time.Time
	var tlsHandshakeStart, tlsHandshakeDone time.Time
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{TLSHandshakeStart: func() { tlsHandshakeStart = time.Now() }, TLSHandshakeDone: func(tls.ConnectionState, error) { tlsHandshakeDone = time.Now() }, GotFirstResponseByte: func() {
		if traceStage == "blob" && blobFirstByte.IsZero() {
			blobFirstByte = time.Now()
		}
	}})
	var pinMu sync.RWMutex
	pinnedIPs := map[string][]net.IP{strings.ToLower(strings.TrimSuffix(u.Hostname(), ".")): append([]net.IP(nil), ips...)}
	pinHost := func(c context.Context, host string) ([]net.IP, error) {
		key := strings.ToLower(strings.TrimSuffix(host, "."))
		pinMu.RLock()
		cached := append([]net.IP(nil), pinnedIPs[key]...)
		pinMu.RUnlock()
		if len(cached) > 0 {
			return cached, nil
		}
		resolved, resolveErr := resolveHost(c, host, allowPrivate)
		if resolveErr != nil {
			return nil, resolveErr
		}
		pinMu.Lock()
		if existing := pinnedIPs[key]; len(existing) > 0 {
			resolved = append([]net.IP(nil), existing...)
		} else {
			pinnedIPs[key] = append([]net.IP(nil), resolved...)
		}
		pinMu.Unlock()
		return resolved, nil
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, VerifyConnection: func(state tls.ConnectionState) error { tlsState = state; return nil }}, Proxy: http.ProxyFromEnvironment, DialContext: func(c context.Context, network, address string) (net.Conn, error) {
		host, service, splitErr := net.SplitHostPort(address)
		if splitErr != nil {
			return nil, splitErr
		}
		resolved, resolveErr := pinHost(c, host)
		if resolveErr != nil {
			return nil, resolveErr
		}
		dialer := &net.Dialer{}
		return dialer.DialContext(c, network, net.JoinHostPort(resolved[0].String(), service))
	}}
	if rawProxy := strings.TrimSpace(os.Getenv("PROBE_SOCKS5_PROXY")); rawProxy != "" {
		proxyURL, parseErr := url.Parse(rawProxy)
		if parseErr != nil || proxyURL.Host == "" || !strings.EqualFold(proxyURL.Scheme, "socks5") {
			result.Error = "invalid SOCKS5 proxy"
			return result
		}
		var auth *socksproxy.Auth
		if proxyURL.User != nil {
			password, _ := proxyURL.User.Password()
			auth = &socksproxy.Auth{User: proxyURL.User.Username(), Password: password}
		}
		dialer, dialErr := socksproxy.SOCKS5("tcp", proxyURL.Host, auth, &net.Dialer{Timeout: timeout})
		if dialErr != nil {
			result.Error = "socks5: " + dialErr.Error()
			return result
		}
		tr.Proxy = nil
		tr.DialContext = func(c context.Context, network, address string) (net.Conn, error) {
			host, service, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				return nil, splitErr
			}
			resolved, resolveErr := pinHost(c, host)
			if resolveErr != nil {
				return nil, resolveErr
			}
			return dialer.Dial(network, net.JoinHostPort(resolved[0].String(), service))
		}
	}
	client := &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("redirect: too many redirects")
		}
		if err := validateRedirectTarget(req.Context(), req.URL, allowPrivate); err != nil {
			return err
		}
		_, err := pinHost(req.Context(), req.URL.Hostname())
		return err
	}}
	probeClient := registry.Client{BaseURL: strings.TrimRight(raw, "/"), HTTPClient: client, UserAgent: "registrypulse/0.1"}
	tlsStart := time.Now()
	registryStart := time.Now()
	result.RegistryStatus, result.RegistryAPIVersion, err = probeClient.CheckV2Details(ctx)
	result.TLSMS = time.Since(tlsStart).Milliseconds()
	if !tlsHandshakeStart.IsZero() && !tlsHandshakeDone.IsZero() {
		result.TLSMS = tlsHandshakeDone.Sub(tlsHandshakeStart).Milliseconds()
	}
	result.RegistryMS = time.Since(registryStart).Milliseconds()
	if err != nil {
		result.ErrorStage = "tls_or_registry"
		result.Error = err.Error()
		if tlsState.Version == 0 {
			result.TLSError = result.Error
		} else {
			result.RegistryAPIError = result.Error
		}
		return result
	}
	result.TLSSuccess = strings.EqualFold(u.Scheme, "https")
	if tlsState.Version != 0 {
		result.TLSVersion = tlsVersionName(tlsState.Version)
		result.TLSCipher = tls.CipherSuiteName(tlsState.CipherSuite)
		result.TLSSuccess = true
		certificateDetails(&result, tlsState.PeerCertificates)
	}
	result.RegistrySuccess = true
	manifestStart := time.Now()
	manifest, err := probeClient.Manifest(ctx, options.TestRepository, options.TestTag)
	result.ManifestMS = time.Since(manifestStart).Milliseconds()
	if err != nil {
		result.ErrorStage = "manifest"
		result.Error = err.Error()
		result.ManifestError = result.Error
		return result
	}
	result.ManifestSuccess, result.ManifestStatus = true, manifest.ManifestStatus
	result.ManifestContentType, result.ManifestDigest = manifest.Manifest.MediaType, manifest.ManifestDigest
	result.ManifestSize = manifest.ManifestSize
	if options.SkipBlob {
		result.Status = "online"
		return result
	}
	if manifest.Manifest.Config.Digest != "" {
		blobStart := time.Now()
		blobStarted := time.Now()
		traceStage = "blob"
		result.BlobStatus, result.BlobBytes, result.BlobRangeSupported, err = probeClient.BlobRange(ctx, options.TestRepository, manifest.Manifest.Config.Digest, options.DownloadTestBytes)
		result.BlobMS = time.Since(blobStart).Milliseconds()
		if !blobFirstByte.IsZero() {
			result.BlobTTFBMS = blobFirstByte.Sub(blobStarted).Milliseconds()
		} else {
			result.BlobTTFBMS = result.BlobMS
		}
		if result.BlobMS > 0 {
			result.BlobSpeedBPS = result.BlobBytes * int64(time.Second) / (result.BlobMS * int64(time.Millisecond))
		}
		if err != nil {
			result.ErrorStage = "blob"
			result.Error = err.Error()
			result.BlobError = result.Error
			return result
		}
		result.BlobSuccess = true
	}
	result.Status = "online"
	return result
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	}
	return fmt.Sprintf("0x%x", version)
}
func certificateDetails(result *Result, certificates []*x509.Certificate) {
	if len(certificates) == 0 {
		return
	}
	cert := certificates[0]
	result.CertificateSubject, result.CertificateIssuer = cert.Subject.String(), cert.Issuer.String()
	result.CertificateNotBefore, result.CertificateNotAfter = cert.NotBefore, cert.NotAfter
	result.CertificateDaysRemaining = int(time.Until(cert.NotAfter).Hours() / 24)
}

func resolveHost(ctx context.Context, host string, allowPrivate bool) ([]net.IP, error) {
	family := strings.ToLower(strings.TrimSpace(os.Getenv("PROBE_IP_FAMILY")))
	network := "ip"
	if family == "4" || family == "ipv4" {
		network = "ip4"
	}
	if family == "6" || family == "ipv6" {
		network = "ip6"
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, network, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no address for %s", host)
	}
	if !allowPrivate {
		for _, ip := range ips {
			if isPrivateHost(ip.String()) {
				return nil, fmt.Errorf("ssrf: resolved private address blocked")
			}
		}
	}
	return ips, nil
}

func validateRedirectTarget(ctx context.Context, target *url.URL, allowPrivate bool) error {
	if target == nil {
		return fmt.Errorf("redirect target is missing")
	}
	if err := ValidateTarget(target.String(), allowPrivate); err != nil {
		return fmt.Errorf("redirect: %w", err)
	}
	_, err := resolveHost(ctx, target.Hostname(), allowPrivate)
	return err
}

func port(u *url.URL) string {
	if u.Port() != "" {
		return u.Port()
	}
	if u.Scheme == "http" {
		return "80"
	}
	return "443"
}
func errString(err error) string {
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}
