package cloud

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Stream event types emitted by the run stream.
const (
	EventStatus            = "status"
	EventAssistant         = "assistant"
	EventThinking          = "thinking"
	EventToolCall          = "tool_call"
	EventInteractionUpdate = "interaction_update"
	EventHeartbeat         = "heartbeat"
	EventResult            = "result"
	EventError             = "error"
	EventDone              = "done"
)

// StreamEvent is one Server-Sent Event from a run stream.
type StreamEvent struct {
	// ID is the SSE id line. Empty for status and heartbeat events; only
	// non-empty ids are valid for Last-Event-ID resume.
	ID string
	// Type is the SSE event line, one of the Event* constants. Unknown
	// types are delivered unchanged.
	Type string
	// Data is the raw JSON payload.
	Data json.RawMessage
}

// StatusData is the payload of a status event.
type StatusData struct {
	RunID  string `json:"runId"`
	Status string `json:"status"`
}

// TextData is the payload of assistant and thinking events.
type TextData struct {
	RunID string `json:"runId,omitempty"`
	Text  string `json:"text"`
}

// ResultData is the payload of the terminal result event.
type ResultData struct {
	RunID      string  `json:"runId"`
	Status     string  `json:"status"`
	Text       string  `json:"text,omitempty"`
	DurationMs int     `json:"durationMs,omitempty"`
	Git        *RunGit `json:"git,omitempty"`
}

// StreamOptions tunes a run stream.
type StreamOptions struct {
	// LastEventID resumes after a disconnect. It must be an id previously
	// received for this same run, or the API returns 400
	// invalid_last_event_id.
	LastEventID string
}

// RunStream is an open Server-Sent Events stream for one run.
type RunStream struct {
	// RetentionSeconds is the server's stream retention window, from the
	// X-Cursor-Stream-Retention-Seconds header. Zero when absent.
	RetentionSeconds int

	body    io.ReadCloser
	scanner *bufio.Scanner
	lastID  string
}

// StreamRun opens the SSE stream for a run. The caller must Close the stream.
func (c *Client) StreamRun(ctx context.Context, agentID, runID string, opts *StreamOptions) (*RunStream, error) {
	if err := requireID("agentID", agentID); err != nil {
		return nil, err
	}
	if err := requireID("runID", runID); err != nil {
		return nil, err
	}
	path := "/v1/agents/" + pathEscape(agentID) + "/runs/" + pathEscape(runID) + "/stream"
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if opts != nil && opts.LastEventID != "" {
		req.Header.Set("Last-Event-ID", opts.LastEventID)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		return nil, decodeAPIError(resp)
	}

	stream := &RunStream{body: resp.Body, scanner: bufio.NewScanner(resp.Body)}
	stream.scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	if v := resp.Header.Get("X-Cursor-Stream-Retention-Seconds"); v != "" {
		stream.RetentionSeconds, _ = strconv.Atoi(v)
	}
	if opts != nil {
		stream.lastID = opts.LastEventID
	}
	return stream, nil
}

// Recv returns the next event, or io.EOF when the stream ends.
func (s *RunStream) Recv() (*StreamEvent, error) {
	var (
		id    string
		name  string
		data  strings.Builder
		lines int
	)
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" {
			if lines == 0 {
				continue
			}
			event := &StreamEvent{ID: id, Type: name, Data: json.RawMessage(data.String())}
			if event.Type == "" {
				event.Type = "message"
			}
			if event.ID != "" {
				s.lastID = event.ID
			}
			return event, nil
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			field, value = line, ""
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "id":
			id = value
			lines++
		case "event":
			name = value
			lines++
		case "data":
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(value)
			lines++
		}
	}
	if err := s.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// LastEventID is the most recent non-empty event id, suitable for resuming.
func (s *RunStream) LastEventID() string { return s.lastID }

// Close releases the underlying response body.
func (s *RunStream) Close() error { return s.body.Close() }
