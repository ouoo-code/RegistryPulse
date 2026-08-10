package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/auth"
	"github.com/ouoo-code/RegistryPulse/internal/database"
	"github.com/ouoo-code/RegistryPulse/internal/domain"
	metricspkg "github.com/ouoo-code/RegistryPulse/internal/metrics"
	"github.com/ouoo-code/RegistryPulse/internal/probe"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	store          domain.Store
	adminToken     string
	sessions       *auth.SessionStore
	agents         domain.AgentRegistry
	metricRegistry *metricspkg.Registry
	replayMu       sync.Mutex
	replay         map[string]time.Time
	loginMu        sync.Mutex
	loginFailures  map[string]loginFailure
	rateMu         sync.Mutex
	rateRequests   map[string][]time.Time
	readyCheck     func(context.Context) error
	redis          redis.UniversalClient
}

func New(store domain.Store, sessions ...*auth.SessionStore) *Server {
	var sessionStore *auth.SessionStore
	if len(sessions) > 0 {
		sessionStore = sessions[0]
	}
	return NewWithAgentRegistry(store, sessionStore, domain.NewMemoryAgentRegistry())
}

func NewWithAgentRegistry(store domain.Store, sessions *auth.SessionStore, agents domain.AgentRegistry) *Server {
	if agents == nil {
		agents = domain.NewMemoryAgentRegistry()
	}
	return &Server{store: store, adminToken: os.Getenv("ADMIN_API_TOKEN"), sessions: sessions, agents: agents, metricRegistry: metricspkg.New(), replay: make(map[string]time.Time), loginFailures: make(map[string]loginFailure), rateRequests: make(map[string][]time.Time)}
}

// SetReadinessChecker lets the process entrypoint include external
// dependencies (PostgreSQL, Redis and task dispatch) in /health/ready without
// coupling the HTTP package to a particular deployment.
func (s *Server) SetReadinessChecker(check func(context.Context) error) { s.readyCheck = check }

func (s *Server) SetRedisClient(client redis.UniversalClient) { s.redis = client }

type loginFailure struct {
	Count int
	Since time.Time
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", s.live)
	mux.HandleFunc("/health/ready", s.ready)
	mux.HandleFunc("/metrics", s.metrics)
	mux.HandleFunc("/api/v1/health", s.health)
	mux.HandleFunc("/api/v1/auth/login", s.login)
	mux.HandleFunc("/api/v1/auth/logout", s.logout)
	mux.HandleFunc("/api/v1/auth/me", s.me)
	mux.HandleFunc("/api/v1/auth/change-password", s.changePassword)
	mux.HandleFunc("/api/v1/public/summary", s.summary)
	mux.HandleFunc("/api/v1/public/categories", s.categories)
	mux.HandleFunc("/api/v1/public/categories/", s.category)
	mux.HandleFunc("/api/v1/public/sources", s.sources)
	mux.HandleFunc("/api/v1/public/sources/", s.source)
	mux.HandleFunc("/api/v1/public/probes", s.probes)
	mux.HandleFunc("/api/v1/public/config-generator/options", s.configOptions)
	mux.HandleFunc("/api/v1/public/config-generator/render", s.renderConfig)
	mux.HandleFunc("/api/v1/admin/sources", s.adminSources)
	mux.HandleFunc("/api/v1/admin/sources/", s.adminSource)
	mux.HandleFunc("/api/v1/admin/categories", s.adminCategories)
	mux.HandleFunc("/api/v1/admin/categories/", s.adminCategories)
	mux.HandleFunc("/api/v1/admin/tasks", s.adminTasks)
	mux.HandleFunc("/api/v1/admin/tasks/", s.adminTasks)
	mux.HandleFunc("/api/v1/admin/results", s.adminResults)
	mux.HandleFunc("/api/v1/admin/incidents", s.adminIncidents)
	mux.HandleFunc("/api/v1/admin/probes", s.adminProbes)
	mux.HandleFunc("/api/v1/admin/probes/", s.adminProbes)
	mux.HandleFunc("/api/v1/admin/settings", s.adminSettings)
	mux.HandleFunc("/api/v1/admin/proxy", s.adminProxy)
	mux.HandleFunc("/api/v1/admin/totp", s.adminTOTP)
	mux.HandleFunc("/api/v1/admin/test-images", s.adminTestImages)
	mux.HandleFunc("/api/v1/admin/test-images/", s.adminTestImages)
	mux.HandleFunc("/api/v1/admin/credential-profiles", s.adminCredentialProfiles)
	mux.HandleFunc("/api/v1/admin/credential-profiles/", s.adminCredentialProfiles)
	mux.HandleFunc("/api/v1/admin/notifications", s.adminNotifications)
	mux.HandleFunc("/api/v1/admin/notifications/", s.adminNotifications)
	mux.HandleFunc("/api/v1/admin/notification-rules", s.adminNotificationRules)
	mux.HandleFunc("/api/v1/admin/notification-rules/", s.adminNotificationRules)
	mux.HandleFunc("/api/v1/admin/users", s.adminUsers)
	mux.HandleFunc("/api/v1/admin/users/", s.adminUsers)
	mux.HandleFunc("/api/v1/admin/roles", s.adminRoles)
	mux.HandleFunc("/api/v1/admin/roles/", s.adminRoles)
	mux.HandleFunc("/api/v1/admin/audit-logs", s.auditLogs)
	mux.HandleFunc("/api/v1/agent/register", s.agentRegister)
	mux.HandleFunc("/api/v1/agent/heartbeat", s.agentHeartbeat)
	mux.HandleFunc("/api/v1/agent/tasks/poll", s.agentPoll)
	mux.HandleFunc("/api/v1/agent/tasks/", s.agentTask)
	return requestID(s.logging(s.security(mux)))
}

func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && corsAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token, X-Agent-Timestamp, X-Agent-Nonce, X-Agent-Signature")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			if r.Header.Get("Origin") == "" || corsAllowed(r.Header.Get("Origin")) {
				w.WriteHeader(http.StatusNoContent)
			} else {
				writeError(w, http.StatusForbidden, "CORS_ORIGIN_DENIED", "origin is not allowed")
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/public/") && !s.allowPublicRequest(r) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "public API rate limit exceeded")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && r.URL.Path != "/api/v1/auth/login" && r.Header.Get("Authorization") == "" {
			if cookie, err := r.Cookie("session"); err == nil && cookie.Value != "" {
				csrf, csrfErr := r.Cookie("csrf_token")
				if csrfErr != nil || csrf.Value == "" || csrf.Value != r.Header.Get("X-CSRF-Token") {
					writeError(w, http.StatusForbidden, "CSRF_TOKEN_REQUIRED", "csrf token is required")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) allowPublicRequest(r *http.Request) bool {
	limit := 120
	if raw, err := strconv.Atoi(os.Getenv("PUBLIC_API_RATE_LIMIT")); err == nil && raw > 0 {
		limit = raw
	}
	key := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(key); err == nil {
		key = host
	} else if strings.HasPrefix(key, "[") && strings.Contains(key, "]") {
		key = strings.Trim(key[1:strings.IndexByte(key, ']')], " ")
	}
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	window := now.Add(-time.Minute)
	items := s.rateRequests[key]
	cut := 0
	for cut < len(items) && items[cut].Before(window) {
		cut++
	}
	items = items[cut:]
	if len(items) >= limit {
		s.rateRequests[key] = items
		return false
	}
	s.rateRequests[key] = append(items, now)
	return true
}

func corsAllowed(origin string) bool {
	for _, allowed := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}
func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeData(w, map[string]string{"status": "ok"}, nil)
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.readyCheck != nil {
		if err := s.readyCheck(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "NOT_READY", "required service is not ready")
			return
		}
	}
	writeData(w, map[string]string{"status": "ready"}, nil)
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeData(w, map[string]string{"status": "ok"}, nil)
}
func (s *Server) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(s.metricRegistry.Prometheus()))
}
func (s *Server) summary(w http.ResponseWriter, _ *http.Request) {
	counts := map[string]int{"total": 0}
	var lastUpdated *time.Time
	for _, v := range s.store.Sources() {
		if !v.Enabled {
			continue
		}
		counts[v.Status]++
		counts["total"]++
		if !v.LastChecked.IsZero() && (lastUpdated == nil || v.LastChecked.After(*lastUpdated)) {
			checkedAt := v.LastChecked
			lastUpdated = &checkedAt
		}
	}
	writeData(w, map[string]any{"counts": counts, "last_updated": lastUpdated}, nil)
}
func (s *Server) categories(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/public/categories" {
		s.category(w, r)
		return
	}
	writeData(w, s.store.Categories(), map[string]any{"count": len(s.store.Categories())})
}
func (s *Server) category(w http.ResponseWriter, r *http.Request) {
	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/public/categories/"), "/")
	if slug == "" {
		writeError(w, 400, "INVALID_CATEGORY", "category slug is required")
		return
	}
	v, err := s.store.Category(slug)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, 404, "CATEGORY_NOT_FOUND", "category not found")
		return
	}
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	sources := filterEnabledSources(filterSources(s.store.Sources(), "", v.ID))
	writeData(w, map[string]any{"category": v, "sources": sources}, map[string]any{"count": len(sources)})
}
func (s *Server) sources(w http.ResponseWriter, r *http.Request) {
	q, cat := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q"))), strings.TrimSpace(r.URL.Query().Get("category"))
	if cat != "" {
		// Public URLs may use either the stable category ID or its display slug.
		// Resolve the latter before filtering sources, whose foreign key remains
		// the stable ID by design.
		if category, err := s.store.Category(cat); err == nil {
			cat = category.ID
		}
	}
	out := filterEnabledSources(filterSources(s.store.Sources(), q, cat))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" {
		filtered := out[:0]
		for _, source := range out {
			if source.Status == status {
				filtered = append(filtered, source)
			}
		}
		out = filtered
	}
	sortSources(out, r.URL.Query().Get("sort"), r.URL.Query().Get("order"))
	page, pageSize := queryPage(r)
	total := len(out)
	pages := (total + pageSize - 1) / pageSize
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	writeData(w, out[start:end], map[string]any{"count": total, "page": page, "page_size": pageSize, "pages": pages})
}

func queryPage(r *http.Request) (int, int) {
	page, pageSize := 1, 100
	if raw, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && raw > 0 {
		page = raw
	}
	if raw, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && raw > 0 {
		pageSize = raw
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func sortSources(items []domain.Source, field, order string) {
	sort.SliceStable(items, func(i, j int) bool {
		if field == "response_ms" {
			if items[i].ResponseMS == items[j].ResponseMS {
				return items[i].ID < items[j].ID
			}
			if strings.EqualFold(order, "desc") {
				return items[i].ResponseMS > items[j].ResponseMS
			}
			return items[i].ResponseMS < items[j].ResponseMS
		}
		var left, right string
		switch field {
		case "status":
			left, right = items[i].Status, items[j].Status
		default:
			left, right = strings.ToLower(items[i].Name), strings.ToLower(items[j].Name)
		}
		if left == right {
			return items[i].ID < items[j].ID
		}
		if strings.EqualFold(order, "desc") {
			return left > right
		}
		return left < right
	})
}
func (s *Server) source(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/public/sources/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, 400, "INVALID_SOURCE", "source id is required")
		return
	}
	v, err := s.store.Source(parts[0])
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, 404, "SOURCE_NOT_FOUND", "source not found")
		return
	}
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if len(parts) > 1 && parts[1] == "history" {
		writeData(w, s.store.History(parts[0], 50), nil)
		return
	}
	if len(parts) > 1 && parts[1] == "aggregates" {
		if reader, ok := s.store.(interface {
			SourceAggregates(context.Context, string) (database.SourceAggregates, error)
		}); ok {
			aggregates, aggregateErr := reader.SourceAggregates(r.Context(), parts[0])
			if aggregateErr != nil {
				writeError(w, 500, "AGGREGATE_QUERY_FAILED", "could not query source aggregates")
				return
			}
			writeData(w, aggregates, nil)
			return
		}
		writeData(w, map[string]any{"hourly": []any{}, "daily": []any{}}, nil)
		return
	}
	if len(parts) > 1 && parts[1] == "incidents" {
		s.sourceIncidents(w, r, parts[0])
		return
	}
	writeData(w, v, nil)
}
func (s *Server) probes(w http.ResponseWriter, _ *http.Request) {
	items := make([]domain.ProbeResult, 0)
	for _, source := range s.store.Sources() {
		items = append(items, s.store.History(source.ID, 200)...)
	}
	writeData(w, items, map[string]any{"count": len(items)})
}
func filterSources(in []domain.Source, q, cat string) []domain.Source {
	out := make([]domain.Source, 0, len(in))
	for _, v := range in {
		hay := strings.ToLower(v.Name + " " + v.BaseURL + " " + v.Provider)
		if (q == "" || strings.Contains(hay, q)) && (cat == "" || v.CategoryID == cat) {
			out = append(out, v)
		}
	}
	return out
}

// Public directories represent active monitoring targets only. Disabled
// sources remain visible in the admin API, but must not appear as offline
// targets in the public site or contribute to public statistics.
func filterEnabledSources(in []domain.Source) []domain.Source {
	out := make([]domain.Source, 0, len(in))
	for _, source := range in {
		if source.Enabled {
			out = append(out, source)
		}
	}
	return out
}

func (s *Server) adminSources(w http.ResponseWriter, r *http.Request) {
	permission := "source.write"
	if r.Method == http.MethodGet {
		permission = "source.read"
	}
	user, ok := s.requirePermission(w, r, permission)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		writeData(w, s.store.Sources(), nil)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	var in domain.SourceInput
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Name == "" || in.BaseURL == "" {
		writeError(w, 400, "INVALID_SOURCE", "name and base_url are required")
		return
	}
	if err := validateSourceInput(in); err != nil {
		writeError(w, 400, "INVALID_SOURCE_URL", err.Error())
		return
	}
	v, err := s.store.UpsertSource(in, "")
	if err != nil {
		if writeSourceSaveError(w, err) {
			return
		}
		writeError(w, 500, "INTERNAL_ERROR", "could not create source")
		return
	}
	s.audit(r, user, "source.create", v.ID, in)
	writeJSON(w, 201, v, nil)
}
func (s *Server) adminSource(w http.ResponseWriter, r *http.Request) {
	restPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/sources/"), "/")
	if restPath == "test" && r.Method == http.MethodPost {
		user, ok := s.requirePermission(w, r, "probe.write")
		if !ok {
			return
		}
		var input struct {
			BaseURL               string `json:"base_url"`
			ProbeMode             string `json:"probe_mode"`
			TestRepository        string `json:"test_repository"`
			TestTag               string `json:"test_tag"`
			TestImageReference    string `json:"test_image_reference"`
			RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
			DownloadTestBytes     int64  `json:"download_test_bytes"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
			return
		}
		input.BaseURL = strings.TrimSpace(input.BaseURL)
		if input.BaseURL == "" {
			writeError(w, http.StatusBadRequest, "INVALID_SOURCE_URL", "base_url is required")
			return
		}
		if err := validateSourceInput(domain.SourceInput{BaseURL: input.BaseURL}); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_SOURCE_URL", err.Error())
			return
		}
		mode := strings.TrimSpace(input.ProbeMode)
		if mode == "" {
			mode = probe.ModeRegistry
		}
		if !supportedProbeMode(mode) {
			writeError(w, http.StatusBadRequest, "INVALID_PROBE_MODE", "unsupported probe mode")
			return
		}
		timeoutSeconds := input.RequestTimeoutSeconds
		if timeoutSeconds <= 0 {
			timeoutSeconds = 15
		}
		if timeoutSeconds > 300 {
			writeError(w, http.StatusBadRequest, "INVALID_TIMEOUT", "request_timeout_seconds must be between 1 and 300")
			return
		}
		repository := strings.TrimSpace(input.TestRepository)
		tag := strings.TrimSpace(input.TestTag)
		if input.TestImageReference != "" {
			defaultRepository, defaultTag := splitProbeImageReference(input.TestImageReference)
			if repository == "" {
				repository = defaultRepository
			}
			if tag == "" {
				tag = defaultTag
			}
		}
		if repository == "" {
			repository = "library/alpine"
		}
		if tag == "" {
			tag = "latest"
		}
		var result probe.Result
		timeout := time.Duration(timeoutSeconds) * time.Second
		switch mode {
		case probe.ModeDockerPull:
			image := strings.TrimSpace(input.TestImageReference)
			if image == "" {
				image = repository + ":" + tag
			}
			result = probe.RunDockerPull(r.Context(), timeout, image, input.DownloadTestBytes)
		case probe.ModeHTTP:
			result = probe.RunHTTP(r.Context(), input.BaseURL, timeout)
		default:
			result = probe.RunWithOptions(r.Context(), input.BaseURL, timeout, probe.Options{
				TestRepository:    repository,
				TestTag:           tag,
				DownloadTestBytes: input.DownloadTestBytes,
				SkipBlob:          mode == probe.ModeManifest,
			})
		}
		s.audit(r, user, "source.probe_test", input.BaseURL, map[string]any{"probe_mode": mode, "test_repository": repository, "test_tag": tag})
		writeData(w, result, nil)
		return
	}
	if restPath == "batch" && r.Method == http.MethodPost {
		user, ok := s.requirePermission(w, r, "source.write")
		if !ok {
			return
		}
		var input struct {
			IDs           []string `json:"ids"`
			Action        string   `json:"action"`
			CategoryID    string   `json:"category_id"`
			Enabled       *bool    `json:"enabled"`
			Maintenance   *bool    `json:"maintenance"`
			IsOfficial    *bool    `json:"is_official"`
			IsRecommended *bool    `json:"is_recommended"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input) != nil || len(input.IDs) == 0 || (input.Action != "enable" && input.Action != "disable" && input.Action != "edit") {
			writeError(w, 400, "INVALID_BATCH", "ids and action(enable|disable|edit) are required")
			return
		}
		updated := 0
		for _, id := range input.IDs {
			source, err := s.store.Source(id)
			if err != nil {
				continue
			}
			enabled := source.Enabled
			if input.Action == "enable" {
				enabled = true
			} else if input.Action == "disable" {
				enabled = false
			} else if input.Enabled != nil {
				enabled = *input.Enabled
			}
			maintenance := source.Maintenance
			if input.Maintenance != nil {
				maintenance = *input.Maintenance
			}
			isOfficial := source.IsOfficial
			if input.IsOfficial != nil {
				isOfficial = *input.IsOfficial
			}
			isRecommended := source.IsRecommended
			if input.IsRecommended != nil {
				isRecommended = *input.IsRecommended
			}
			categoryID := source.CategoryID
			if input.CategoryID != "" {
				categoryID = input.CategoryID
			}
			var testImageID *string
			if source.TestImageID != "" {
				value := source.TestImageID
				testImageID = &value
			}
			_, err = s.store.UpsertSource(domain.SourceInput{Name: source.Name, BaseURL: source.BaseURL, DisplayURL: source.DisplayURL, CategoryID: categoryID, Description: source.Description, Provider: source.Provider, Country: source.Country, Region: source.Region, Operator: source.Operator, Tags: source.Tags, Enabled: &enabled, IsOfficial: &isOfficial, IsCloudflare: &source.IsCloudflare, IsRecommended: &isRecommended, Priority: &source.Priority, SortOrder: &source.SortOrder, TestRepository: source.TestRepository, TestTag: source.TestTag, TestDigest: source.TestDigest, RequestTimeout: &source.RequestTimeout, DownloadTestBytes: &source.DownloadTestBytes, TestImageID: testImageID, Maintenance: &maintenance, ProbeConfigCustom: source.ProbeConfigCustom, ProbeMode: source.ProbeMode}, id)
			if err == nil {
				updated++
			}
		}
		s.audit(r, user, "source.batch_"+input.Action, "sources", map[string]any{"count": updated})
		writeData(w, map[string]any{"updated": updated}, nil)
		return
	}
	if restPath == "export" && r.Method == http.MethodGet {
		if _, ok := s.requirePermission(w, r, "source.read"); !ok {
			return
		}
		if strings.EqualFold(r.URL.Query().Get("format"), "csv") {
			var buf bytes.Buffer
			writer := csv.NewWriter(&buf)
			_ = writer.Write([]string{"name", "base_url", "category_id", "description", "provider", "country", "region", "operator", "tags", "is_official", "is_recommended", "priority", "sort_order", "maintenance", "probe_mode", "test_repository", "test_tag", "test_digest", "request_timeout_seconds", "download_test_bytes"})
			for _, source := range s.store.Sources() {
				_ = writer.Write([]string{source.Name, source.BaseURL, source.CategoryID, source.Description, source.Provider, source.Country, source.Region, source.Operator, strings.Join(source.Tags, ","), strconv.FormatBool(source.IsOfficial), strconv.FormatBool(source.IsRecommended), strconv.Itoa(source.Priority), strconv.Itoa(source.SortOrder), strconv.FormatBool(source.Maintenance), source.ProbeMode, source.TestRepository, source.TestTag, source.TestDigest, strconv.Itoa(source.RequestTimeout), strconv.FormatInt(source.DownloadTestBytes, 10)})
			}
			writer.Flush()
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", "attachment; filename=registry-sources.csv")
			_, _ = w.Write(buf.Bytes())
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=registry-sources.json")
		writeData(w, s.store.Sources(), map[string]any{"count": len(s.store.Sources())})
		return
	}
	if restPath == "import" && r.Method == http.MethodPost {
		user, ok := s.requirePermission(w, r, "source.write")
		if !ok {
			return
		}
		var inputs []domain.SourceInput
		if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "text/csv") {
			reader := csv.NewReader(io.LimitReader(r.Body, 2<<20))
			records, err := reader.ReadAll()
			if err != nil || len(records) < 1 {
				writeError(w, 400, "INVALID_IMPORT", "expected a CSV source file")
				return
			}
			for _, record := range records[1:] {
				if len(record) < 2 {
					writeError(w, 400, "INVALID_IMPORT", "CSV row is invalid")
					return
				}
				input := domain.SourceInput{Name: strings.TrimSpace(record[0]), BaseURL: strings.TrimSpace(record[1])}
				if len(record) > 2 {
					input.CategoryID = strings.TrimSpace(record[2])
				}
				if len(record) > 3 {
					input.Description = strings.TrimSpace(record[3])
				}
				if len(record) > 4 {
					input.Provider = strings.TrimSpace(record[4])
				}
				if len(record) > 5 {
					input.Country = strings.TrimSpace(record[5])
				}
				if len(record) > 6 {
					input.Region = strings.TrimSpace(record[6])
				}
				if len(record) > 7 {
					input.Operator = strings.TrimSpace(record[7])
				}
				if len(record) > 8 {
					input.Tags = strings.Split(record[8], ",")
				}
				if len(record) > 9 {
					value, _ := strconv.ParseBool(record[9])
					input.IsOfficial = &value
				}
				if len(record) > 10 {
					value, _ := strconv.ParseBool(record[10])
					input.IsRecommended = &value
				}
				if len(record) > 11 {
					value, _ := strconv.Atoi(record[11])
					input.Priority = &value
				}
				if len(record) > 12 {
					value, _ := strconv.Atoi(record[12])
					input.SortOrder = &value
				}
				if len(record) > 13 {
					value, _ := strconv.ParseBool(record[13])
					input.Maintenance = &value
				}
				if len(record) > 14 {
					input.ProbeMode = strings.TrimSpace(record[14])
				}
				if len(record) > 15 {
					input.TestRepository = strings.TrimSpace(record[15])
				}
				if len(record) > 16 {
					input.TestTag = strings.TrimSpace(record[16])
				}
				if len(record) > 17 {
					input.TestDigest = strings.TrimSpace(record[17])
				}
				if len(record) > 18 {
					value, _ := strconv.Atoi(record[18])
					input.RequestTimeout = &value
				}
				if len(record) > 19 {
					value, _ := strconv.ParseInt(record[19], 10, 64)
					input.DownloadTestBytes = &value
				}
				inputs = append(inputs, input)
			}
		} else if json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&inputs) != nil {
			writeError(w, 400, "INVALID_IMPORT", "expected a JSON array or CSV source file")
			return
		}
		created := []domain.Source{}
		for _, input := range inputs {
			if input.Name == "" || input.BaseURL == "" {
				writeError(w, 400, "INVALID_SOURCE", "name and base_url are required")
				return
			}
			if err := validateSourceInput(input); err != nil {
				writeError(w, 400, "INVALID_SOURCE_URL", err.Error())
				return
			}
			v, err := s.store.UpsertSource(input, "")
			if err != nil {
				if writeSourceSaveError(w, err) {
					return
				}
				writeError(w, 500, "IMPORT_FAILED", "could not import source")
				return
			}
			created = append(created, v)
		}
		s.audit(r, user, "source.import", "sources", map[string]any{"count": len(created)})
		writeJSON(w, 201, created, map[string]any{"count": len(created)})
		return
	}
	if strings.HasSuffix(restPath, "/probe") {
		user, ok := s.requirePermission(w, r, "probe.write")
		if !ok {
			return
		}
		id := strings.TrimSuffix(restPath, "/probe")
		source, err := s.store.Source(id)
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "SOURCE_NOT_FOUND", "source not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load source")
			return
		}
		task := s.agents.Enqueue(source.ID, "oci_probe", map[string]any{"target_url": source.BaseURL})
		s.audit(r, user, "source.probe", source.ID, map[string]any{"task_id": task.ID})
		writeJSON(w, http.StatusAccepted, task, nil)
		return
	}
	permission := "source.write"
	if r.Method == http.MethodGet {
		permission = "source.read"
	}
	user, ok := s.requirePermission(w, r, permission)
	if !ok {
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/sources/"), "/")
	if id == "" {
		writeError(w, 400, "INVALID_SOURCE", "source id is required")
		return
	}
	if r.Method == http.MethodDelete {
		if _, err := s.store.Source(id); errors.Is(err, domain.ErrNotFound) {
			writeError(w, 404, "SOURCE_NOT_FOUND", "source not found")
			return
		}
		// Historical probe rows can be large and may briefly contend with a
		// worker write. Queue the destructive cleanup so the HTTP request is
		// not held until the reverse proxy times out with 504.
		go func(sourceID string) {
			for attempt := 1; attempt <= 3; attempt++ {
				if err := s.store.DeleteSource(sourceID); err == nil {
					return
				} else {
					slog.Warn("source deletion retry", "source_id", sourceID, "attempt", attempt, "error", err)
				}
				time.Sleep(time.Duration(attempt) * time.Second)
			}
			slog.Error("source deletion failed", "source_id", sourceID)
		}(id)
		s.audit(r, user, "source.delete", id, nil)
		writeJSON(w, http.StatusAccepted, map[string]any{"queued": true, "source_id": id}, nil)
		return
	}
	if r.Method == http.MethodGet {
		v, err := s.store.Source(id)
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, 404, "SOURCE_NOT_FOUND", "source not found")
			return
		}
		writeData(w, v, nil)
		return
	}
	if r.Method == http.MethodPut {
		var in domain.SourceInput
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			writeError(w, 400, "INVALID_JSON", "invalid request body")
			return
		}
		if err := validateSourceInput(in); err != nil {
			writeError(w, 400, "INVALID_SOURCE_URL", err.Error())
			return
		}
		v, err := s.store.UpsertSource(in, id)
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, 404, "SOURCE_NOT_FOUND", "source not found")
			return
		}
		if writeSourceSaveError(w, err) {
			return
		}
		s.audit(r, user, "source.update", id, in)
		writeData(w, v, nil)
		return
	}
	writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
}

func validateSourceInput(in domain.SourceInput) error {
	allowPrivate := strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_PRIVATE_TARGETS")), "true")
	if err := probe.ValidateTarget(in.BaseURL, allowPrivate); err != nil {
		return err
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(in.BaseURL)), "http://") && !strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_INSECURE_HTTP")), "true") {
		return errors.New("http targets require ALLOW_INSECURE_HTTP=true")
	}
	return nil
}

func writeSourceSaveError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, database.ErrTestImageNotApplicable) {
		writeError(w, http.StatusBadRequest, "INVALID_TEST_IMAGE_SCOPE", "test_image_id is not applicable to category_id and probe_mode")
		return true
	}
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "INVALID_TEST_IMAGE", "test_image_id, category_id or source selector was not found")
		return true
	}
	if strings.Contains(err.Error(), "test_image_id") || strings.Contains(err.Error(), "category_id is required") {
		writeError(w, http.StatusBadRequest, "INVALID_TEST_IMAGE_SCOPE", err.Error())
		return true
	}
	return false
}

func supportedProbeMode(mode string) bool {
	for _, supported := range probe.SupportedModes() {
		if mode == supported {
			return true
		}
	}
	return false
}

func splitProbeImageReference(reference string) (string, string) {
	value := strings.TrimSpace(reference)
	separator := strings.LastIndex(value, ":")
	if separator > strings.LastIndex(value, "/") {
		return value[:separator], value[separator+1:]
	}
	return value, "latest"
}

func (s *Server) authorized(r *http.Request) bool {
	if s.adminToken != "" && strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) == s.adminToken {
		return true
	}
	user, ok := s.currentUser(r)
	return ok && user.Role == "admin"
}
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		w.Header().Set("X-Request-ID", hex.EncodeToString(b))
		next.ServeHTTP(w, r)
	})
}
func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
		s.metricRegistry.HTTPRequestFinished(time.Since(started))
	})
}
