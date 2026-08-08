package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

const agentTokenBytes = 32

func newAgentToken() (string, error) {
	b := make([]byte, agentTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "agt_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeAgentJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

func (s *Server) agentRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	var input domain.AgentRegisterInput
	if err := decodeAgentJSON(r, &input); err != nil || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Version) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_AGENT", "name and version are required")
		return
	}
	token, err := newAgentToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_GENERATION_FAILED", "could not create agent token")
		return
	}
	node := s.agents.Register(input, token)
	writeJSON(w, http.StatusCreated, map[string]any{"agent": node, "token": token, "token_type": "Bearer"}, nil)
}

func (s *Server) authenticatedAgent(r *http.Request) (domain.ProbeNode, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(header) < 7 || !strings.EqualFold(header[:7], "bearer ") {
		return domain.ProbeNode{}, false
	}
	token := strings.TrimSpace(header[7:])
	if token == "" {
		return domain.ProbeNode{}, false
	}
	node, ok := s.agents.Authenticate(token)
	if !ok || !s.verifyAgentRequest(r, token) {
		return domain.ProbeNode{}, false
	}
	return node, true
}

func (s *Server) agentHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	node, ok := s.authenticatedAgent(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "AGENT_UNAUTHORIZED", "valid agent bearer token required")
		return
	}
	var input domain.AgentHeartbeatInput
	if r.ContentLength != 0 {
		if err := decodeAgentJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid heartbeat payload")
			return
		}
	}
	updated, err := s.agents.Heartbeat(node.ID, input)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "AGENT_UNAUTHORIZED", "agent is not registered")
		return
	}
	writeData(w, map[string]any{"agent": updated, "server_time": time.Now().UTC()}, nil)
}

func (s *Server) agentPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	node, ok := s.authenticatedAgent(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "AGENT_UNAUTHORIZED", "valid agent bearer token required")
		return
	}
	var input domain.AgentPollInput
	if r.ContentLength != 0 {
		if err := decodeAgentJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid poll payload")
			return
		}
	}
	tasks := s.agents.Poll(node.ID, input.Limit, 2*time.Minute)
	writeData(w, map[string]any{"tasks": tasks, "count": len(tasks), "server_time": time.Now().UTC()}, nil)
}

func (s *Server) agentTask(w http.ResponseWriter, r *http.Request) {
	node, ok := s.authenticatedAgent(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "AGENT_UNAUTHORIZED", "valid agent bearer token required")
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/agent/tasks/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "INVALID_TASK", "task action is required")
		return
	}
	taskID, action := parts[0], parts[1]
	switch action {
	case "start":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		task, err := s.agents.StartTask(node.ID, taskID)
		if err != nil {
			s.writeTaskError(w, err)
			return
		}
		writeData(w, task, nil)
	case "result":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		var input domain.AgentResultInput
		if err := decodeAgentJSON(r, &input); err != nil || input.Status == "" {
			writeError(w, http.StatusBadRequest, "INVALID_RESULT", "status is required")
			return
		}
		task, err := s.agents.CompleteTask(node.ID, taskID, input)
		if err != nil {
			s.writeTaskError(w, err)
			return
		}
		writeData(w, task, nil)
	case "fail":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		var input domain.AgentFailureInput
		if err := decodeAgentJSON(r, &input); err != nil || strings.TrimSpace(input.Error) == "" {
			writeError(w, http.StatusBadRequest, "INVALID_FAILURE", "error is required")
			return
		}
		task, err := s.agents.FailTask(node.ID, taskID, strings.TrimSpace(input.Error))
		if err != nil {
			s.writeTaskError(w, err)
			return
		}
		writeData(w, task, nil)
	default:
		writeError(w, http.StatusNotFound, "TASK_ACTION_NOT_FOUND", "task action not found")
	}
}

func (s *Server) writeTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "TASK_NOT_FOUND", "task not found")
	case errors.Is(err, domain.ErrTaskNotAssigned):
		writeError(w, http.StatusConflict, "TASK_NOT_ASSIGNED", "task is not assigned to this agent")
	case errors.Is(err, domain.ErrTaskState):
		writeError(w, http.StatusConflict, "TASK_INVALID_STATE", "task is not in a mutable state")
	default:
		writeError(w, http.StatusInternalServerError, "TASK_FAILED", "could not update task")
	}
}
