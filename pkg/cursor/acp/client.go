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

	// done is closed once the child has exited and waitErr has been set.
	// Reading waitErr is only safe after done is closed; the close is the
	// happens-before edge.
	done    chan struct{}
	waitErr error
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
	c := &Client{
		cmd:   cmd,
		stdin: stdin,
		conn:  newConn(stdin, stdout, handler),
		done:  make(chan struct{}),
	}
	// One waiter owns cmd.Wait, because calling it twice is an error. It is
	// also what lets a caller observe an exit it did not ask for: the process
	// can die on its own, and without this the first sign of that is a failed
	// request.
	go func() {
		c.waitErr = c.cmd.Wait()
		close(c.done)
	}()
	return c, nil
}

// Done is closed when the cursor-agent process has exited, whether the caller
// asked for that or not. Use it to notice a child that died on its own, rather
// than discovering it on the next request.
func (c *Client) Done() <-chan struct{} { return c.done }

// PID is the process id of the running child, or zero once it has exited or if
// it never started. A caller reporting a live process id should treat zero as
// "no process", not as an unknown pid.
func (c *Client) PID() int {
	select {
	case <-c.done:
		return 0
	default:
	}
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

// ExitErr reports how the process ended. It blocks until the child has exited,
// so callers that only want to poll should select on Done first. A nil result
// means a clean exit.
func (c *Client) ExitErr() error {
	<-c.done
	return c.waitErr
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

// Close shuts the session down and waits for the process to exit. The child
// does not exit zero when its stdin closes, and a context cancel kills it
// outright, so a non-nil error here is usually just how it ended rather than a
// failure to shut down.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		_ = c.stdin.Close()
		c.conn.shutdown(ErrClosed)
		<-c.done
		c.closeErr = c.waitErr
	})
	return c.closeErr
}
