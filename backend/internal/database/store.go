package database

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

type Store struct{ DB *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{DB: db} }
func (s *Store) DBHandle() *sql.DB {
	if s == nil {
		return nil
	}
	return s.DB
}

var _ domain.Store = (*Store)(nil)

func (s *Store) Categories() []domain.Category {
	rows, err := s.DB.Query(`SELECT id,slug,name,description,icon,official_url,default_test_repository,default_test_tag,COALESCE(default_test_image_id::text,''),default_probe_mode,default_timeout_seconds,default_manifest_path,auth_type,enabled,sort_order,created_at FROM registry_categories ORDER BY sort_order,name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.Category{}
	for rows.Next() {
		var v domain.Category
		if rows.Scan(&v.ID, &v.Slug, &v.Name, &v.Description, &v.Icon, &v.OfficialURL, &v.DefaultTestRepository, &v.DefaultTestTag, &v.DefaultTestImageID, &v.DefaultProbeMode, &v.DefaultTimeoutSeconds, &v.DefaultManifestPath, &v.AuthType, &v.Enabled, &v.SortOrder, &v.CreatedAt) == nil {
			out = append(out, v)
		}
	}
	return out
}
func (s *Store) Category(slug string) (domain.Category, error) {
	var v domain.Category
	err := s.DB.QueryRow(`SELECT id,slug,name,description,icon,official_url,default_test_repository,default_test_tag,COALESCE(default_test_image_id::text,''),default_probe_mode,default_timeout_seconds,default_manifest_path,auth_type,enabled,sort_order,created_at FROM registry_categories WHERE lower(slug)=lower($1) OR id=$1`, slug).Scan(&v.ID, &v.Slug, &v.Name, &v.Description, &v.Icon, &v.OfficialURL, &v.DefaultTestRepository, &v.DefaultTestTag, &v.DefaultTestImageID, &v.DefaultProbeMode, &v.DefaultTimeoutSeconds, &v.DefaultManifestPath, &v.AuthType, &v.Enabled, &v.SortOrder, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return v, domain.ErrNotFound
	}
	return v, err
}
func (s *Store) Sources() []domain.Source {
	// Always provide a deterministic tie-breaker. Many sources share the same
	// display name, and PostgreSQL does not guarantee the order of rows that
	// compare equal on the ORDER BY columns after an UPDATE.
	rows, err := s.DB.Query(sourceSelect + ` ORDER BY name, sort_order, id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.Source{}
	for rows.Next() {
		if v, err := scanSource(rows); err == nil {
			out = append(out, v)
		}
	}
	return out
}
func (s *Store) Source(id string) (domain.Source, error) {
	v, err := scanSource(s.DB.QueryRow(sourceSelect+` WHERE rs.id=$1`, id))
	if err == sql.ErrNoRows {
		return v, domain.ErrNotFound
	}
	return v, err
}
func (s *Store) History(id string, limit int) []domain.ProbeResult {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.DB.Query(`SELECT id::text,source_id::text,status,dns_duration_ms,tcp_duration_ms,tls_duration_ms,registry_duration_ms,manifest_duration_ms,blob_duration_ms,blob_bytes,error,error_stage,dns_success,resolved_ips,tcp_success,remote_ip,remote_port,tls_success,tls_version,tls_cipher,certificate_subject,certificate_issuer,certificate_days_remaining,registry_api_success,registry_api_status,manifest_success,manifest_status,manifest_content_type,manifest_digest,blob_success,blob_status,blob_ttfb_ms,blob_speed_bps,checked_at,certificate_not_before,certificate_not_after,registry_api_version,manifest_size,blob_range_supported,dns_error,tcp_error,tls_error,registry_api_error,manifest_error,blob_error FROM probe_results WHERE source_id=$1 ORDER BY checked_at DESC LIMIT $2`, id, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.ProbeResult{}
	for rows.Next() {
		var v domain.ProbeResult
		var resolved []byte
		var notBefore, notAfter sql.NullTime
		if rows.Scan(&v.ID, &v.SourceID, &v.Status, &v.DNSMS, &v.TCPMS, &v.TLSMS, &v.RegistryMS, &v.ManifestMS, &v.BlobMS, &v.BlobBytes, &v.Error, &v.ErrorStage, &v.DNSSuccess, &resolved, &v.TCPSuccess, &v.RemoteIP, &v.RemotePort, &v.TLSSuccess, &v.TLSVersion, &v.TLSCipher, &v.CertificateSubject, &v.CertificateIssuer, &v.CertificateDaysRemaining, &v.RegistrySuccess, &v.RegistryStatus, &v.ManifestSuccess, &v.ManifestStatus, &v.ManifestContentType, &v.ManifestDigest, &v.BlobSuccess, &v.BlobStatus, &v.BlobTTFBMS, &v.BlobSpeedBPS, &v.CheckedAt, &notBefore, &notAfter, &v.RegistryAPIVersion, &v.ManifestSize, &v.BlobRangeSupported, &v.DNSError, &v.TCPError, &v.TLSError, &v.RegistryAPIError, &v.ManifestError, &v.BlobError) == nil {
			if notBefore.Valid {
				v.CertificateNotBefore = notBefore.Time
			}
			if notAfter.Valid {
				v.CertificateNotAfter = notAfter.Time
			}
			v.ResponseMS = v.RegistryMS + v.ManifestMS + v.BlobMS
			_ = json.Unmarshal(resolved, &v.ResolvedIPs)
			out = append(out, v)
		}
	}
	return out
}
func (s *Store) UpsertSource(in domain.SourceInput, id string) (domain.Source, error) {
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	isOfficial, isCloudflare, isRecommended := false, false, false
	if in.IsOfficial != nil {
		isOfficial = *in.IsOfficial
	}
	if in.IsCloudflare != nil {
		isCloudflare = *in.IsCloudflare
	}
	if in.IsRecommended != nil {
		isRecommended = *in.IsRecommended
	}
	priority, sortOrder := 0, 0
	if in.Priority != nil {
		priority = *in.Priority
	}
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	requestTimeout, downloadBytes := 10, int64(2<<20)
	probeConfigCustom := in.ProbeConfigCustom
	if in.RequestTimeout != nil {
		requestTimeout = *in.RequestTimeout
	}
	if in.DownloadTestBytes != nil {
		downloadBytes = *in.DownloadTestBytes
	}
	if requestTimeout < 1 || requestTimeout > 300 {
		requestTimeout = 10
	}
	if downloadBytes < 0 || downloadBytes > 64<<20 {
		downloadBytes = 2 << 20
	}
	testRepository, testTag := in.TestRepository, in.TestTag
	if testRepository == "" {
		testRepository = "library/alpine"
	}
	if testTag == "" {
		testTag = "latest"
	}
	var testImageID any
	if in.TestImageID != nil && strings.TrimSpace(*in.TestImageID) != "" {
		testImageID = strings.TrimSpace(*in.TestImageID)
	}
	displayURL := in.DisplayURL
	if displayURL == "" {
		displayURL = in.BaseURL
	}
	tags, _ := json.Marshal(in.Tags)
	now := time.Now().UTC()
	if id == "" {
		// Imports do not carry database UUIDs. Reconcile by the stable source
		// identity so an imported source catalog cannot create duplicate rows.
		lookupErr := s.DB.QueryRow(`SELECT id::text FROM registry_sources WHERE category_id=$1 AND name=$2 AND base_url=$3 ORDER BY id LIMIT 1`, in.CategoryID, in.Name, in.BaseURL).Scan(&id)
		if lookupErr != nil && lookupErr != sql.ErrNoRows {
			return domain.Source{}, lookupErr
		}
		if lookupErr == sql.ErrNoRows {
			id = ""
		}
	}
	if id == "" {
		maintenance := false
		if in.Maintenance != nil {
			maintenance = *in.Maintenance
		}
		mode := in.ProbeMode
		if mode == "" {
			mode = "registry"
		}
		err := s.DB.QueryRow(`INSERT INTO registry_sources(category_id,name,base_url,display_url,registry_host,description,provider,country,region,operator,tags,is_official,is_cloudflare,is_recommended,is_enabled,priority,sort_order,maintenance,probe_config_custom,probe_mode,test_repository,test_tag,test_digest,request_timeout_seconds,download_test_bytes,test_image_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26) RETURNING id::text`, in.CategoryID, in.Name, in.BaseURL, displayURL, host(in.BaseURL), in.Description, in.Provider, in.Country, in.Region, in.Operator, tags, isOfficial, isCloudflare, isRecommended, enabled, priority, sortOrder, maintenance, probeConfigCustom, mode, testRepository, testTag, in.TestDigest, requestTimeout, downloadBytes, testImageID).Scan(&id)
		if err != nil {
			return domain.Source{}, err
		}
	} else {
		maintenance := false
		if in.Maintenance != nil {
			maintenance = *in.Maintenance
		}
		mode := in.ProbeMode
		if mode == "" {
			mode = "registry"
		}
		res, err := s.DB.Exec(`UPDATE registry_sources SET category_id=$1,name=$2,base_url=$3,display_url=$4,registry_host=$5,description=$6,provider=$7,country=$8,region=$9,operator=$10,tags=$11,is_official=$12,is_cloudflare=$13,is_recommended=$14,is_enabled=$15,priority=$16,sort_order=$17,maintenance=$18,status=CASE WHEN $18 THEN 'maintenance' WHEN status='maintenance' THEN 'unknown' ELSE status END,probe_config_custom=$19,probe_mode=$20,test_repository=$21,test_tag=$22,test_digest=$23,request_timeout_seconds=$24,download_test_bytes=$25,test_image_id=$26,updated_at=now() WHERE id=$27`, in.CategoryID, in.Name, in.BaseURL, displayURL, host(in.BaseURL), in.Description, in.Provider, in.Country, in.Region, in.Operator, tags, isOfficial, isCloudflare, isRecommended, enabled, priority, sortOrder, maintenance, probeConfigCustom, mode, testRepository, testTag, in.TestDigest, requestTimeout, downloadBytes, testImageID, id)
		if err != nil {
			return domain.Source{}, err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return domain.Source{}, domain.ErrNotFound
		}
	}
	v, err := s.Source(id)
	if err == nil {
		v.UpdatedAt = now
	}
	return v, err
}
func (s *Store) DeleteSource(id string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Historical probe rows are removed in the same transaction so source
	// deletion keeps its existing data-retention semantics.
	if _, err = tx.Exec(`DELETE FROM probe_results WHERE source_id=$1`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM registry_sources WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit()
}

const sourceSelect = `SELECT rs.id::text,rs.category_id,rs.name,rs.base_url,COALESCE(rs.display_url,rs.base_url),rs.registry_host,rs.description,rs.provider,rs.country,rs.region,rs.operator,rs.tags,rs.is_official,rs.is_cloudflare,rs.is_recommended,rs.is_enabled,rs.priority,rs.sort_order,rs.maintenance,rs.probe_config_custom,CASE WHEN rs.probe_config_custom THEN rs.probe_mode ELSE COALESCE(NULLIF(c.default_probe_mode,''),'registry') END,CASE WHEN rs.probe_config_custom THEN rs.test_repository ELSE COALESCE(NULLIF(c.default_test_repository,''),'library/alpine') END,CASE WHEN rs.probe_config_custom THEN rs.test_tag ELSE COALESCE(NULLIF(c.default_test_tag,''),'latest') END,rs.test_digest,CASE WHEN rs.probe_config_custom THEN rs.request_timeout_seconds ELSE COALESCE(NULLIF(c.default_timeout_seconds,0),15) END,rs.download_test_bytes,rs.status,rs.response_ms,rs.last_checked,rs.error,rs.created_at,rs.updated_at,COALESCE(test_image.image_id::text,''),COALESCE(test_image.reference,''),COALESCE(test_image.max_bytes,0) FROM registry_sources rs JOIN registry_categories c ON c.id=rs.category_id LEFT JOIN LATERAL (SELECT id AS image_id,reference,max_bytes,0 AS selection_priority FROM test_images WHERE rs.probe_config_custom AND id=rs.test_image_id AND enabled UNION ALL SELECT id AS image_id,reference,max_bytes,1 AS selection_priority FROM test_images WHERE NOT rs.probe_config_custom AND id=c.default_test_image_id AND enabled UNION ALL SELECT id AS image_id,reference,max_bytes,2 AS selection_priority FROM test_images WHERE is_default AND enabled ORDER BY selection_priority,image_id LIMIT 1) AS test_image ON true`

type scanner interface{ Scan(...any) error }

func scanSource(row scanner) (domain.Source, error) {
	var v domain.Source
	var raw []byte
	var lastChecked sql.NullTime
	var testImageID sql.NullString
	err := row.Scan(&v.ID, &v.CategoryID, &v.Name, &v.BaseURL, &v.DisplayURL, &v.RegistryHost, &v.Description, &v.Provider, &v.Country, &v.Region, &v.Operator, &raw, &v.IsOfficial, &v.IsCloudflare, &v.IsRecommended, &v.Enabled, &v.Priority, &v.SortOrder, &v.Maintenance, &v.ProbeConfigCustom, &v.ProbeMode, &v.TestRepository, &v.TestTag, &v.TestDigest, &v.RequestTimeout, &v.DownloadTestBytes, &v.Status, &v.ResponseMS, &lastChecked, &v.Error, &v.CreatedAt, &v.UpdatedAt, &testImageID, &v.TestImageReference, &v.TestImageMaxBytes)
	if err != nil {
		return v, err
	}
	if lastChecked.Valid {
		v.LastChecked = lastChecked.Time
	}
	if testImageID.Valid {
		v.TestImageID = testImageID.String
		if v.TestImageReference != "" {
			if separator := strings.LastIndex(v.TestImageReference, ":"); separator > 0 {
				v.TestRepository = v.TestImageReference[:separator]
				v.TestTag = v.TestImageReference[separator+1:]
			}
		}
		if v.TestImageMaxBytes > 0 {
			v.DownloadTestBytes = v.TestImageMaxBytes
		}
	}
	_ = json.Unmarshal(raw, &v.Tags)
	return v, nil
}
func host(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	if i := strings.IndexByte(raw, '/'); i >= 0 {
		raw = raw[:i]
	}
	return raw
}
