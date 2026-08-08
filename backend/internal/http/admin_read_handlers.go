package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/incident"
)

type incidentReader interface {
	Incidents(ctx context.Context, sourceID string, limit int) ([]incident.Incident, error)
}

func (s *Server) results(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	items := make([]any, 0)
	for _, source := range s.store.Sources() {
		for _, result := range s.store.History(source.ID, 200) {
			items = append(items, result)
		}
	}
	writeData(w, items, map[string]any{"count": len(items)})
}

func (s *Server) adminResults(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePermission(w, r, "probe.read"); !ok {
		return
	}
	s.results(w, r)
}

func (s *Server) adminIncidents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePermission(w, r, "incident.read"); !ok {
		return
	}
	reader, ok := s.store.(incidentReader)
	if !ok {
		writeData(w, []incident.Incident{}, map[string]any{"count": 0})
		return
	}
	items, err := reader.Incidents(r.Context(), r.URL.Query().Get("source_id"), 200)
	if err != nil {
		writeError(w, 500, "INCIDENT_QUERY_FAILED", "could not query incidents")
		return
	}
	writeData(w, items, map[string]any{"count": len(items)})
}

func (s *Server) adminProbes(w http.ResponseWriter, r *http.Request) {
	permission := "probe.read"
	if r.Method != http.MethodGet {
		permission = "probe.write"
	}
	user, ok := s.requirePermission(w, r, permission)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		items := s.agents.Nodes()
		writeData(w, items, map[string]any{"count": len(items)})
		return
	}
	if r.Method == http.MethodPost {
		var input domain.AgentRegisterInput
		if json.NewDecoder(r.Body).Decode(&input) != nil || input.Name == "" || input.Version == "" {
			writeError(w, 400, "INVALID_AGENT", "name and version are required")
			return
		}
		token, err := newAgentToken()
		if err != nil {
			writeError(w, 500, "TOKEN_GENERATION_FAILED", "could not create agent token")
			return
		}
		node := s.agents.Register(input, token)
		s.audit(r, user, "agent.create", node.ID, map[string]any{"name": node.Name})
		writeJSON(w, 201, map[string]any{"agent": node, "token": token}, nil)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/probes/"), "/")
	if id == "" {
		writeError(w, 400, "INVALID_AGENT", "agent id is required")
		return
	}
	if r.Method == http.MethodPut {
		var input domain.AgentHeartbeatInput
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			writeError(w, 400, "INVALID_JSON", "invalid request body")
			return
		}
		node, err := s.agents.Heartbeat(id, input)
		if err != nil {
			writeError(w, 404, "AGENT_NOT_FOUND", "agent not found")
			return
		}
		s.audit(r, user, "agent.update", id, input)
		writeData(w, node, nil)
		return
	}
	if r.Method == http.MethodDelete {
		if err := s.agents.Remove(id); err != nil {
			writeError(w, 404, "AGENT_NOT_FOUND", "agent not found")
			return
		}
		s.audit(r, user, "agent.delete", id, nil)
		writeData(w, map[string]bool{"deleted": true}, nil)
		return
	}
	writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
}
