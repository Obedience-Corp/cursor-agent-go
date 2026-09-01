package cursor

import (
	"context"
	"os/exec"
)

// LoginCommand returns an exec.Cmd for "cursor-agent login". The caller owns
// stdio so the flow can run on a TTY. The process still receives
// NO_OPEN_BROWSER=1 from BuildEnv; set that env to empty in Cmd.Env only if a
// human-driven browser login is required.
func (c *Client) LoginCommand(ctx context.Context) *exec.Cmd {
	cmd := c.command(ctx, "login")
	cmd.Env = c.envWith(BuildEnv(c.APIKey, c.Endpoint, nil))
	return cmd
}
