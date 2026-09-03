<h1 align="center">cursor-agent-go</h1>

<p align="center">
  <strong>Go SDK for the Cursor Agent CLI</strong>
</p>

<p align="center">
  <img src="docs/images/hero.png" alt="A CRT on a drafting table, wrapped in brass traces: a Go program holding a CLI." width="100%">
</p>

<p align="center">
  <a href="https://github.com/Obedience-Corp/cursor-agent-go/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/Obedience-Corp/cursor-agent-go/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://pkg.go.dev/github.com/Obedience-Corp/cursor-agent-go/pkg/cursor"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/Obedience-Corp/cursor-agent-go/pkg/cursor.svg"></a>
  <img alt="License: Apache-2.0" src="https://img.shields.io/badge/license-Apache--2.0-blue.svg">
  <img alt="Go 1.24+" src="https://img.shields.io/badge/go-1.24%2B-00ADD8.svg">
</p>

Wraps the installed **`cursor-agent`** binary the same way
[`grok-go-sdk`](https://github.com/lancekrogers/grok-go-sdk) wraps `grok` and
[`vercel-fx-go`](https://github.com/Obedience-Corp/vercel-fx-go) wraps `fx`.

| Runtime | Transport | Status |
| --- | --- | --- |
| Local one-shot | `cursor-agent -p --output-format json` | Ready |
| Local streaming | `cursor-agent acp` | Landing |
| Cloud agents | HTTP + SSE on `api.cursor.com` | Landing |

It does **not** wrap the `cursor` editor, a generic `agent` command, or the
official Node/Python SDKs. `agent` collides with other tools; locate prefers
`cursor-agent` and only accepts `agent` when the file is actually Cursor's CLI.

Standard library only. Auth is `Client.APIKey` or `CURSOR_API_KEY`. Every
spawned process gets `NO_OPEN_BROWSER=1`.

Compatibility target: `cursor.TestedAgentVersion` (`2026.08.25-3e8eec8`). The
API is unstable until v1.0.

This is an independent Obedience Corp project. It is not affiliated with or
endorsed by Anysphere / Cursor.

## Requirements

- Go 1.24 or newer
- [Cursor Agent CLI](https://cursor.com/docs/cli/overview) installed as
  `cursor-agent`
- `CURSOR_API_KEY`, or `cursor-agent login` for interactive use

```bash
cursor-agent --version
export CURSOR_API_KEY="cursor_..."
```

## Install

```bash
go get github.com/Obedience-Corp/cursor-agent-go@latest
```

Private checkout: `GOPRIVATE=github.com/Obedience-Corp/*`.

## Quick start

```go
package main

import (
	"fmt"
	"log"

	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor"
)

func main() {
	client, err := cursor.NewClientFromPath()
	if err != nil {
		log.Fatal(err)
	}

	result, err := client.Ask("Summarize README.md in one sentence.", &cursor.AskOptions{
		Model: "composer-2.5",
		Trust: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Result)
}
```

`Ask`/`AskCtx` always pass `-p --output-format json`. Force/yolo is rejected
unless you go through `pkg/cursor/dangerous`.

`AskResult.Usage` carries the CLI's real token counters (`inputTokens`,
`outputTokens`, `cacheReadTokens`, `cacheWriteTokens`); they are read off the
wire, never estimated from cost.

```bash
just build ask
./bin/ask "What is in go.mod?"
```

## Long-lived sessions (ACP)

`pkg/cursor/acp` drives `cursor-agent acp` over newline-delimited JSON-RPC for
a session that survives many turns, streams incrementally, and can be canceled
mid-turn.

```go
client, err := acp.Start(ctx, acp.Options{
	WorkingDirectory: ".",
	Handler: acp.HandlerFuncs{
		Update: func(_ context.Context, _ string, u acp.Update) {
			if u.SessionUpdate == acp.UpdateAgentMessageChunk && u.Content != nil {
				fmt.Print(u.Content.Text)
			}
		},
		Permission: func(_ context.Context, r acp.PermissionRequest) acp.PermissionOutcome {
			return acp.AllowPermission(r.Options[0].OptionID)
		},
	},
})
defer client.Close()

client.Initialize(ctx)
session, _ := client.NewSession(ctx, "")
result, _ := client.Ask(ctx, session.SessionID, "What changed in the last commit?")
fmt.Println(result.StopReason) // end_turn

// From another goroutine, to stop the turn early:
client.Cancel(session.SessionID) // the pending Ask returns StopCancelled
```

Text arrives as many `agent_message_chunk` updates, so concatenate them.
Unmodeled update variants are still delivered, with the original payload on
`Update.Raw`.

`CollectAsk` aggregates a whole turn for you, merging `tool_call` frames with
the later `tool_call_update` frames that actually carry the arguments:

```go
tr, _ := client.CollectAsk(ctx, session.SessionID, "Add a test for Parse")
fmt.Println(tr.Text, tr.StopReason, len(tr.ToolCalls))
```

**Do not treat `StopReason` or the reply text as proof the work happened.** In
live testing an edit tool call never reached a terminal status (the last update
read `in_progress` even on success), and one run in three returned `end_turn`
with the text `DONE` while the file it claimed to write did not exist. Check
the side effect yourself.

**The permission callback is not a sandbox.** `cursor-agent 2026.08.31` never
sent `session/request_permission` in default configuration during testing. It
edited files, ran shell commands, and wrote a file outside the directory passed
as the session `cwd`, all without asking. The handler is implemented for spec
compliance and future CLI versions; confine the agent with OS-level isolation
rather than relying on it.

## Cloud agents

`pkg/cursor/cloud` targets the Cloud Agents v1 API. An agent is durable; each
prompt creates a run, and streaming and cancellation are scoped to a run.

```go
c := cloud.New(os.Getenv("CURSOR_API_KEY"))
created, err := c.CreateAgent(ctx, cloud.CreateAgentRequest{
	Prompt: cloud.Prompt{Text: "Add a README with setup instructions"},
})

stream, err := c.StreamRun(ctx, created.Agent.ID, created.Run.ID, nil)
defer stream.Close()
for {
	event, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		break
	}
	// event.Type is one of cloud.EventStatus, EventAssistant, EventResult, ...
}
```

Resume a dropped stream with `StreamOptions{LastEventID: stream.LastEventID()}`.
Non-2xx responses come back as `*cloud.APIError` with `IsRetryable`,
`IsNotFound`, and `IsUnauthorized`.

## Locate

`NewClientFromPath` calls `LocateBinary`:

1. `CURSOR_AGENT_BIN`
2. `cursor-agent` on `PATH`
3. `~/.local/bin/cursor-agent` and the usual Homebrew/usr paths
4. `agent` only if it resolves to cursor-agent (symlink or install tree)

```go
client := cursor.NewClient("/usr/local/bin/cursor-agent")
```

## Admin

```go
about, _ := client.About(ctx)   // cliVersion, latestStatus, tier, os
status, _ := client.Status(ctx) // isAuthenticated, token flags, userInfo
models, _ := client.Models(ctx) // parsed from the text listing
```

`About` and `Status` use the CLI's `--format json`. `Models` has no JSON mode,
so it parses the `id - Name` listing and skips the header. `Status.UserInfo`
carries an email, a user id, and a name; treat it as personal data.

## Errors

A thrown `*cursor.Error` with `KindAuth` or `KindTransport` means the run never
executed correctly. `KindWorkspaceUntrusted` means the CLI refused the
directory: on a first run in an untrusted workspace it prints a human-readable
trust prompt instead of JSON, so set `AskOptions.Trust`. `result.IsError` plus a classified process error means the
CLI ran and failed. Check `errors.As` and `err.IsRetryable()`.

## Dangerous mode

`--force` / `--yolo` live behind a second gate:

```go
import "github.com/Obedience-Corp/cursor-agent-go/pkg/cursor/dangerous"

// CURSOR_GO_ENABLE_DANGEROUS=i-accept-all-risks, and not GO_ENV/NODE_ENV=production
guarded, err := dangerous.Wrap(client)
result, err := guarded.Force(ctx, prompt, nil)
```

Use it only in a disposable workspace. The production check is best-effort.

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): process model and package layout
- [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md): flags the SDK emits
- [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md): gates, mock binary, hero image

## Development

```bash
just lint
just test all
just test race
just build all
just docs hero-check
```

`just docs hero` regenerates `docs/images/hero.png` with the grok CLI Imagine
tool (same recipe as the technical videos campaign).

## License

Apache-2.0. See [LICENSE](LICENSE).
