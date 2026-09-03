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

Go client for the [Cursor Agent CLI](https://cursor.com/docs/cli/overview).
It wraps the installed **`cursor-agent`** binary.

| Runtime | Transport | Status |
| --- | --- | --- |
| Local one-shot | `cursor-agent -p --output-format json` | Ready |
| Local streaming | `cursor-agent acp` | Landing |
| Cloud agents | HTTP + SSE on `api.cursor.com` | Landing |

This is not a wrapper for the `cursor` editor or Cursor's Node/Python SDKs.
Locate looks for `cursor-agent` first. It only accepts a command named `agent`
when that file is actually Cursor's CLI.

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
the later `tool_call_update` frames that actually carry the arguments.
Collections on different sessions run concurrently and stay isolated; two
overlapping collections on the *same* session return
`acp.ErrCollectionInProgress`, because `session/update` carries no turn
correlation and their updates cannot be told apart:

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
If you only need the final state, `WaitRun` polls to a terminal status without
holding a connection open.

Non-2xx responses come back as `*cloud.APIError`. Alongside `IsRetryable`,
`IsNotFound`, and `IsUnauthorized`, three conditions have their own checks
because they are recoverable in specific ways: `IsBusy` (409 `agent_busy`, the
agent already has an active run), `IsStreamExpired` (410, the retention window
passed), and `IsInvalidLastEventID` (400, resume id does not belong to the run).

`ListArtifacts` and `DownloadArtifact` reach files the agent left under
`artifacts/`; download returns a presigned URL.

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

`*cursor.Error` with `KindAuth` or `KindTransport` means the run never started.
`result.IsError` plus a classified process error means the CLI ran and failed.
Check `errors.As` and `err.IsRetryable()`.

`KindWorkspaceUntrusted` means the CLI refused the directory: on a first run in
an untrusted workspace it prints a human-readable trust prompt instead of JSON,
so set `AskOptions.Trust`.

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
- [docs/ACP.md](docs/ACP.md): ACP transport, update variants, cancellation, and the permission and completion caveats
- [docs/CLOUD.md](docs/CLOUD.md): Cloud Agents model, streaming, resume, and error taxonomy
- [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md): gates, mock binary, hero image

## Examples

| Example | What it does |
| --- | --- |
| [examples/ask](examples/ask) | one-shot print-mode question |
| [examples/acp_stream](examples/acp_stream) | long-lived ACP session, streamed |
| [examples/cloud_create](examples/cloud_create) | create a cloud agent and follow its run over SSE |
| [examples/models](examples/models) | list models, CLI version, and auth state |

## Testing

```bash
just test all          # unit tests, all packages
just test race         # race detector
just test integration  # end to end against the mock binary and mock cloud server
just test integration-real  # same lanes against a real installed cursor-agent
```

`just test integration` needs no credentials and makes no network calls: it
builds `test/mockagent` (which impersonates print mode, the admin subcommands,
and a full ACP session) and starts `test/mockcloud` (an httptest Cloud Agents
API). `integration-real` sets `CURSOR_INTEGRATION_REAL=1`, spawns the installed
`cursor-agent`, and consumes account quota, so it is opt in.

The mock ACP scenarios are selected with `CURSOR_MOCK_ACP`: unset for a normal
turn, `permission` to exercise the permission round trip the real CLI never
sends, `cancel` for a cancelled stop reason, and `toolnewline` for the
newline-bearing tool call id seen in live capture.

## Development

```bash
just lint
just test all
just test race
just build all
```

## Related

Go SDKs for other coding-agent CLIs:

- [claude-code-go](https://github.com/lancekrogers/claude-code-go) wraps `claude`
- [grok-go-sdk](https://github.com/lancekrogers/grok-go-sdk) wraps `grok`
- [vercel-fx-go](https://github.com/Obedience-Corp/vercel-fx-go) wraps `fx`

## License

Apache-2.0. See [LICENSE](LICENSE).
