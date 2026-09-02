package cloud

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CreateAgent creates an agent and enqueues its initial run.
func (c *Client) CreateAgent(ctx context.Context, req CreateAgentRequest) (*CreateAgentResponse, error) {
	if strings.TrimSpace(req.Prompt.Text) == "" {
		return nil, errors.New("cursor cloud: prompt text must not be empty")
	}
	var out CreateAgentResponse
	if err := c.do(ctx, http.MethodPost, "/v1/agents", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Agent fetches one agent by id.
func (c *Client) Agent(ctx context.Context, agentID string) (*Agent, error) {
	if err := requireID("agentID", agentID); err != nil {
		return nil, err
	}
	var out Agent
	if err := c.do(ctx, http.MethodGet, "/v1/agents/"+pathEscape(agentID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAgents lists agents newest first. A zero limit uses the server default.
func (c *Client) ListAgents(ctx context.Context, limit int, cursor string) (*AgentList, error) {
	var out AgentList
	if err := c.do(ctx, http.MethodGet, "/v1/agents"+pageQuery(limit, cursor), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAgent permanently removes an agent.
func (c *Client) DeleteAgent(ctx context.Context, agentID string) error {
	if err := requireID("agentID", agentID); err != nil {
		return err
	}
	return c.do(ctx, http.MethodDelete, "/v1/agents/"+pathEscape(agentID), nil, nil)
}

// ArchiveAgent archives an agent.
func (c *Client) ArchiveAgent(ctx context.Context, agentID string) error {
	if err := requireID("agentID", agentID); err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, "/v1/agents/"+pathEscape(agentID)+"/archive", nil, nil)
}

// UnarchiveAgent restores an archived agent.
func (c *Client) UnarchiveAgent(ctx context.Context, agentID string) error {
	if err := requireID("agentID", agentID); err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, "/v1/agents/"+pathEscape(agentID)+"/unarchive", nil, nil)
}

// CreateRun sends a follow-up prompt to an existing agent.
func (c *Client) CreateRun(ctx context.Context, agentID string, req CreateRunRequest) (*Run, error) {
	if err := requireID("agentID", agentID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Prompt.Text) == "" {
		return nil, errors.New("cursor cloud: prompt text must not be empty")
	}
	var out CreateRunResponse
	if err := c.do(ctx, http.MethodPost, "/v1/agents/"+pathEscape(agentID)+"/runs", req, &out); err != nil {
		return nil, err
	}
	return &out.Run, nil
}

// Run fetches one run.
func (c *Client) Run(ctx context.Context, agentID, runID string) (*Run, error) {
	if err := requireID("agentID", agentID); err != nil {
		return nil, err
	}
	if err := requireID("runID", runID); err != nil {
		return nil, err
	}
	var out Run
	path := "/v1/agents/" + pathEscape(agentID) + "/runs/" + pathEscape(runID)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListRuns lists an agent's runs newest first.
func (c *Client) ListRuns(ctx context.Context, agentID string, limit int, cursor string) (*RunList, error) {
	if err := requireID("agentID", agentID); err != nil {
		return nil, err
	}
	var out RunList
	path := "/v1/agents/" + pathEscape(agentID) + "/runs" + pageQuery(limit, cursor)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelRun stops an in-flight run.
func (c *Client) CancelRun(ctx context.Context, agentID, runID string) error {
	if err := requireID("agentID", agentID); err != nil {
		return err
	}
	if err := requireID("runID", runID); err != nil {
		return err
	}
	path := "/v1/agents/" + pathEscape(agentID) + "/runs/" + pathEscape(runID) + "/cancel"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// AgentUsage reports an agent's accumulated usage.
func (c *Client) AgentUsage(ctx context.Context, agentID string) (*Usage, error) {
	if err := requireID("agentID", agentID); err != nil {
		return nil, err
	}
	var out Usage
	if err := c.do(ctx, http.MethodGet, "/v1/agents/"+pathEscape(agentID)+"/usage", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListModels returns the models available to cloud agents.
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	var out struct {
		Models []Model `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/models", nil, &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

func requireID(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("cursor cloud: " + field + " must not be empty")
	}
	return nil
}

func pageQuery(limit int, cursor string) string {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}
