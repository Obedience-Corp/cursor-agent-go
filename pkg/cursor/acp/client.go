package acp

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Options configures an ACP client.
type Options struct {
	// BinPath is the cursor-agent binary. Empty means "cursor-agent" on PATH.
	BinPath string
	// WorkingDirectory is the agent's cwd and the default session cwd.
	WorkingDirectory string
	// Args are extra flags appended after the acp subcommand.
	Args []string
	// Env is added to the parent environment, which the child always
	// inherits; it does not replace it. NO_OPEN_BROWSER=1 is appended last
	// and always wins.
	Env []string
	// Handler receives updates and permission requests. Nil means DenyAll.
	Handler Handler
	// Stderr receives the child's stderr. Nil discards it.
	Stderr io.Writer
}

// Client is a long-lived "cursor-agent acp" process.
type Client struct {
	cmd  *exec.Cmd
	conn *conn

	closeOnce sync.Once
	closeErr  error

	stdin io.WriteCloser
}

// Start spawns cursor-agent acp and returns a client ready for Initialize.
func Start(ctx context.Context, opts Options) (*Client, error) {
	bin := opts.BinPath
	if strings.TrimSpace(bin) == "" {
		bin = "cursor-agent"
	}
	args := append([]string{"acp"}, opts.Args...)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = opts.WorkingDirectory
	cmd.Env = childEnv(opts.Env)
	cmd.Stderr = opts.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	handler := opts.Handler
	if handler == nil {
		handler = DenyAll
	}
	return &Client{cmd: cmd, stdin: stdin, conn: newConn(stdin, stdout, handler)}, nil
}

func childEnv(extra []string) []string {
	env := os.Environ()
	if len(extra) > 0 {
		env = append(env, extra...)
	}
	return append(env, "NO_OPEN_BROWSER=1")
}

// Initialize performs the protocol handshake.
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	params := map[string]any{
		"protocolVersion":    ProtocolVersion,
		"clientCapabilities": ClientCapabilities{},
	}
	var out InitializeResult
	if err := c.conn.call(ctx, "initialize", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// NewSession opens a session rooted at cwd. Empty cwd uses the client's
// working directory.
func (c *Client) NewSession(ctx context.Context, cwd string) (*NewSessionResult, error) {
	if strings.TrimSpace(cwd) == "" {
		cwd = c.cmd.Dir
	}
	params := map[string]any{"cwd": cwd, "mcpServers": []any{}}
	var out NewSessionResult
	if err := c.conn.call(ctx, "session/new", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Prompt sends one turn and blocks until the agent stops. Updates arrive on
// the Handler while this call is in flight.
func (c *Client) Prompt(ctx context.Context, sessionID string, blocks ...ContentBlock) (*PromptResult, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("acp: sessionID must not be empty")
	}
	if len(blocks) == 0 {
		return nil, errors.New("acp: prompt must not be empty")
	}
	params := map[string]any{"sessionId": sessionID, "prompt": blocks}
	var out PromptResult
	if err := c.conn.call(ctx, "session/prompt", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Ask is Prompt with a single text block.
func (c *Client) Ask(ctx context.Context, sessionID, text string) (*PromptResult, error) {
	return c.Prompt(ctx, sessionID, TextBlock(text))
}

// Cancel asks the agent to stop the in-flight turn. The pending Prompt then
// returns with StopCancelled.
func (c *Client) Cancel(sessionID string) error {
	return c.conn.notify("session/cancel", map[string]any{"sessionId": sessionID})
}

// Close shuts the session down and waits for the process to exit.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		_ = c.stdin.Close()
		c.conn.shutdown(ErrClosed)
		c.closeErr = c.cmd.Wait()
	})
	return c.closeErr
}
