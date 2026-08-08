package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ouoo-code/RegistryPulse/internal/auth"
)

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
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
		id := ""
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/users/") {
			id = strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/"), "/")
		}
		if id != "" {
			var item struct {
				ID, Username, Roles string
				Active              bool
			}
			err := db.QueryRowContext(r.Context(), `SELECT u.id::text,u.username,u.is_active,COALESCE(string_agg(r.name,',' ORDER BY r.name),'') FROM users u LEFT JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN roles r ON r.id=ur.role_id WHERE u.id=$1 GROUP BY u.id,u.username,u.is_active`, id).Scan(&item.ID, &item.Username, &item.Active, &item.Roles)
			if err != nil {
				writeError(w, 404, "USER_NOT_FOUND", "user not found")
				return
			}
			writeData(w, item, nil)
			return
		}
		rows, err := db.QueryContext(r.Context(), `SELECT u.id::text,u.username,u.is_active,COALESCE(string_agg(r.name,',' ORDER BY r.name),'') FROM users u LEFT JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN roles r ON r.id=ur.role_id GROUP BY u.id,u.username,u.is_active ORDER BY u.username`)
		if err != nil {
			writeError(w, 500, "USER_QUERY_FAILED", "could not query users")
			return
		}
		defer rows.Close()
		type item struct {
			ID, Username string
			Active       bool
			Roles        string
		}
		out := []item{}
		for rows.Next() {
			var v item
			if rows.Scan(&v.ID, &v.Username, &v.Active, &v.Roles) == nil {
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
	if r.Method == http.MethodDelete {
		if id == "" {
			writeError(w, 400, "INVALID_USER", "user id is required")
			return
		}
		if _, err := db.ExecContext(r.Context(), `UPDATE users SET is_active=false,updated_at=now() WHERE id=$1`, id); err != nil {
			writeError(w, 400, "USER_DELETE_FAILED", err.Error())
			return
		}
		s.audit(r, user, "user.disable", id, nil)
		writeData(w, map[string]any{"id": id, "active": false}, nil)
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
	if _, err = db.ExecContext(r.Context(), `INSERT INTO roles(name) VALUES($1) ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name`, role); err != nil {
		writeError(w, 500, "ROLE_ASSIGN_FAILED", "could not create role")
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
	permission := "settings.read"
	if r.Method != http.MethodGet {
		permission = "settings.write"
	}
	if _, ok := s.requirePermission(w, r, permission); !ok {
		return
	}
	db := s.db(w)
	if db == nil {
		return
	}
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/roles/"), "/")
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
	rows, err := db.QueryContext(r.Context(), `SELECT r.name,COALESCE(string_agg(p.name,',' ORDER BY p.name),'') FROM roles r LEFT JOIN role_permissions rp ON rp.role_id=r.id LEFT JOIN permissions p ON p.id=rp.permission_id GROUP BY r.name ORDER BY r.name`)
	if err != nil {
		writeError(w, 500, "ROLE_QUERY_FAILED", "could not query roles")
		return
	}
	defer rows.Close()
	type item struct{ Name, Permissions string }
	out := []item{}
	for rows.Next() {
		var v item
		if rows.Scan(&v.Name, &v.Permissions) == nil {
			out = append(out, v)
		}
	}
	writeData(w, out, map[string]any{"count": len(out)})
}
