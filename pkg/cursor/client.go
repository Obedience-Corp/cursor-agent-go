package cursor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"time"
)

var execCommand = exec.CommandContext

// Client runs the cursor-agent binary. Calls are safe for concurrent use after
// the exported configuration fields have been set and are no longer mutated.
type Client struct {
	BinPath        string
	WorkingDir     string
	APIKey         string
	Endpoint       string
	Env            []string
	DefaultOptions *AskOptions
}

// NewClient returns a client bound to an explicit cursor-agent binary path.
func NewClient(binPath string) *Client {
	return &Client{BinPath: binPath, DefaultOptions: &AskOptions{}}
}

// NewClientFromPath locates cursor-agent and returns a client for it.
func NewClientFromPath() (*Client, error) {
	p, err := LocateBinary()
	if err != nil {
		return nil, err
	}
	return NewClient(p), nil
}

func (c *Client) command(ctx context.Context, args ...string) *exec.Cmd {
	return execCommand(ctx, c.BinPath, args...)
}

func (c *Client) workDir(override string) (string, *Error) {
	if override != "" {
		return override, nil
	}
	if c.WorkingDir != "" {
		return c.WorkingDir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", transportError("resolve working directory", err)
	}
	return cwd, nil
}

func (c *Client) envWith(managed []string) []string {
	base := append([]string(nil), os.Environ()...)
	base = append(base, c.Env...)
	base = stripEnvKeys(base, envKeys(managed)...)
	return append(base, managed...)
}

func (c *Client) prepareAsk(opts *AskOptions) (*AskOptions, *Error) {
	if opts == nil {
		opts = c.DefaultOptions
	}
	prepared := cloneAskOptions(opts)
	if err := prepared.validate(); err != nil {
		return nil, err
	}
	return prepared, nil
}

type commandOutcome struct {
	stdout   []byte
	stderr   string
	exitCode int
	err      error
}

func (c *Client) runCommand(ctx context.Context, args, managedEnv []string, dir string, stdin *bytes.Reader) commandOutcome {
	cmd := c.command(ctx, args...)
	cmd.Dir = dir
	cmd.Env = c.envWith(managedEnv)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := commandOutcome{stdout: stdout.Bytes(), stderr: stderr.String(), err: err}
	if err == nil {
		return out
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		out.exitCode = exitErr.ExitCode()
		return out
	}
	out.exitCode = -1
	return out
}

func contextWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}
