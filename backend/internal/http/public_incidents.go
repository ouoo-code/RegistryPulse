package httpapi

import (
	"github.com/ouoo-code/RegistryPulse/internal/incident"
	"net/http"
)

func (s *Server) sourceIncidents(w http.ResponseWriter, r *http.Request, sourceID string) {
	reader, ok := s.store.(incidentReader)
	if !ok {
		writeData(w, []incident.Incident{}, map[string]any{"count": 0})
		return
	}
	items, err := reader.Incidents(r.Context(), sourceID, 100)
	if err != nil {
		writeError(w, 500, "INCIDENT_QUERY_FAILED", "could not query incidents")
		return
	}
	writeData(w, items, map[string]any{"count": len(items)})
}
