package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the production Cloud Agents endpoint.
const DefaultBaseURL = "https://api.cursor.com"

// Client talks to the Cursor Cloud Agents API.
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// New builds a client for the production API.
func New(apiKey string) *Client {
	return &Client{APIKey: apiKey, BaseURL: DefaultBaseURL}
}

// APIError is a non-2xx response from the Cloud Agents API.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Body       string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("cursor cloud: %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("cursor cloud: %d: %s", e.StatusCode, e.Message)
}

// IsRetryable reports whether retrying the request could succeed.
func (e *APIError) IsRetryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}

// IsNotFound reports whether the resource does not exist.
func (e *APIError) IsNotFound() bool { return e.StatusCode == http.StatusNotFound }

// IsUnauthorized reports whether the API key was rejected.
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden
}

// IsBusy reports a 409 caused by an agent already having an active run. Only
// one run is active per agent, so a follow-up prompt must wait.
func (e *APIError) IsBusy() bool {
	return e.StatusCode == http.StatusConflict && e.Code == CodeAgentBusy
}

// IsStreamExpired reports a 410 from the run stream, meaning the server's
// retention window passed and the events can no longer be replayed.
func (e *APIError) IsStreamExpired() bool {
	return e.StatusCode == http.StatusGone && e.Code == CodeStreamExpired
}

// IsInvalidLastEventID reports a 400 caused by resuming with an event id that
// does not belong to the requested run.
func (e *APIError) IsInvalidLastEventID() bool {
	return e.StatusCode == http.StatusBadRequest && e.Code == CodeInvalidLastEventID
}

// Error codes the API returns in the error envelope.
const (
	CodeAgentBusy          = "agent_busy"
	CodeAgentIDConflict    = "agent_id_conflict"
	CodeStreamExpired      = "stream_expired"
	CodeInvalidLastEventID = "invalid_last_event_id"
)

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) baseURL() string {
	if strings.TrimSpace(c.BaseURL) == "" {
		return DefaultBaseURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, errors.New("cursor cloud: APIKey must not be empty")
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	return req, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func decodeAPIError(resp *http.Response) *APIError {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	apiErr := &APIError{StatusCode: resp.StatusCode, Body: string(raw)}
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		apiErr.Code = firstNonEmpty(envelope.Error.Code, envelope.Code)
		apiErr.Message = firstNonEmpty(envelope.Error.Message, envelope.Message)
	}
	if apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(raw))
	}
	if after := resp.Header.Get("Retry-After"); after != "" {
		if secs, err := strconv.Atoi(after); err == nil {
			apiErr.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	return apiErr
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func pathEscape(id string) string { return url.PathEscape(id) }
