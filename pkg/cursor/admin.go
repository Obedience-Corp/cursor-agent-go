package cursor

import (
	"context"
	"encoding/json"
	"strings"
)

// Model is one entry from "cursor-agent models".
type Model struct {
	ID   string
	Name string
}

// About is the decoded "cursor-agent about --format json" payload.
type About struct {
	CLIVersion       string `json:"cliVersion"`
	LatestStatus     string `json:"latestStatus"`
	LatestVersion    string `json:"latestVersion"`
	Model            string `json:"model"`
	SubscriptionTier string `json:"subscriptionTier"`
	OSPlatform       string `json:"osPlatform"`
	OSArch           string `json:"osArch"`
	UserEmail        string `json:"userEmail"`
	TerminalProgram  string `json:"terminalProgram"`
	Shell            string `json:"shell"`
	LastRequestID    string `json:"lastRequestId"`
}

// IsUpToDate reports whether the CLI matches the latest published release.
func (a *About) IsUpToDate() bool { return a.LatestStatus == "up_to_date" }

// UserInfo is the account block inside Status.
//
// It carries personal data. Do not log it or embed it in captured fixtures.
type UserInfo struct {
	Email     string `json:"email"`
	UserID    int64  `json:"userId"`
	FirstName string `json:"firstName"`
	CreatedAt string `json:"createdAt"`
}

// Status is the decoded "cursor-agent status --format json" payload.
type Status struct {
	Status          string          `json:"status"`
	IsAuthenticated bool            `json:"isAuthenticated"`
	HasAccessToken  bool            `json:"hasAccessToken"`
	HasRefreshToken bool            `json:"hasRefreshToken"`
	UserInfo        UserInfo        `json:"userInfo"`
	Raw             json.RawMessage `json:"-"`
}

// About runs "cursor-agent about --format json".
func (c *Client) About(ctx context.Context) (*About, error) {
	raw, err := c.adminJSON(ctx, "about")
	if err != nil {
		return nil, err
	}
	var out About
	if jsonErr := json.Unmarshal(raw, &out); jsonErr != nil {
		return nil, validationErrorWith("cursor-agent about stdout is not JSON: "+truncate(raw, 400), jsonErr)
	}
	return &out, nil
}

// Status runs "cursor-agent status --format json".
func (c *Client) Status(ctx context.Context) (*Status, error) {
	raw, err := c.adminJSON(ctx, "status")
	if err != nil {
		return nil, err
	}
	var out Status
	if jsonErr := json.Unmarshal(raw, &out); jsonErr != nil {
		return nil, validationErrorWith("cursor-agent status stdout is not JSON: "+truncate(raw, 400), jsonErr)
	}
	out.Raw = append(json.RawMessage(nil), raw...)
	return &out, nil
}

// Models runs "cursor-agent models" and parses its "id - Name" listing.
//
// The subcommand has no JSON mode, so this parses the text listing. Lines that
// do not carry an "id - Name" pair, such as the "Available models" header, are
// skipped.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, &Error{Kind: KindInterrupted, Message: "context done before cursor-agent models", Original: ctxErr}
	}
	outcome := c.runCommand(ctx, []string{"models"}, BuildEnv(c.APIKey, c.Endpoint, nil), "", nil)
	if sdkErr := classifyAdmin("models", outcome); sdkErr != nil {
		return nil, sdkErr
	}
	var models []Model
	for line := range strings.SplitSeq(string(outcome.stdout), "\n") {
		id, name, found := strings.Cut(strings.TrimSpace(line), " - ")
		if !found || id == "" || strings.ContainsAny(id, " \t") {
			continue
		}
		models = append(models, Model{ID: id, Name: name})
	}
	if len(models) == 0 {
		return nil, validationError("cursor-agent models returned no parseable entries")
	}
	return models, nil
}

func (c *Client) adminJSON(ctx context.Context, subcommand string) ([]byte, *Error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, &Error{Kind: KindInterrupted, Message: "context done before cursor-agent " + subcommand, Original: ctxErr}
	}
	args := []string{subcommand, "--format", "json"}
	outcome := c.runCommand(ctx, args, BuildEnv(c.APIKey, c.Endpoint, nil), "", nil)
	if sdkErr := classifyAdmin(subcommand, outcome); sdkErr != nil {
		return nil, sdkErr
	}
	return outcome.stdout, nil
}

func classifyAdmin(subcommand string, outcome commandOutcome) *Error {
	if outcome.exitCode < 0 && outcome.err != nil {
		return transportError("run cursor-agent "+subcommand, outcome.err)
	}
	if outcome.exitCode != 0 {
		kind := classifyKind(nil, outcome.stderr, outcome.exitCode)
		return &Error{
			Kind:     kind,
			Message:  firstLine(outcome.stderr),
			ExitCode: outcome.exitCode,
			Stderr:   outcome.stderr,
		}
	}
	return nil
}
