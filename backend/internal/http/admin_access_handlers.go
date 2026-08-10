package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ouoo-code/RegistryPulse/internal/auth"
)

var managedPermissions = map[string]struct{}{
	"source.read": {}, "source.write": {}, "probe.read": {}, "probe.write": {},
	"incident.read": {}, "settings.read": {}, "settings.write": {},
	"audit.read": {}, "agent.manage": {},
}

func validRoleName(name string) bool {
	if name == "" || len(name) > 64 || strings.ContainsAny(name, " /\\\t\r\n") {
		return false
	}
	return true
}

func validManagedPermissions(values []string) bool {
	for _, value := range values {
		if _, ok := managedPermissions[value]; !ok {
			return false
		}
	}
	return true
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	db := s.db(w)
	if db == nil {
		return
	}
	if r.Method == http.MethodGet {
		id := ""
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/users/") {
			id = strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/"), "/")
		}
		if id != "" {
			var item struct {
				ID          string `json:"id"`
				Username    string `json:"username"`
				Roles       string `json:"roles"`
				Active      bool   `json:"active"`
				TOTPEnabled bool   `json:"totp_enabled"`
			}
			err := db.QueryRowContext(r.Context(), `SELECT u.id::text,u.username,u.is_active,COALESCE(u.totp_enabled,false),COALESCE(string_agg(r.name,',' ORDER BY r.name),'') FROM users u LEFT JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN roles r ON r.id=ur.role_id WHERE u.id=$1 GROUP BY u.id,u.username,u.is_active,u.totp_enabled`, id).Scan(&item.ID, &item.Username, &item.Active, &item.TOTPEnabled, &item.Roles)
			if err != nil {
				writeError(w, 404, "USER_NOT_FOUND", "user not found")
				return
			}
			writeData(w, item, nil)
			return
		}
		rows, err := db.QueryContext(r.Context(), `SELECT u.id::text,u.username,u.is_active,COALESCE(u.totp_enabled,false),COALESCE(string_agg(r.name,',' ORDER BY r.name),'') FROM users u LEFT JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN roles r ON r.id=ur.role_id GROUP BY u.id,u.username,u.is_active,u.totp_enabled ORDER BY u.username`)
		if err != nil {
			writeError(w, 500, "USER_QUERY_FAILED", "could not query users")
			return
		}
		defer rows.Close()
		type item struct {
			ID          string `json:"id"`
			Username    string `json:"username"`
			Active      bool   `json:"active"`
			Roles       string `json:"roles"`
			TOTPEnabled bool   `json:"totp_enabled"`
		}
		out := []item{}
		for rows.Next() {
			var v item
			if rows.Scan(&v.ID, &v.Username, &v.Active, &v.TOTPEnabled, &v.Roles) == nil {
				out = append(out, v)
			}
		}
		writeData(w, out, map[string]any{"count": len(out)})
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Active   *bool  `json:"active"`
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/"), "/")
	if r.Method == http.MethodPost && strings.HasSuffix(id, "/totp/reset") {
		id = strings.TrimSuffix(id, "/totp/reset")
		var username string
		if err := db.QueryRowContext(r.Context(), `SELECT username FROM users WHERE id=$1`, id).Scan(&username); err != nil {
			writeError(w, 404, "USER_NOT_FOUND", "user not found")
			return
		}
		if _, err := db.ExecContext(r.Context(), `UPDATE users SET totp_secret_ciphertext='',totp_secret_nonce='',totp_secret='',totp_enabled=false,updated_at=now() WHERE id=$1`, id); err != nil {
			writeError(w, 500, "TOTP_RESET_FAILED", "could not reset TOTP")
			return
		}
		_, _ = db.ExecContext(r.Context(), `DELETE FROM sessions WHERE user_id=$1`, id)
		s.audit(r, user, "auth.totp.reset", "user/"+id, map[string]any{"username": username})
		writeData(w, map[string]any{"id": id, "username": username, "totp_enabled": false}, nil)
		return
	}
	if r.Method == http.MethodDelete {
		if id == "" {
			writeError(w, 400, "INVALID_USER", "user id is required")
			return
		}
		if id == user.ID {
			writeError(w, 400, "SELF_DELETE_FORBIDDEN", "the current administrator cannot delete itself")
			return
		}
		var username string
		var active bool
		if err := db.QueryRowContext(r.Context(), `SELECT username,is_active FROM users WHERE id=$1`, id).Scan(&username, &active); err != nil {
			writeError(w, 404, "USER_NOT_FOUND", "user not found")
			return
		}
		if active {
			writeError(w, 409, "USER_MUST_BE_DISABLED", "disable the user before deleting it")
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			writeError(w, 500, "USER_DELETE_FAILED", "could not begin user deletion")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE audit_logs SET actor_username=$1 WHERE user_id=$2 AND COALESCE(actor_username,'')=''`, username, id); err != nil {
			_ = tx.Rollback()
			writeError(w, 500, "USER_DELETE_FAILED", "could not preserve audit actor")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM users WHERE id=$1 AND is_active=false`, id); err != nil {
			_ = tx.Rollback()
			writeError(w, 500, "USER_DELETE_FAILED", "could not delete user")
			return
		}
		if err = tx.Commit(); err != nil {
			writeError(w, 500, "USER_DELETE_FAILED", "could not commit user deletion")
			return
		}
		s.audit(r, user, "user.delete", id, map[string]any{"deleted_username": username})
		writeData(w, map[string]any{"id": id, "username": username, "deleted": true}, nil)
		return
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || strings.TrimSpace(input.Username) == "" || (r.Method == http.MethodPost && input.Password == "") {
		writeError(w, 400, "INVALID_USER", "username is required; password is required when creating a user")
		return
	}
	if r.Method == http.MethodPut {
		if id == "" {
			writeError(w, 400, "INVALID_USER", "user id is required")
			return
		}
		if input.Active != nil && !*input.Active && id == user.ID {
			writeError(w, 400, "SELF_DISABLE_FORBIDDEN", "the current administrator cannot disable itself")
			return
		}
		if _, err := db.ExecContext(r.Context(), `UPDATE users SET username=$1,is_active=COALESCE($2,is_active),updated_at=now() WHERE id=$3`, strings.TrimSpace(input.Username), input.Active, id); err != nil {
			writeError(w, 400, "USER_UPDATE_FAILED", err.Error())
			return
		}
		if input.Password != "" {
			hash, err := auth.HashPassword(input.Password)
			if err != nil {
				writeError(w, 500, "PASSWORD_HASH_FAILED", err.Error())
				return
			}
			if _, err = db.ExecContext(r.Context(), `UPDATE users SET password_hash=$1,updated_at=now() WHERE id=$2`, hash, id); err != nil {
				writeError(w, 400, "USER_UPDATE_FAILED", err.Error())
				return
			}
		}
		if input.Role != "" {
			var roleExists bool
			if err := db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM roles WHERE name=$1)`, input.Role).Scan(&roleExists); err != nil || !roleExists {
				writeError(w, 400, "ROLE_NOT_FOUND", "role does not exist")
				return
			}
			_, _ = db.ExecContext(r.Context(), `DELETE FROM user_roles WHERE user_id=$1`, id)
			_, _ = db.ExecContext(r.Context(), `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name=$2 ON CONFLICT DO NOTHING`, id, input.Role)
		}
		s.audit(r, user, "user.update", id, map[string]any{"username": input.Username, "role": input.Role})
		writeData(w, map[string]any{"id": id, "username": input.Username}, nil)
		return
	}
	id, err := auth.CreateUser(r.Context(), db, strings.TrimSpace(input.Username), input.Password)
	if err != nil {
		writeError(w, 400, "USER_CREATE_FAILED", err.Error())
		return
	}
	role := input.Role
	if role == "" {
		role = "viewer"
	}
	var roleExists bool
	if err = db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM roles WHERE name=$1)`, role).Scan(&roleExists); err != nil || !roleExists {
		writeError(w, 400, "ROLE_NOT_FOUND", "role does not exist")
		return
	}
	if _, err = db.ExecContext(r.Context(), `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name=$2 ON CONFLICT DO NOTHING`, id, role); err != nil {
		writeError(w, 500, "ROLE_ASSIGN_FAILED", "could not assign role")
		return
	}
	s.audit(r, user, "user.create", id, map[string]any{"username": input.Username, "role": role})
	writeJSON(w, 201, map[string]any{"id": id, "username": input.Username, "role": role}, nil)
}

func (s *Server) adminRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	db := s.db(w)
	if db == nil {
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/roles/"), "/")
	if r.Method == http.MethodDelete {
		if name == "" {
			writeError(w, 400, "INVALID_ROLE", "role name is required")
			return
		}
		if name == "admin" || name == "operator" || name == "viewer" {
			writeError(w, 409, "BUILTIN_ROLE", "built-in roles cannot be deleted")
			return
		}
		var users int
		if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE r.name=$1`, name).Scan(&users); err != nil {
			writeError(w, 500, "ROLE_DELETE_FAILED", "could not check role assignments")
			return
		}
		if users > 0 {
			writeError(w, 409, "ROLE_IN_USE", "role is assigned to one or more users")
			return
		}
		result, err := db.ExecContext(r.Context(), `DELETE FROM roles WHERE name=$1`, name)
		if err != nil {
			writeError(w, 500, "ROLE_DELETE_FAILED", "could not delete role")
			return
		}
		if count, _ := result.RowsAffected(); count == 0 {
			writeError(w, 404, "ROLE_NOT_FOUND", "role not found")
			return
		}
		actor, _ := s.currentUser(r)
		s.audit(r, actor, "role.delete", name, nil)
		writeData(w, map[string]any{"name": name, "deleted": true}, nil)
		return
	}
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		var input struct {
			Name        string   `json:"name"`
			Permissions []string `json:"permissions"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			writeError(w, 400, "INVALID_ROLE", "invalid role payload")
			return
		}
		if name == "" {
			name = strings.TrimSpace(input.Name)
		}
		if !validRoleName(name) || (name == "admin" && r.Method == http.MethodPut) || !validManagedPermissions(input.Permissions) {
			writeError(w, 400, "INVALID_ROLE", "invalid role name or permission")
			return
		}
		if name == "" {
			writeError(w, 400, "INVALID_ROLE", "role name is required")
			return
		}
		if _, err := db.ExecContext(r.Context(), `INSERT INTO roles(name) VALUES($1) ON CONFLICT(name) DO NOTHING`, name); err != nil {
			writeError(w, 400, "ROLE_WRITE_FAILED", err.Error())
			return
		}
		if r.Method == http.MethodPut {
			_, _ = db.ExecContext(r.Context(), `DELETE FROM role_permissions WHERE role_id=(SELECT id FROM roles WHERE name=$1)`, name)
		}
		for _, permission := range input.Permissions {
			_, _ = db.ExecContext(r.Context(), `INSERT INTO permissions(name) VALUES($1) ON CONFLICT(name) DO NOTHING`, permission)
			_, _ = db.ExecContext(r.Context(), `INSERT INTO role_permissions(role_id,permission_id) SELECT r.id,p.id FROM roles r,permissions p WHERE r.name=$1 AND p.name=$2 ON CONFLICT DO NOTHING`, name, permission)
		}
		user, _ := s.currentUser(r)
		s.audit(r, user, "role.write", name, input)
		writeData(w, map[string]any{"name": name, "permissions": input.Permissions}, nil)
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT r.name,COALESCE(string_agg(DISTINCT p.name,',' ORDER BY p.name),''),COUNT(DISTINCT ur.user_id) FROM roles r LEFT JOIN role_permissions rp ON rp.role_id=r.id LEFT JOIN permissions p ON p.id=rp.permission_id LEFT JOIN user_roles ur ON ur.role_id=r.id GROUP BY r.id,r.name ORDER BY r.name`)
	if err != nil {
		writeError(w, 500, "ROLE_QUERY_FAILED", "could not query roles")
		return
	}
	defer rows.Close()
	type item struct {
		Name        string `json:"name"`
		Permissions string `json:"permissions"`
		UserCount   int    `json:"user_count"`
	}
	out := []item{}
	for rows.Next() {
		var v item
		if rows.Scan(&v.Name, &v.Permissions, &v.UserCount) == nil {
			out = append(out, v)
		}
	}
	writeData(w, out, map[string]any{"count": len(out)})
}
