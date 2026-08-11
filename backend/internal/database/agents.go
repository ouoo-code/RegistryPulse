package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

// AgentRegistry persists probe identities and task leases in PostgreSQL.
type AgentRegistry struct{ DB *sql.DB }

func NewAgentRegistry(db *sql.DB) *AgentRegistry { return &AgentRegistry{DB: db} }
func agentTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func scanNode(row interface{ Scan(...any) error }) (domain.ProbeNode, error) {
	var n domain.ProbeNode
	var caps []byte
	err := row.Scan(&n.ID, &n.Name, &n.Region, &n.Version, &caps, &n.Status, &n.LastSeenAt, &n.CreatedAt, &n.UpdatedAt)
	if err == nil {
		_ = json.Unmarshal(caps, &n.Capabilities)
	}
	return n, err
}

func (r *AgentRegistry) Register(in domain.AgentRegisterInput, token string) domain.ProbeNode {
	caps, _ := json.Marshal(in.Capabilities)
	n, err := scanNode(r.DB.QueryRow(`INSERT INTO probe_nodes(name,region,version,capabilities,status,token_hash,last_seen_at)
		VALUES($1,$2,$3,$4,'online',$5,now())
		ON CONFLICT(name) DO UPDATE SET region=EXCLUDED.region,version=EXCLUDED.version,capabilities=EXCLUDED.capabilities,status='online',token_hash=EXCLUDED.token_hash,last_seen_at=now(),updated_at=now()
		RETURNING id::text,name,region,version,capabilities,status,last_seen_at,created_at,updated_at`,
		in.Name, in.Region, in.Version, caps, agentTokenHash(token)))
	if err != nil {
		return domain.ProbeNode{}
	}
	return n
}

func (r *AgentRegistry) Authenticate(token string) (domain.ProbeNode, bool) {
	n, err := scanNode(r.DB.QueryRow(`SELECT id::text,name,region,version,capabilities,status,COALESCE(last_seen_at,created_at),created_at,updated_at FROM probe_nodes WHERE token_hash=$1`, agentTokenHash(token)))
	return n, err == nil
}

func (r *AgentRegistry) Heartbeat(id string, in domain.AgentHeartbeatInput) (domain.ProbeNode, error) {
	if in.Capabilities == nil {
		in.Capabilities = []string{}
	}
	caps, _ := json.Marshal(in.Capabilities)
	n, err := scanNode(r.DB.QueryRow(`UPDATE probe_nodes SET status=COALESCE(NULLIF($1,''),status),version=COALESCE(NULLIF($2,''),version),
		capabilities=CASE WHEN $3::jsonb='[]'::jsonb THEN capabilities ELSE $3::jsonb END,last_seen_at=now(),updated_at=now()
		WHERE id=$4 RETURNING id::text,name,region,version,capabilities,status,last_seen_at,created_at,updated_at`, in.Status, in.Version, caps, id))
	if err == sql.ErrNoRows {
		return domain.ProbeNode{}, domain.ErrAgentUnauthorized
	}
	if err == nil {
		_, _ = r.DB.Exec(`INSERT INTO probe_heartbeats(probe_node_id,status,version,capabilities) VALUES($1,$2,$3,$4)`, id, n.Status, n.Version, caps)
	}
	return n, err
}

func (r *AgentRegistry) Poll(id string, limit int, lease time.Duration) []domain.ProbeTask {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	tx, err := r.DB.Begin()
	if err != nil {
		return nil
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id::text,COALESCE(source_id::text,''),COALESCE(probe_node_id::text,''),task_type,payload,status,
		COALESCE(lease_until,'epoch'),COALESCE(started_at,'epoch'),COALESCE(finished_at,'epoch'),error,created_at,updated_at
		FROM probe_tasks WHERE status='pending' OR (status='leased' AND lease_until<now()) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1`, limit)
	if err != nil {
		return nil
	}
	until := time.Now().UTC().Add(lease)
	out := []domain.ProbeTask{}
	for rows.Next() {
		var t domain.ProbeTask
		var raw []byte
		if rows.Scan(&t.ID, &t.SourceID, &t.ProbeNodeID, &t.Type, &raw, &t.Status, &t.LeaseUntil, &t.StartedAt, &t.FinishedAt, &t.Error, &t.CreatedAt, &t.UpdatedAt) != nil {
			continue
		}
		_ = json.Unmarshal(raw, &t.Payload)
		out = append(out, t)
	}
	if err = rows.Close(); err != nil {
		return nil
	}
	for i := range out {
		if _, err = tx.Exec(`UPDATE probe_tasks SET status='leased',probe_node_id=$1,lease_until=$2,updated_at=now() WHERE id=$3`, id, until, out[i].ID); err != nil {
			return nil
		}
		out[i].Status, out[i].ProbeNodeID, out[i].LeaseUntil = "leased", id, until
	}
	if tx.Commit() != nil {
		return nil
	}
	return out
}

func scanTask(row interface{ Scan(...any) error }) (domain.ProbeTask, error) {
	var t domain.ProbeTask
	var raw []byte
	err := row.Scan(&t.ID, &t.SourceID, &t.ProbeNodeID, &t.Type, &raw, &t.Status, &t.LeaseUntil, &t.StartedAt, &t.FinishedAt, &t.Error, &t.CreatedAt, &t.UpdatedAt)
	if err == nil {
		_ = json.Unmarshal(raw, &t.Payload)
	}
	return t, err
}

func (r *AgentRegistry) StartTask(agentID, taskID string) (domain.ProbeTask, error) {
	t, err := scanTask(r.DB.QueryRow(`UPDATE probe_tasks SET status='running',started_at=now(),updated_at=now()
		WHERE id=$1 AND probe_node_id=$2 AND status IN ('leased','running')
		RETURNING id::text,COALESCE(source_id::text,''),COALESCE(probe_node_id::text,''),task_type,payload,status,COALESCE(lease_until,'epoch'),COALESCE(started_at,'epoch'),COALESCE(finished_at,'epoch'),error,created_at,updated_at`, taskID, agentID))
	if err == sql.ErrNoRows {
		return domain.ProbeTask{}, domain.ErrTaskNotAssigned
	}
	return t, err
}

func (r *AgentRegistry) CompleteTask(agentID, taskID string, in domain.AgentResultInput) (domain.ProbeTask, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return domain.ProbeTask{}, err
	}
	defer tx.Rollback()
	var source, probeMode string
	if err = tx.QueryRow(`SELECT COALESCE(t.source_id::text,''),COALESCE(CASE WHEN rs.probe_config_custom THEN NULLIF(rs.probe_mode,'') ELSE NULLIF(rc.default_probe_mode,'') END,'unknown') FROM probe_tasks t LEFT JOIN registry_sources rs ON rs.id=t.source_id LEFT JOIN registry_categories rc ON rc.id=rs.category_id WHERE t.id=$1 AND t.probe_node_id=$2 AND t.status IN ('running','leased') FOR UPDATE`, taskID, agentID).Scan(&source, &probeMode); err == sql.ErrNoRows {
		return domain.ProbeTask{}, domain.ErrTaskNotAssigned
	} else if err != nil {
		return domain.ProbeTask{}, err
	}
	var resultID string
	if err = tx.QueryRow(`INSERT INTO probe_results(source_id,probe_node_id,task_id,status,dns_duration_ms,tcp_duration_ms,tls_duration_ms,registry_duration_ms,manifest_duration_ms,blob_duration_ms,blob_bytes,error,probe_mode,checked_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,now()) RETURNING id::text`, source, agentID, taskID, in.Status, in.DNSMS, in.TCPMS, in.TLSMS, in.RegistryMS, in.ManifestMS, in.BlobMS, in.BlobBytes, in.Error, probeMode).Scan(&resultID); err != nil {
		return domain.ProbeTask{}, err
	}
	// Derive the source status from the newest result reported by each probe.
	// A mixed fleet is degraded; only an all-offline fleet is offline.
	var online, total int
	if err = tx.QueryRow(`SELECT COUNT(*) FILTER (WHERE status='online'), COUNT(*) FROM (
		SELECT DISTINCT ON (probe_node_id) probe_node_id,status FROM probe_results
		WHERE source_id=$1 AND probe_node_id IS NOT NULL ORDER BY probe_node_id,checked_at DESC,id DESC
	) latest`, source).Scan(&online, &total); err != nil {
		return domain.ProbeTask{}, err
	}
	aggregate := "unknown"
	if total > 0 {
		switch {
		case online == 0:
			aggregate = "offline"
		case online < total:
			aggregate = "degraded"
		default:
			aggregate = "online"
		}
	}
	if _, err = tx.Exec(`UPDATE registry_sources SET status=$1,response_ms=$2,last_checked=now(),error=$3,updated_at=now() WHERE id=$4`, aggregate, in.RegistryMS+in.ManifestMS+in.BlobMS, in.Error, source); err != nil {
		return domain.ProbeTask{}, err
	}
	if _, err = tx.Exec(`UPDATE probe_tasks SET status='completed',finished_at=now(),updated_at=now(),error='' WHERE id=$1`, taskID); err != nil {
		return domain.ProbeTask{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.ProbeTask{}, err
	}
	return domain.ProbeTask{ID: taskID, SourceID: source, ProbeNodeID: agentID, Status: "completed", Result: &domain.ProbeResult{ID: resultID, SourceID: source, ProbeNodeID: agentID, TaskID: taskID, Status: in.Status, Error: in.Error}}, nil
}

func (r *AgentRegistry) FailTask(agentID, taskID, message string) (domain.ProbeTask, error) {
	var id string
	err := r.DB.QueryRow(`UPDATE probe_tasks SET status='failed',error=$1,finished_at=now(),updated_at=now()
		WHERE id=$2 AND probe_node_id=$3 AND status IN ('running','leased') RETURNING id::text`, message, taskID, agentID).Scan(&id)
	if err == sql.ErrNoRows {
		return domain.ProbeTask{}, domain.ErrTaskNotAssigned
	}
	if err != nil {
		return domain.ProbeTask{}, err
	}
	return domain.ProbeTask{ID: id, ProbeNodeID: agentID, Status: "failed", Error: message}, nil
}

func (r *AgentRegistry) Enqueue(sourceID, taskType string, payload map[string]any) domain.ProbeTask {
	raw, _ := json.Marshal(payload)
	t, err := scanTask(r.DB.QueryRow(`INSERT INTO probe_tasks(source_id,task_type,payload)
		VALUES($1,$2,$3) RETURNING id::text,COALESCE(source_id::text,''),COALESCE(probe_node_id::text,''),task_type,payload,status,COALESCE(lease_until,'epoch'),COALESCE(started_at,'epoch'),COALESCE(finished_at,'epoch'),error,created_at,updated_at`, sourceID, taskType, raw))
	if err != nil {
		return domain.ProbeTask{}
	}
	return t
}

func (r *AgentRegistry) Tasks(limit int) []domain.ProbeTask {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.DB.Query(`SELECT id::text,COALESCE(source_id::text,''),COALESCE(probe_node_id::text,''),task_type,payload,status,COALESCE(lease_until,'epoch'),COALESCE(started_at,'epoch'),COALESCE(finished_at,'epoch'),error,created_at,updated_at FROM probe_tasks ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.ProbeTask{}
	for rows.Next() {
		if t, err := scanTask(rows); err == nil {
			out = append(out, t)
		}
	}
	return out
}

func (r *AgentRegistry) Nodes() []domain.ProbeNode {
	rows, err := r.DB.Query(`SELECT id::text,name,region,version,capabilities,status,COALESCE(last_seen_at,created_at),created_at,updated_at FROM probe_nodes ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.ProbeNode{}
	for rows.Next() {
		if n, err := scanNode(rows); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func (r *AgentRegistry) Remove(id string) error {
	res, err := r.DB.Exec(`DELETE FROM probe_nodes WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AgentRegistry) CancelTask(id string) (domain.ProbeTask, error) {
	t, err := scanTask(r.DB.QueryRow(`UPDATE probe_tasks SET status='cancelled',finished_at=now(),updated_at=now() WHERE id=$1 AND status NOT IN ('completed','failed','cancelled') RETURNING id::text,COALESCE(source_id::text,''),COALESCE(probe_node_id::text,''),task_type,payload,status,COALESCE(lease_until,'epoch'),COALESCE(started_at,'epoch'),COALESCE(finished_at,'epoch'),error,created_at,updated_at`, id))
	if err == sql.ErrNoRows {
		return domain.ProbeTask{}, domain.ErrTaskState
	}
	return t, err
}

func (r *AgentRegistry) RetryTask(id string) (domain.ProbeTask, error) {
	t, err := scanTask(r.DB.QueryRow(`UPDATE probe_tasks SET status='pending',probe_node_id=NULL,lease_until=NULL,started_at=NULL,finished_at=NULL,error='',updated_at=now() WHERE id=$1 AND status IN ('failed','cancelled') RETURNING id::text,COALESCE(source_id::text,''),COALESCE(probe_node_id::text,''),task_type,payload,status,COALESCE(lease_until,'epoch'),COALESCE(started_at,'epoch'),COALESCE(finished_at,'epoch'),error,created_at,updated_at`, id))
	if err == sql.ErrNoRows {
		return domain.ProbeTask{}, domain.ErrTaskState
	}
	return t, err
}

// ClearTasks removes only terminal tasks. Leased or running tasks are kept so
// an active probe cannot lose its lease while a worker is reporting a result.
func (r *AgentRegistry) ClearTasks() (int64, error) {
	result, err := r.DB.Exec(`DELETE FROM probe_tasks WHERE status IN ('completed','failed','cancelled')`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

var _ domain.AgentRegistry = (*AgentRegistry)(nil)
