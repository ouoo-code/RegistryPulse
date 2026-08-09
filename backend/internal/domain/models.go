package domain

import "time"

type Category struct {
	ID                    string    `json:"id"`
	Slug                  string    `json:"slug"`
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	Icon                  string    `json:"icon"`
	OfficialURL           string    `json:"official_url"`
	DefaultTestRepository string    `json:"default_test_repository"`
	DefaultTestTag        string    `json:"default_test_tag"`
	DefaultTestImageID    string    `json:"default_test_image_id,omitempty"`
	DefaultProbeMode      string    `json:"default_probe_mode"`
	DefaultTimeoutSeconds int       `json:"default_timeout_seconds"`
	DefaultManifestPath   string    `json:"default_manifest_path"`
	AuthType              string    `json:"auth_type"`
	Enabled               bool      `json:"enabled"`
	SortOrder             int       `json:"sort_order"`
	CreatedAt             time.Time `json:"created_at"`
}

type TestImage struct {
	ID           string    `json:"id"`
	Reference    string    `json:"reference"`
	Enabled      bool      `json:"enabled"`
	MaxBytes     int64     `json:"max_bytes"`
	IsDefault    bool      `json:"is_default"`
	AuthStrategy string    `json:"auth_strategy"`
	CategoryIDs  []string  `json:"category_ids,omitempty"`
	ProbeModes   []string  `json:"probe_modes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CredentialProfile struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	AuthType     string    `json:"auth_type"`
	Username     string    `json:"username,omitempty"`
	SourceID     string    `json:"source_id,omitempty"`
	RegistryHost string    `json:"registry_host,omitempty"`
	CategoryID   string    `json:"category_id,omitempty"`
	Enabled      bool      `json:"enabled"`
	HasSecret    bool      `json:"has_secret"`
	SecretMasked string    `json:"secret_masked,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CredentialProfileInput struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AuthType     string `json:"auth_type"`
	Username     string `json:"username"`
	Secret       string `json:"secret"`
	ClearSecret  bool   `json:"clear_secret"`
	SourceID     string `json:"source_id"`
	RegistryHost string `json:"registry_host"`
	CategoryID   string `json:"category_id"`
	Enabled      *bool  `json:"enabled"`
}

type ResolvedCredential struct {
	AuthType string
	Username string
	Secret   string
}

type Source struct {
	ID                    string    `json:"id"`
	CategoryID            string    `json:"category_id"`
	Name                  string    `json:"name"`
	BaseURL               string    `json:"base_url"`
	DisplayURL            string    `json:"display_url"`
	RegistryHost          string    `json:"registry_host"`
	Description           string    `json:"description"`
	Provider              string    `json:"provider"`
	Country               string    `json:"country"`
	Region                string    `json:"region"`
	Operator              string    `json:"operator"`
	Tags                  []string  `json:"tags"`
	IsOfficial            bool      `json:"is_official"`
	IsCloudflare          bool      `json:"is_cloudflare"`
	IsRecommended         bool      `json:"is_recommended"`
	Enabled               bool      `json:"enabled"`
	Priority              int       `json:"priority"`
	SortOrder             int       `json:"sort_order"`
	Maintenance           bool      `json:"maintenance"`
	ProbeConfigCustom     bool      `json:"probe_config_custom"`
	ProbeMode             string    `json:"probe_mode"`
	TestRepository        string    `json:"test_repository"`
	TestTag               string    `json:"test_tag"`
	TestDigest            string    `json:"test_digest"`
	RequestTimeout        int       `json:"request_timeout_seconds"`
	DownloadTestBytes     int64     `json:"download_test_bytes"`
	TestImageID           string    `json:"test_image_id,omitempty"`
	TestImageReference    string    `json:"test_image_reference,omitempty"`
	TestImageMaxBytes     int64     `json:"test_image_max_bytes,omitempty"`
	TestImageAuthStrategy string    `json:"test_image_auth_strategy,omitempty"`
	Status                string    `json:"status"`
	ResponseMS            int64     `json:"response_ms"`
	LastChecked           time.Time `json:"last_checked"`
	Error                 string    `json:"error,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type ProbeResult struct {
	ID                       string    `json:"id"`
	SourceID                 string    `json:"source_id"`
	ProbeNodeID              string    `json:"probe_node_id,omitempty"`
	TaskID                   string    `json:"task_id,omitempty"`
	Status                   string    `json:"status"`
	DNSMS                    int64     `json:"dns_duration_ms"`
	TCPMS                    int64     `json:"tcp_duration_ms"`
	TLSMS                    int64     `json:"tls_duration_ms"`
	RegistryMS               int64     `json:"registry_duration_ms"`
	ManifestMS               int64     `json:"manifest_duration_ms"`
	BlobMS                   int64     `json:"blob_duration_ms"`
	BlobBytes                int64     `json:"blob_bytes"`
	ResponseMS               int64     `json:"response_ms"`
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

// ProbeNode is the registration and liveness record for a remote probe agent.
type ProbeNode struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Region       string    `json:"region"`
	Version      string    `json:"version"`
	Capabilities []string  `json:"capabilities"`
	Status       string    `json:"status"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ProbeTask struct {
	ID          string         `json:"id"`
	SourceID    string         `json:"source_id"`
	ProbeNodeID string         `json:"probe_node_id,omitempty"`
	Type        string         `json:"type"`
	Payload     map[string]any `json:"payload,omitempty"`
	Status      string         `json:"status"`
	LeaseUntil  time.Time      `json:"lease_until,omitempty"`
	StartedAt   time.Time      `json:"started_at,omitempty"`
	FinishedAt  time.Time      `json:"finished_at,omitempty"`
	Result      *ProbeResult   `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type AgentRegisterInput struct {
	Name         string   `json:"name"`
	Region       string   `json:"region"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

type AgentHeartbeatInput struct {
	Status       string   `json:"status"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

type AgentPollInput struct {
	Limit int `json:"limit"`
}

type AgentResultInput struct {
	Status     string `json:"status"`
	SourceID   string `json:"source_id"`
	Error      string `json:"error,omitempty"`
	DNSMS      int64  `json:"dns_duration_ms"`
	TCPMS      int64  `json:"tcp_duration_ms"`
	TLSMS      int64  `json:"tls_duration_ms"`
	RegistryMS int64  `json:"registry_duration_ms"`
	ManifestMS int64  `json:"manifest_duration_ms"`
	BlobMS     int64  `json:"blob_duration_ms"`
	BlobBytes  int64  `json:"blob_bytes"`
	HTTPStatus int    `json:"http_status"`
}

type AgentFailureInput struct {
	Error string `json:"error"`
}

type SourceInput struct {
	CategoryID        string   `json:"category_id"`
	Name              string   `json:"name"`
	BaseURL           string   `json:"base_url"`
	DisplayURL        string   `json:"display_url"`
	Description       string   `json:"description"`
	Provider          string   `json:"provider"`
	Country           string   `json:"country"`
	Region            string   `json:"region"`
	Operator          string   `json:"operator"`
	Tags              []string `json:"tags"`
	IsOfficial        *bool    `json:"is_official"`
	IsCloudflare      *bool    `json:"is_cloudflare"`
	IsRecommended     *bool    `json:"is_recommended"`
	Enabled           *bool    `json:"enabled"`
	Priority          *int     `json:"priority"`
	SortOrder         *int     `json:"sort_order"`
	Maintenance       *bool    `json:"maintenance"`
	ProbeConfigCustom bool     `json:"probe_config_custom"`
	ProbeMode         string   `json:"probe_mode"`
	TestRepository    string   `json:"test_repository"`
	TestTag           string   `json:"test_tag"`
	TestDigest        string   `json:"test_digest"`
	RequestTimeout    *int     `json:"request_timeout_seconds"`
	DownloadTestBytes *int64   `json:"download_test_bytes"`
	TestImageID       *string  `json:"test_image_id"`
}
