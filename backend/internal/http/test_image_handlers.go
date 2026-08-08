package httpapi

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

var testImageReference = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*:[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

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
	if r.Method == http.MethodGet {
		rows, err := db.QueryContext(r.Context(), `SELECT id::text,reference,enabled,max_bytes,is_default,created_at,updated_at FROM test_images ORDER BY reference`)
		if err != nil {
			writeError(w, 500, "TEST_IMAGE_QUERY_FAILED", "could not query test images")
			return
		}
		defer rows.Close()
		type image struct {
			ID        string `json:"id"`
			Reference string `json:"reference"`
			Enabled   bool   `json:"enabled"`
			MaxBytes  int64  `json:"max_bytes"`
			IsDefault bool   `json:"is_default"`
			CreatedAt any    `json:"created_at"`
			UpdatedAt any    `json:"updated_at"`
		}
		out := []image{}
		for rows.Next() {
			var item image
			if rows.Scan(&item.ID, &item.Reference, &item.Enabled, &item.MaxBytes, &item.IsDefault, &item.CreatedAt, &item.UpdatedAt) == nil {
				out = append(out, item)
			}
		}
		writeData(w, out, map[string]any{"count": len(out)})
		return
	}
	pathID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/test-images/"), "/")
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
		n, _ := res.RowsAffected()
		if n == 0 {
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
	var input struct {
		ID        string `json:"id"`
		Reference string `json:"reference"`
		Enabled   bool   `json:"enabled"`
		MaxBytes  int64  `json:"max_bytes"`
		IsDefault bool   `json:"is_default"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || !testImageReference.MatchString(strings.TrimSpace(input.Reference)) || strings.Contains(input.Reference, "..") {
		writeError(w, 400, "INVALID_TEST_IMAGE", "reference must be a safe tagged image reference")
		return
	}
	if input.MaxBytes <= 0 {
		input.MaxBytes = 64 << 20
	}
	if input.MaxBytes > 4<<30 {
		writeError(w, 400, "INVALID_TEST_IMAGE", "max_bytes is too large")
		return
	}
	if r.Method == http.MethodPost {
		if err := db.QueryRowContext(r.Context(), `INSERT INTO test_images(reference,enabled,max_bytes,is_default) VALUES($1,$2,$3,false) RETURNING id::text`, strings.TrimSpace(input.Reference), input.Enabled, input.MaxBytes).Scan(&input.ID); err != nil {
			writeError(w, 409, "TEST_IMAGE_EXISTS", "could not create test image")
			return
		}
		s.audit(r, user, "test_image.create", input.ID, map[string]any{"reference": input.Reference, "enabled": input.Enabled, "max_bytes": input.MaxBytes})
		writeJSON(w, 201, input, nil)
		return
	}
	if r.Method == http.MethodPut {
		if input.ID == "" {
			input.ID = strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/test-images/"), "/")
		}
		res, err := db.ExecContext(r.Context(), `UPDATE test_images SET reference=$1,enabled=$2,max_bytes=$3,updated_at=now() WHERE id=$4`, strings.TrimSpace(input.Reference), input.Enabled, input.MaxBytes, input.ID)
		if err != nil {
			writeError(w, 500, "TEST_IMAGE_UPDATE_FAILED", "could not update test image")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, 404, "TEST_IMAGE_NOT_FOUND", "test image not found")
			return
		}
		s.audit(r, user, "test_image.update", input.ID, map[string]any{"reference": input.Reference, "enabled": input.Enabled, "max_bytes": input.MaxBytes})
		writeData(w, input, nil)
		return
	}
	writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
}
