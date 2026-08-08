package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (s *Server) configOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	type option struct {
		ID, Name, URL string
		Status        string
	}
	items := []option{}
	for _, source := range s.store.Sources() {
		if source.Enabled {
			items = append(items, option{source.ID, source.Name, source.BaseURL, source.Status})
		}
	}
	writeData(w, map[string]any{"formats": []string{"docker", "1panel", "podman", "containerd", "nerdctl"}, "sources": items}, nil)
}

func (s *Server) renderConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	var input struct {
		Format    string   `json:"format"`
		SourceIDs []string `json:"source_ids"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeError(w, 400, "INVALID_JSON", "invalid request body")
		return
	}
	format := strings.ToLower(strings.TrimSpace(input.Format))
	if format == "" {
		format = "docker"
	}
	allowed := map[string]bool{"docker": true, "1panel": true, "podman": true, "containerd": true, "nerdctl": true}
	if !allowed[format] {
		writeError(w, 400, "INVALID_FORMAT", "unsupported config format")
		return
	}
	urls := []string{}
	for _, source := range s.store.Sources() {
		if len(input.SourceIDs) > 0 && !contains(input.SourceIDs, source.ID) {
			continue
		}
		if !source.Enabled {
			continue
		}
		if u, err := url.Parse(source.BaseURL); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" {
			urls = append(urls, strings.TrimRight(source.BaseURL, "/"))
		}
	}
	if len(urls) == 0 {
		writeError(w, 400, "NO_SOURCES", "no enabled sources selected")
		return
	}
	writeData(w, map[string]any{"format": format, "content": renderConfig(format, urls), "source_count": len(urls)}, nil)
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
func renderConfig(format string, urls []string) string {
	switch format {
	case "docker", "1panel":
		b, _ := json.MarshalIndent(map[string]any{"registry-mirrors": urls}, "", "  ")
		return string(b)
	case "podman":
		var b strings.Builder
		b.WriteString("unqualified-search-registries = [\"docker.io\"]\n\n")
		for _, raw := range urls {
			u, _ := url.Parse(raw)
			fmt.Fprintf(&b, "[[registry.mirror]]\nlocation = \"%s\"\ninsecure = false\n\n", u.Host)
		}
		return b.String()
	case "containerd", "nerdctl":
		var b strings.Builder
		for _, raw := range urls {
			u, _ := url.Parse(raw)
			fmt.Fprintf(&b, "server = \"https://%s\"\n\n[host.\"%s\"]\n  capabilities = [\"pull\", \"resolve\"]\n  skip_verify = false\n\n", u.Host, u.Host)
		}
		return b.String()
	}
	return ""
}
