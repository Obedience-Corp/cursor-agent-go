package cloud

import "time"

// Image is an image input attached to a prompt. Provide either Data with
// MimeType, or URL.
type Image struct {
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	URL      string `json:"url,omitempty"`
}

// Prompt is the task instruction for an agent or run.
type Prompt struct {
	Text   string  `json:"text"`
	Images []Image `json:"images,omitempty"`
}

// ModelParam is one per-model parameter such as reasoning effort.
type ModelParam struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// ModelRef selects a model returned by ListModels.
type ModelRef struct {
	ID     string       `json:"id"`
	Params []ModelParam `json:"params,omitempty"`
}

// RepoConfig points an agent at a repository.
type RepoConfig struct {
	Repository string `json:"repository,omitempty"`
	Ref        string `json:"ref,omitempty"`
}

// AgentEnv selects the execution environment.
type AgentEnv struct {
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
}

// Agent lifecycle states.
const (
	AgentActive   = "ACTIVE"
	AgentArchived = "ARCHIVED"
)

// Agent is a durable cloud agent.
type Agent struct {
	ID                  string       `json:"id"`
	Name                string       `json:"name,omitempty"`
	Status              string       `json:"status"`
	Env                 AgentEnv     `json:"env"`
	URL                 string       `json:"url"`
	CreatedAt           time.Time    `json:"createdAt"`
	UpdatedAt           time.Time    `json:"updatedAt"`
	Repos               []RepoConfig `json:"repos,omitempty"`
	WorkOnCurrentBranch bool         `json:"workOnCurrentBranch,omitempty"`
	AutoCreatePR        bool         `json:"autoCreatePR,omitempty"`
}

// Run statuses. Terminal statuses are FINISHED, ERROR, CANCELLED and EXPIRED.
const (
	RunCreating  = "CREATING"
	RunRunning   = "RUNNING"
	RunFinished  = "FINISHED"
	RunError     = "ERROR"
	RunCancelled = "CANCELLED"
	RunExpired   = "EXPIRED"
)

// IsTerminal reports whether a run status will not change again.
func IsTerminal(status string) bool {
	switch status {
	case RunFinished, RunError, RunCancelled, RunExpired:
		return true
	}
	return false
}

// RunGit is the agent's pushed branch and PR state.
type RunGit struct {
	Branch string `json:"branch,omitempty"`
	PRURL  string `json:"prUrl,omitempty"`
}

// Run is one prompt submission against an agent.
type Run struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agentId"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	DurationMs int       `json:"durationMs,omitempty"`
	Result     string    `json:"result,omitempty"`
	Git        *RunGit   `json:"git,omitempty"`
}

// CreateAgentRequest creates an agent and enqueues its first run.
type CreateAgentRequest struct {
	Prompt              Prompt       `json:"prompt"`
	Model               *ModelRef    `json:"model,omitempty"`
	Name                string       `json:"name,omitempty"`
	AgentID             string       `json:"agentId,omitempty"`
	Env                 *AgentEnv    `json:"env,omitempty"`
	Repos               []RepoConfig `json:"repos,omitempty"`
	WorkOnCurrentBranch bool         `json:"workOnCurrentBranch,omitempty"`
	AutoCreatePR        bool         `json:"autoCreatePR,omitempty"`
}

// CreateAgentResponse carries the durable agent and its initial run.
type CreateAgentResponse struct {
	Agent Agent `json:"agent"`
	Run   Run   `json:"run"`
}

// CreateRunRequest sends a follow-up prompt to an existing agent.
type CreateRunRequest struct {
	Prompt Prompt    `json:"prompt"`
	Model  *ModelRef `json:"model,omitempty"`
}

// CreateRunResponse carries the newly enqueued run.
type CreateRunResponse struct {
	Run Run `json:"run"`
}

// AgentList is a page of agents.
type AgentList struct {
	Agents     []Agent `json:"agents"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

// RunList is a page of runs.
type RunList struct {
	Runs       []Run  `json:"runs"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// Usage is an agent's accumulated cost and token accounting.
type Usage struct {
	AgentID          string `json:"agentId,omitempty"`
	InputTokens      int    `json:"inputTokens,omitempty"`
	OutputTokens     int    `json:"outputTokens,omitempty"`
	CacheReadTokens  int    `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens int    `json:"cacheWriteTokens,omitempty"`
	TotalCents       int    `json:"totalCents,omitempty"`
}

// Artifact is one file the agent produced under the workspace artifacts/ dir.
type Artifact struct {
	Path      string    `json:"path"`
	SizeBytes int64     `json:"sizeBytes"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ArtifactList is the reply to ListArtifacts.
type ArtifactList struct {
	Artifacts []Artifact `json:"artifacts"`
}

// ArtifactDownload carries the presigned URL for one artifact.
type ArtifactDownload struct {
	URL string `json:"url"`
}

// Model is one selectable cloud model.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}
