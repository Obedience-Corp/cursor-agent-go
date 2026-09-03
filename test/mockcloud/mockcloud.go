// Package mockcloud provides an httptest server that impersonates the Cursor
// Cloud Agents v1 API, so cloud client behaviour can be exercised without an
// API key or a billable agent.
package mockcloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// Server is a stub Cloud Agents API.
type Server struct {
	*httptest.Server

	mu     sync.Mutex
	agents map[string]map[string]any
	runs   map[string]map[string]any
	seq    int

	// StreamEvents is written verbatim as the SSE body for a run stream.
	// Empty means a default transcript is served.
	StreamEvents string
	// FailNext, when set, makes the next request return this status and code
	// instead of its normal response.
	FailNext     int
	FailNextCode string
}

// Start boots a mock server. Callers must Close it.
func Start() *Server {
	s := &Server{
		agents: make(map[string]map[string]any),
		runs:   make(map[string]map[string]any),
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.route))
	return s
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	s.mu.Lock()
	if s.FailNext != 0 {
		status, code := s.FailNext, s.FailNextCode
		s.FailNext, s.FailNextCode = 0, ""
		s.mu.Unlock()
		writeErr(w, status, code, "injected failure")
		return
	}
	s.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/v1/")
	parts := strings.Split(path, "/")
	switch {
	case parts[0] == "agents" && len(parts) == 1 && r.Method == http.MethodPost:
		s.createAgent(w)
	case parts[0] == "agents" && len(parts) == 2 && r.Method == http.MethodGet:
		s.getAgent(w, parts[1])
	case parts[0] == "agents" && len(parts) == 3 && parts[2] == "runs" && r.Method == http.MethodPost:
		s.createRun(w, parts[1])
	case parts[0] == "agents" && len(parts) == 4 && parts[2] == "runs" && r.Method == http.MethodGet:
		s.getRun(w, parts[3])
	case parts[0] == "agents" && len(parts) == 5 && parts[4] == "stream":
		s.stream(w)
	case parts[0] == "agents" && len(parts) == 5 && parts[4] == "cancel":
		s.cancelRun(w, parts[3])
	case parts[0] == "agents" && len(parts) == 3 && parts[2] == "usage":
		writeJSON(w, 200, map[string]any{
			"agentId": parts[1], "inputTokens": 1200, "outputTokens": 340,
			"cacheReadTokens": 90, "totalCents": 7,
		})
	case parts[0] == "agents" && len(parts) == 3 && parts[2] == "artifacts":
		writeJSON(w, 200, map[string]any{"artifacts": []any{
			map[string]any{"path": "artifacts/out.txt", "sizeBytes": 12, "updatedAt": time.Now().UTC()},
		}})
	default:
		writeErr(w, http.StatusNotFound, "not_found", "no such route: "+r.URL.Path)
	}
}

func (s *Server) createAgent(w http.ResponseWriter) {
	s.mu.Lock()
	s.seq++
	agentID := fmt.Sprintf("bc-%08d", s.seq)
	runID := fmt.Sprintf("run-%08d", s.seq)
	agent := map[string]any{
		"id": agentID, "status": "ACTIVE", "url": "https://cursor.com/agents/" + agentID,
		"env": map[string]any{"type": "cloud"}, "createdAt": time.Now().UTC(), "updatedAt": time.Now().UTC(),
	}
	run := map[string]any{
		"id": runID, "agentId": agentID, "status": "CREATING",
		"createdAt": time.Now().UTC(), "updatedAt": time.Now().UTC(),
	}
	s.agents[agentID] = agent
	s.runs[runID] = run
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"agent": agent, "run": run})
}

func (s *Server) getAgent(w http.ResponseWriter, id string) {
	s.mu.Lock()
	agent, ok := s.agents[id]
	s.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "agent_not_found", "no such agent")
		return
	}
	writeJSON(w, 200, agent)
}

func (s *Server) createRun(w http.ResponseWriter, agentID string) {
	s.mu.Lock()
	if _, ok := s.agents[agentID]; !ok {
		s.mu.Unlock()
		writeErr(w, http.StatusNotFound, "agent_not_found", "no such agent")
		return
	}
	for _, run := range s.runs {
		if run["agentId"] == agentID && run["status"] != "FINISHED" {
			s.mu.Unlock()
			writeErr(w, http.StatusConflict, "agent_busy", "a run is already active")
			return
		}
	}
	s.seq++
	runID := fmt.Sprintf("run-%08d", s.seq)
	run := map[string]any{"id": runID, "agentId": agentID, "status": "CREATING",
		"createdAt": time.Now().UTC(), "updatedAt": time.Now().UTC()}
	s.runs[runID] = run
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"run": run})
}

// getRun advances a run one step per poll so WaitRun terminates.
func (s *Server) getRun(w http.ResponseWriter, runID string) {
	s.mu.Lock()
	run, ok := s.runs[runID]
	if ok {
		switch run["status"] {
		case "CREATING":
			run["status"] = "RUNNING"
		case "RUNNING":
			run["status"] = "FINISHED"
			run["result"] = "mock run complete"
			run["durationMs"] = 1234
		}
	}
	s.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "run_not_found", "no such run")
		return
	}
	writeJSON(w, 200, run)
}

func (s *Server) cancelRun(w http.ResponseWriter, runID string) {
	s.mu.Lock()
	run, ok := s.runs[runID]
	if ok {
		run["status"] = "CANCELLED"
	}
	s.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "run_not_found", "no such run")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DefaultStream is the SSE body served when StreamEvents is empty. It
// deliberately includes a comment line and id-less status and done events.
const DefaultStream = ": stream opened\n" +
	"event: status\ndata: {\"runId\":\"run-1\",\"status\":\"RUNNING\"}\n\n" +
	"id: evt-1\nevent: assistant\ndata: {\"text\":\"Hello \"}\n\n" +
	"id: evt-2\nevent: assistant\ndata: {\"text\":\"world\"}\n\n" +
	"id: evt-3\nevent: result\ndata: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"Hello world\",\"durationMs\":1234}\n\n" +
	"event: done\ndata: {}\n\n"

func (s *Server) stream(w http.ResponseWriter) {
	s.mu.Lock()
	body := s.StreamEvents
	s.mu.Unlock()
	if body == "" {
		body = DefaultStream
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Cursor-Stream-Retention-Seconds", "600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"code": code, "message": message},
	})
}
