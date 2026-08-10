package proxy

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/redis/go-redis/v9"
)

// SnapshotKey is deliberately versioned. A future route schema can be
// introduced without making an older proxy interpret fields it does not know.
const SnapshotKey = "registrypulse:proxy:routes:v1"

type RouteSnapshot struct {
	Version     int64       `json:"version"`
	GeneratedAt time.Time   `json:"generated_at"`
	Candidates  []Candidate `json:"candidates"`
}

// Candidate contains routing metadata only. Credentials, cookies, and bearer
// tokens never enter the Redis snapshot.
type Candidate struct {
	SourceID      string    `json:"source_id"`
	CategoryID    string    `json:"category_id"`
	Name          string    `json:"name"`
	BaseURL       string    `json:"base_url"`
	RegistryHost  string    `json:"registry_host,omitempty"`
	Status        string    `json:"status"`
	Enabled       bool      `json:"enabled"`
	Maintenance   bool      `json:"maintenance"`
	Priority      int       `json:"priority"`
	SortOrder     int       `json:"sort_order"`
	ResponseMS    int64     `json:"response_ms"`
	LastChecked   time.Time `json:"last_checked,omitempty"`
	IsRecommended bool      `json:"is_recommended"`
	IsOfficial    bool      `json:"is_official"`
}

func BuildSnapshot(sources []domain.Source) RouteSnapshot {
	now := time.Now().UTC()
	out := RouteSnapshot{Version: now.UnixNano(), GeneratedAt: now}
	for _, source := range sources {
		if strings.TrimSpace(source.BaseURL) == "" {
			continue
		}
		out.Candidates = append(out.Candidates, Candidate{
			SourceID:      source.ID,
			CategoryID:    source.CategoryID,
			Name:          source.Name,
			BaseURL:       source.BaseURL,
			RegistryHost:  source.RegistryHost,
			Status:        source.Status,
			Enabled:       source.Enabled,
			Maintenance:   source.Maintenance,
			Priority:      source.Priority,
			SortOrder:     source.SortOrder,
			ResponseMS:    source.ResponseMS,
			LastChecked:   source.LastChecked,
			IsRecommended: source.IsRecommended,
			IsOfficial:    source.IsOfficial,
		})
	}
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		return candidateLess(out.Candidates[i], out.Candidates[j])
	})
	return out
}

func PublishSnapshot(ctx context.Context, client redis.UniversalClient, snapshot RouteSnapshot) error {
	if client == nil {
		return nil
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	// No TTL is used. Redis AOF preserves the last known route state during a
	// proxy restart; the API refreshes it whenever source state changes.
	return client.Set(ctx, SnapshotKey, payload, 0).Err()
}

func LoadSnapshot(ctx context.Context, client redis.UniversalClient) (RouteSnapshot, error) {
	var snapshot RouteSnapshot
	if client == nil {
		return snapshot, nil
	}
	raw, err := client.Get(ctx, SnapshotKey).Bytes()
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func candidateLess(a, b Candidate) bool {
	if statusRank(a.Status) != statusRank(b.Status) {
		return statusRank(a.Status) < statusRank(b.Status)
	}
	if a.IsRecommended != b.IsRecommended {
		return a.IsRecommended
	}
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	if a.ResponseMS > 0 && b.ResponseMS > 0 && a.ResponseMS != b.ResponseMS {
		return a.ResponseMS < b.ResponseMS
	}
	if a.ResponseMS == 0 && b.ResponseMS > 0 {
		return false
	}
	if a.ResponseMS > 0 && b.ResponseMS == 0 {
		return true
	}
	if a.SortOrder != b.SortOrder {
		return a.SortOrder < b.SortOrder
	}
	if a.IsOfficial != b.IsOfficial {
		return a.IsOfficial
	}
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

func statusRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online":
		return 0
	case "degraded":
		return 1
	default:
		return 2
	}
}
