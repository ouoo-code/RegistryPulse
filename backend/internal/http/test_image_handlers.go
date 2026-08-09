package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/ouoo-code/RegistryPulse/internal/database"
	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

var testImageReference = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*:[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type testImageInput struct {
	ID           string   `json:"id"`
	Reference    string   `json:"reference"`
	Enabled      *bool    `json:"enabled"`
	MaxBytes     int64    `json:"max_bytes"`
	IsDefault    bool     `json:"is_default"`
	AuthStrategy string   `json:"auth_strategy"`
	CategoryIDs  []string `json:"category_ids"`
	ProbeModes   []string `json:"probe_modes"`
}

func (s *Server) adminTestImages(w http.ResponseWriter, r *http.Request) {
	permission := "settings.read"
	if r.Method != http.MethodGet {
		permission = "settings.write"
	}
	user, ok := s.requirePermission(w, r, permission)
	if !ok {
		return
	}
	db := s.db(w)
	if db == nil {
		return
	}

	pathID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/test-images/"), "/")
	if r.Method == http.MethodGet {
		categoryID := strings.TrimSpace(r.URL.Query().Get("category_id"))
		probeMode := strings.TrimSpace(r.URL.Query().Get("probe_mode"))
		items, err := database.QueryTestImages(r.Context(), db, categoryID, probeMode)
		if err != nil {
			writeError(w, 500, "TEST_IMAGE_QUERY_FAILED", "could not query test images")
			return
		}
		if pathID == "options" {
			filtered := items[:0]
			for _, item := range items {
				if item.Enabled {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		writeData(w, items, map[string]any{"count": len(items), "category_id": categoryID, "probe_mode": probeMode})
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(pathID, "/default") {
		id := strings.TrimSuffix(pathID, "/default")
		if id == "" {
			writeError(w, 400, "INVALID_TEST_IMAGE", "image id is required")
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE test_images SET is_default=false,updated_at=now()`)
		}
		if err == nil {
			var enabled bool
			err = tx.QueryRowContext(r.Context(), `UPDATE test_images SET is_default=true,enabled=true,updated_at=now() WHERE id=$1 RETURNING enabled`, id).Scan(&enabled)
		}
		if err != nil {
			if tx != nil {
				_ = tx.Rollback()
			}
			writeError(w, 404, "TEST_IMAGE_NOT_FOUND", "test image not found")
			return
		}
		if err = tx.Commit(); err != nil {
			writeError(w, 500, "TEST_IMAGE_DEFAULT_FAILED", "could not set default test image")
			return
		}
		s.audit(r, user, "test_image.default", id, nil)
		writeData(w, map[string]bool{"is_default": true}, nil)
		return
	}
	if r.Method == http.MethodDelete {
		id := pathID
		if id == "" {
			writeError(w, 400, "INVALID_TEST_IMAGE", "image id is required")
			return
		}
		var wasDefault bool
		if err := db.QueryRowContext(r.Context(), `SELECT is_default FROM test_images WHERE id=$1`, id).Scan(&wasDefault); err != nil {
			writeError(w, 404, "TEST_IMAGE_NOT_FOUND", "test image not found")
			return
		}
		res, err := db.ExecContext(r.Context(), `DELETE FROM test_images WHERE id=$1`, id)
		if err != nil {
			writeError(w, 500, "TEST_IMAGE_DELETE_FAILED", "could not delete test image")
			return
		}
		count, _ := res.RowsAffected()
		if count == 0 {
			writeError(w, 404, "TEST_IMAGE_NOT_FOUND", "test image not found")
			return
		}
		if wasDefault {
			_, _ = db.ExecContext(r.Context(), `UPDATE test_images SET is_default=true,updated_at=now() WHERE id=(SELECT id FROM test_images WHERE enabled ORDER BY reference,id LIMIT 1)`)
		}
		s.audit(r, user, "test_image.delete", id, nil)
		writeData(w, map[string]bool{"deleted": true}, nil)
		return
	}

	var input testImageInput
	if json.NewDecoder(r.Body).Decode(&input) != nil || !testImageReference.MatchString(strings.TrimSpace(input.Reference)) || strings.Contains(input.Reference, "..") {
		writeError(w, 400, "INVALID_TEST_IMAGE", "reference must be a safe tagged image reference")
		return
	}
	input.Reference = strings.TrimSpace(input.Reference)
	enabled := true
	if r.Method == http.MethodPut && input.Enabled == nil {
		if err := db.QueryRowContext(r.Context(), `SELECT enabled FROM test_images WHERE id=$1`, pathID).Scan(&enabled); err != nil {
			writeError(w, 404, "TEST_IMAGE_NOT_FOUND", "test image not found")
			return
		}
	}
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	input.Enabled = &enabled
	if input.MaxBytes <= 0 {
		input.MaxBytes = 1 << 20
	}
	if input.MaxBytes > 4<<30 {
		writeError(w, 400, "INVALID_TEST_IMAGE", "max_bytes is too large")
		return
	}
	if input.AuthStrategy == "" {
		input.AuthStrategy = "anonymous"
	}
	if input.AuthStrategy != "anonymous" && input.AuthStrategy != "optional" && input.AuthStrategy != "required" {
		writeError(w, 400, "INVALID_TEST_IMAGE", "auth_strategy must be anonymous, optional or required")
		return
	}
	categoryIDs, err := validateTestImageCategories(r.Context(), db, input.CategoryIDs)
	if err != nil {
		writeError(w, 400, "INVALID_TEST_IMAGE_SCOPE", err.Error())
		return
	}
	probeModes, err := validateTestImageModes(input.ProbeModes)
	if err != nil {
		writeError(w, 400, "INVALID_TEST_IMAGE_SCOPE", err.Error())
		return
	}
	input.CategoryIDs, input.ProbeModes = categoryIDs, probeModes

	if r.Method == http.MethodPost {
		var id string
		err = withTestImageRelations(r.Context(), db, "", func(tx *sql.Tx) (string, error) {
			if err := tx.QueryRowContext(r.Context(), `INSERT INTO test_images(reference,enabled,max_bytes,is_default,auth_strategy) VALUES($1,$2,$3,false,$4) RETURNING id::text`, input.Reference, *input.Enabled, input.MaxBytes, input.AuthStrategy).Scan(&id); err != nil {
				return "", err
			}
			return id, nil
		}, input.CategoryIDs, input.ProbeModes)
		if err != nil {
			writeError(w, 409, "TEST_IMAGE_EXISTS", "could not create test image")
			return
		}
		input.ID = id
		s.audit(r, user, "test_image.create", input.ID, map[string]any{"reference": input.Reference, "enabled": input.Enabled, "max_bytes": input.MaxBytes, "auth_strategy": input.AuthStrategy, "category_count": len(input.CategoryIDs), "probe_mode_count": len(input.ProbeModes)})
		writeJSON(w, 201, input, nil)
		return
	}
	if r.Method == http.MethodPut {
		if input.ID == "" {
			input.ID = pathID
		}
		if input.ID == "" {
			writeError(w, 400, "INVALID_TEST_IMAGE", "image id is required")
			return
		}
		err = withTestImageRelations(r.Context(), db, input.ID, func(tx *sql.Tx) (string, error) {
			result, updateErr := tx.ExecContext(r.Context(), `UPDATE test_images SET reference=$1,enabled=$2,max_bytes=$3,auth_strategy=$4,updated_at=now() WHERE id=$5`, input.Reference, *input.Enabled, input.MaxBytes, input.AuthStrategy, input.ID)
			if updateErr != nil {
				return "", updateErr
			}
			count, _ := result.RowsAffected()
			if count == 0 {
				return "", domain.ErrNotFound
			}
			return input.ID, nil
		}, input.CategoryIDs, input.ProbeModes)
		if err != nil {
			if err == domain.ErrNotFound {
				writeError(w, 404, "TEST_IMAGE_NOT_FOUND", "test image not found")
			} else {
				writeError(w, 500, "TEST_IMAGE_UPDATE_FAILED", "could not update test image")
			}
			return
		}
		s.audit(r, user, "test_image.update", input.ID, map[string]any{"reference": input.Reference, "enabled": input.Enabled, "max_bytes": input.MaxBytes, "auth_strategy": input.AuthStrategy, "category_count": len(input.CategoryIDs), "probe_mode_count": len(input.ProbeModes)})
		writeData(w, input, nil)
		return
	}
	writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
}

func validateTestImageCategories(ctx context.Context, db *sql.DB, values []string) ([]string, error) {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, raw := range values {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			continue
		}
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM registry_categories WHERE id=$1)`, id).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, errors.New("unknown category_id: " + id)
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func validateTestImageModes(values []string) ([]string, error) {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, raw := range values {
		mode := strings.TrimSpace(raw)
		if mode == "" || seen[mode] {
			continue
		}
		if !supportedProbeMode(mode) {
			return nil, errors.New("unsupported probe_mode: " + mode)
		}
		seen[mode] = true
		out = append(out, mode)
	}
	sort.Strings(out)
	return out, nil
}

func withTestImageRelations(ctx context.Context, db *sql.DB, id string, save func(*sql.Tx) (string, error), categoryIDs, probeModes []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var savedID string
	if savedID, err = save(tx); err != nil {
		return err
	}
	if savedID != "" {
		id = savedID
	}
	if id != "" {
		if _, err = tx.ExecContext(ctx, `DELETE FROM test_image_categories WHERE test_image_id=$1`, id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM test_image_probe_modes WHERE test_image_id=$1`, id); err != nil {
			return err
		}
		for _, categoryID := range categoryIDs {
			if _, err = tx.ExecContext(ctx, `INSERT INTO test_image_categories(test_image_id,category_id) VALUES($1,$2)`, id, categoryID); err != nil {
				return err
			}
		}
		for _, mode := range probeModes {
			if _, err = tx.ExecContext(ctx, `INSERT INTO test_image_probe_modes(test_image_id,probe_mode) VALUES($1,$2)`, id, mode); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
