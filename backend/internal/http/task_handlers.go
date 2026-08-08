package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

func (s *Server) adminTasks(w http.ResponseWriter, r *http.Request) {
	permission := "probe.read"
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
		permission = "probe.write"
	}
	if _, ok := s.requirePermission(w, r, permission); !ok {
		return
	}
	if r.Method == http.MethodGet {
		limit := 100
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if value, err := strconv.Atoi(raw); err == nil {
				limit = value
			}
		}
		items := s.agents.Tasks(limit)
		status, sourceID, nodeID := r.URL.Query().Get("status"), r.URL.Query().Get("source_id"), r.URL.Query().Get("probe_node_id")
		filtered := items[:0]
		for _, item := range items {
			if (status == "" || item.Status == status) && (sourceID == "" || item.SourceID == sourceID) && (nodeID == "" || item.ProbeNodeID == nodeID) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
		sort.SliceStable(items, func(i, j int) bool {
			less := items[i].CreatedAt.Before(items[j].CreatedAt)
			if r.URL.Query().Get("order") == "desc" {
				return !less && items[i].ID != items[j].ID
			}
			return less
		})
		page, pageSize := queryPage(r)
		total := len(items)
		pages := (total + pageSize - 1) / pageSize
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		writeData(w, items[start:end], map[string]any{"count": total, "page": page, "page_size": pageSize, "pages": pages})
		return
	}
	if r.Method == http.MethodDelete {
		deleted, err := s.agents.ClearTasks()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "TASK_CLEAR_FAILED", "could not clear completed tasks")
			return
		}
		user, _ := s.currentUser(r)
		s.audit(r, user, "probe.tasks_clear", "tasks", map[string]any{"deleted": deleted})
		writeData(w, map[string]any{"deleted": deleted}, nil)
		return
	}
	if r.Method == http.MethodPut {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/tasks/"), "/")
		if id == "" {
			writeError(w, 400, "INVALID_TASK", "task id is required")
			return
		}
		var input struct {
			Action string `json:"action"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			writeError(w, 400, "INVALID_TASK", "invalid task action")
			return
		}
		var task domain.ProbeTask
		var err error
		if input.Action == "cancel" {
			task, err = s.agents.CancelTask(id)
		} else if input.Action == "retry" {
			task, err = s.agents.RetryTask(id)
		} else {
			writeError(w, 400, "INVALID_TASK_ACTION", "action must be cancel or retry")
			return
		}
		if err != nil {
			s.writeTaskError(w, err)
			return
		}
		user, _ := s.currentUser(r)
		s.audit(r, user, "probe.task_"+input.Action, id, nil)
		writeData(w, task, nil)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	var input struct {
		SourceID string         `json:"source_id"`
		Type     string         `json:"type"`
		Payload  map[string]any `json:"payload"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.SourceID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_TASK", "source_id is required")
		return
	}
	if input.Type == "" {
		input.Type = "oci_probe"
	}
	task := s.agents.Enqueue(input.SourceID, input.Type, input.Payload)
	user, _ := s.currentUser(r)
	s.audit(r, user, "probe.task_create", task.ID, input)
	writeJSON(w, http.StatusCreated, task, nil)
}
