package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"
)

// Usage is the token accounting print mode reports for a turn. The CLI emits
// real counters, so these are never estimated from cost.
type Usage struct {
	InputTokens      int `json:"inputTokens"`
	OutputTokens     int `json:"outputTokens"`
	CacheReadTokens  int `json:"cacheReadTokens"`
	CacheWriteTokens int `json:"cacheWriteTokens"`
}

// AskResult is the JSON object print mode writes on stdout.
type AskResult struct {
	Type          string          `json:"type"`
	Subtype       string          `json:"subtype"`
	IsError       bool            `json:"is_error"`
	DurationMs    int             `json:"duration_ms"`
	DurationAPIMs int             `json:"duration_api_ms"`
	Result        string          `json:"result"`
	SessionID     string          `json:"session_id"`
	RequestID     string          `json:"request_id,omitempty"`
	Usage         Usage           `json:"usage"`
	Raw           json.RawMessage `json:"-"`
	Stderr        string          `json:"-"`
	ExitCode      int             `json:"-"`
}

// Ask runs one print-json turn with a background context.
func (c *Client) Ask(prompt string, opts *AskOptions) (*AskResult, error) {
	return c.AskCtx(context.Background(), prompt, opts)
}

// AskCtx runs one "cursor-agent -p --output-format json" turn.
func (c *Client) AskCtx(ctx context.Context, prompt string, opts *AskOptions) (*AskResult, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, validationError("prompt must not be empty")
	}
	result, err := c.ask(ctx, prompt, opts)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (c *Client) ask(ctx context.Context, prompt string, opts *AskOptions) (*AskResult, *Error) {
	prepared, prepErr := c.prepareAsk(opts)
	if prepErr != nil {
		return nil, prepErr
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, &Error{Kind: KindInterrupted, Message: "context done before cursor-agent ask", Original: ctxErr}
	}
	dir, dirErr := c.workDir(prepared.WorkingDirectory)
	if dirErr != nil {
		return nil, dirErr
	}
	args := BuildPrintArgs(prompt, prepared)
	env := BuildEnv(c.APIKey, c.Endpoint, prepared.Env)
	return c.askOnce(ctx, args, env, dir, prepared.Timeout)
}

func (c *Client) askOnce(ctx context.Context, args, env []string, dir string, timeout time.Duration) (*AskResult, *Error) {
	runCtx, cancel := contextWithTimeout(ctx, timeout)
	defer cancel()
	outcome := c.runCommand(runCtx, args, env, dir, nil)
	if ctxErr := runCtx.Err(); ctxErr != nil {
		return nil, &Error{Kind: KindInterrupted, Message: "cursor-agent ask canceled", ExitCode: outcome.exitCode, Stderr: outcome.stderr, Original: ctxErr}
	}
	if outcome.exitCode < 0 && outcome.err != nil {
		return nil, transportError("run cursor-agent ask", outcome.err)
	}
	result, parseErr := parseAskResult(outcome.stdout)
	if parseErr != nil {
		// Empty stdout plus a failed exit means the CLI refused the run before
		// it produced any JSON, most often over a bad flag. Its complaint is on
		// stderr, so report that rather than the generic "no output" parse
		// failure, which names a symptom and hides the cause.
		if len(bytes.TrimSpace(outcome.stdout)) == 0 && (outcome.exitCode != 0 || outcome.err != nil) {
			return nil, Classify(nil, outcome.stderr, outcome.exitCode, outcome.err)
		}
		parseErr.ExitCode = outcome.exitCode
		parseErr.Stderr = outcome.stderr
		return nil, parseErr
	}
	result.Stderr = outcome.stderr
	result.ExitCode = outcome.exitCode
	return result, Classify(result, outcome.stderr, outcome.exitCode, outcome.err)
}

func parseAskResult(stdout []byte) (*AskResult, *Error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return nil, validationError("cursor-agent ask produced no output on stdout")
	}
	var result AskResult
	if err := json.Unmarshal(trimmed, &result); err != nil {
		if isWorkspaceTrustPrompt(string(trimmed)) {
			return nil, workspaceUntrustedError(truncate(trimmed, 400))
		}
		return nil, validationErrorWith("cursor-agent ask stdout is not JSON: "+truncate(trimmed, 400), err)
	}
	result.Raw = append(json.RawMessage(nil), trimmed...)
	return &result, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
