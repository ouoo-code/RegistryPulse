package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) adminNotificationRules(w http.ResponseWriter, r *http.Request) {
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
		rows, err := db.QueryContext(r.Context(), `SELECT id::text,channel_id::text,event_type,enabled,cooldown_seconds,aggregation_seconds,template FROM notification_rules ORDER BY event_type`)
		if err != nil {
			writeError(w, 500, "RULE_QUERY_FAILED", "could not query notification rules")
			return
		}
		defer rows.Close()
		type item struct {
			ID                 string `json:"id"`
			ChannelID          string `json:"channel_id"`
			EventType          string `json:"event_type"`
			Enabled            bool   `json:"enabled"`
			CooldownSeconds    int    `json:"cooldown_seconds"`
			AggregationSeconds int    `json:"aggregation_seconds"`
			Template           string `json:"template"`
		}
		out := []item{}
		for rows.Next() {
			var v item
			if rows.Scan(&v.ID, &v.ChannelID, &v.EventType, &v.Enabled, &v.CooldownSeconds, &v.AggregationSeconds, &v.Template) == nil {
				out = append(out, v)
			}
		}
		writeData(w, out, map[string]any{"count": len(out)})
		return
	}
	if r.Method == http.MethodDelete {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/notification-rules/"), "/")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, 400, "INVALID_RULE", "notification rule id is required")
			return
		}
		res, err := db.ExecContext(r.Context(), `DELETE FROM notification_rules WHERE id=$1`, id)
		if err != nil {
			writeError(w, 500, "RULE_DELETE_FAILED", "could not delete notification rule")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, 404, "RULE_NOT_FOUND", "notification rule not found")
			return
		}
		s.audit(r, user, "notification_rule.delete", id, nil)
		writeData(w, map[string]bool{"deleted": true}, nil)
		return
	}
	var input struct {
		ID                 string `json:"id"`
		ChannelID          string `json:"channel_id"`
		EventType          string `json:"event_type"`
		Enabled            bool   `json:"enabled"`
		CooldownSeconds    int    `json:"cooldown_seconds"`
		AggregationSeconds int    `json:"aggregation_seconds"`
		Template           string `json:"template"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.ChannelID == "" || input.EventType == "" {
		writeError(w, 400, "INVALID_RULE", "channel_id and event_type are required")
		return
	}
	if input.CooldownSeconds < 0 {
		input.CooldownSeconds = 300
	}
	if input.AggregationSeconds < 0 {
		input.AggregationSeconds = 0
	}
	if r.Method == http.MethodPost {
		err := db.QueryRowContext(r.Context(), `INSERT INTO notification_rules(channel_id,event_type,enabled,cooldown_seconds,aggregation_seconds,template) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`, input.ChannelID, input.EventType, input.Enabled, input.CooldownSeconds, input.AggregationSeconds, input.Template).Scan(&input.ID)
		if err != nil {
			writeError(w, 400, "RULE_CREATE_FAILED", err.Error())
			return
		}
		s.audit(r, user, "notification_rule.create", input.ID, input)
		writeJSON(w, 201, input, nil)
		return
	}
	if r.Method == http.MethodPut {
		if input.ID == "" {
			input.ID = strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/notification-rules/"), "/")
		}
		res, err := db.ExecContext(r.Context(), `UPDATE notification_rules SET channel_id=$1,event_type=$2,enabled=$3,cooldown_seconds=$4,aggregation_seconds=$5,template=$6,updated_at=now() WHERE id=$7`, input.ChannelID, input.EventType, input.Enabled, input.CooldownSeconds, input.AggregationSeconds, input.Template, input.ID)
		if err != nil {
			writeError(w, 500, "RULE_UPDATE_FAILED", "could not update notification rule")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, 404, "RULE_NOT_FOUND", "notification rule not found")
			return
		}
		s.audit(r, user, "notification_rule.update", input.ID, input)
		writeData(w, input, nil)
		return
	}
	writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
}
