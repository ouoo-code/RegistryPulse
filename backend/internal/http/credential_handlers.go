package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/ouoo-code/RegistryPulse/internal/credential"
	"github.com/ouoo-code/RegistryPulse/internal/database"
	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

func (s *Server) adminCredentialProfiles(w http.ResponseWriter, r *http.Request) {
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
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/credential-profiles/"), "/")
	if r.Method == http.MethodGet {
		items, err := database.QueryCredentialProfiles(r.Context(), db)
		if err != nil {
			writeError(w, 500, "CREDENTIAL_QUERY_FAILED", "could not query credential profiles")
			return
		}
		writeData(w, items, map[string]any{"count": len(items)})
		return
	}
	if r.Method == http.MethodDelete {
		if id == "" {
			writeError(w, 400, "INVALID_CREDENTIAL_PROFILE", "profile id is required")
			return
		}
		err := database.DeleteCredentialProfile(r.Context(), db, id)
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, 404, "CREDENTIAL_NOT_FOUND", "credential profile not found")
			return
		}
		if err != nil {
			writeError(w, 500, "CREDENTIAL_DELETE_FAILED", "could not delete credential profile")
			return
		}
		s.audit(r, user, "credential_profile.delete", id, nil)
		writeData(w, map[string]bool{"deleted": true}, nil)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	var input domain.CredentialProfileInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, 400, "INVALID_JSON", "invalid request body")
		return
	}
	if r.Method == http.MethodPut && input.ID == "" {
		input.ID = id
	}
	if input.ID != "" && r.Method == http.MethodPost {
		writeError(w, 400, "INVALID_CREDENTIAL_PROFILE", "post cannot contain an existing profile id")
		return
	}
	profile, err := database.UpsertCredentialProfile(r.Context(), db, input)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, 404, "CREDENTIAL_SELECTOR_NOT_FOUND", "source or category selector not found")
		return
	}
	if errors.Is(err, credential.ErrEncryptionKey) {
		writeError(w, 503, "CREDENTIAL_ENCRYPTION_UNAVAILABLE", "credential encryption is not configured")
		return
	}
	if err != nil {
		message := "could not save credential profile"
		if strings.Contains(err.Error(), "selector") || strings.Contains(err.Error(), "match") || strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid credential") {
			message = err.Error()
		}
		writeError(w, 400, "INVALID_CREDENTIAL_PROFILE", message)
		return
	}
	action := "credential_profile.create"
	if r.Method == http.MethodPut {
		action = "credential_profile.update"
	}
	s.audit(r, user, action, profile.ID, map[string]any{"name": profile.Name, "auth_type": profile.AuthType, "source_id": profile.SourceID, "registry_host": profile.RegistryHost, "category_id": profile.CategoryID, "has_secret": profile.HasSecret})
	if r.Method == http.MethodPost {
		writeJSON(w, http.StatusCreated, profile, nil)
	} else {
		writeData(w, profile, nil)
	}
}
