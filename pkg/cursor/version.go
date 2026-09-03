package cursor

import (
	"context"
	"strings"
)

// TestedAgentVersion is the Cursor Agent CLI release covered by the SDK's
// fixtures and compatibility lane.
const TestedAgentVersion = "2026.08.31-4057e58"

// Version runs "cursor-agent --version" and returns the trimmed stdout.
func (c *Client) Version(ctx context.Context) (string, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", &Error{Kind: KindInterrupted, Message: "context done before version probe", Original: ctxErr}
	}
	dir, dirErr := c.workDir("")
	if dirErr != nil {
		return "", dirErr
	}
	runCtx, cancel := contextWithTimeout(ctx, 0)
	defer cancel()
	outcome := c.runCommand(runCtx, []string{"--version"}, BuildEnv(c.APIKey, c.Endpoint, nil), dir, nil)
	if ctxErr := runCtx.Err(); ctxErr != nil {
		return "", &Error{Kind: KindInterrupted, Message: "version probe canceled", ExitCode: outcome.exitCode, Stderr: outcome.stderr, Original: ctxErr}
	}
	if outcome.exitCode < 0 && outcome.err != nil {
		return "", transportError("run cursor-agent --version", outcome.err)
	}
	out := strings.TrimSpace(string(outcome.stdout))
	if err := Classify(nil, outcome.stderr, outcome.exitCode, outcome.err); err != nil {
		return out, err
	}
	return out, nil
}
