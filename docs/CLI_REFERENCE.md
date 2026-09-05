# CLI reference

Compatibility target: Cursor Agent CLI `2026.08.25-3e8eec8`
(`cursor.TestedAgentVersion`).

The SDK wraps `cursor-agent`, not the `cursor` editor launcher. The CLI still
prints `Usage: agent ...`; that is the same binary.

## Locate

| Source | Notes |
| --- | --- |
| `CURSOR_AGENT_BIN` | Explicit path. Must be an executable file. |
| `cursor-agent` on PATH | Preferred name |
| `~/.local/bin/cursor-agent`, Homebrew, `/usr/local/bin`, `/usr/bin` | Install locations |
| `agent` | Used only when the file resolves to cursor-agent |

## Print mode (Ask)

```
cursor-agent -p --output-format json --model <id> -- <prompt>
```

| Flag | SDK field |
| --- | --- |
| `-p` / `--print` | always set |
| `--output-format json\|text\|stream-json` | `AskOptions.OutputFormat` (default `json`) |
| `--stream-partial-output` | `StreamPartial` (requires `stream-json`) |
| `--model` | `Model` |
| `--mode plan\|ask` | `Mode` (`ModeAgent` and `ModeUnset` both render as no flag) |
| `--force` / `--yolo` | `Force` / `Yolo` (requires `AllowDangerousMode`) |
| `--auto-review` | `AutoReview` (opt-in only) |
| `--sandbox enabled\|disabled` | `Sandbox` |
| `--approve-mcps` | `ApproveMCPs` |
| `--trust` | `Trust` |
| `--workspace` | `Workspace` |
| `--add-dir` | `AddDirs` |
| `--resume` / `--continue` | `Resume` / `Continue` |
| `-H` | `Headers` |

Modes: `--mode` accepts only `plan` and `ask`. `--mode agent` is rejected with
exit code 1 and `Allowed choices are plan, ask`, on the `acp` subcommand as well
as in print mode, even though the ACP `session/new` reply advertises an agent
mode. Agent is the CLI default, so `ModeAgent` is an absent flag rather than a
value.

Auth: `--api-key` is not passed on argv. Set `Client.APIKey` or `CURSOR_API_KEY`.

Spawned env always includes `NO_OPEN_BROWSER=1`.

## ACP

```
cursor-agent acp
```

Session types and permission round-trips are landing after a live capture.
`BuildACPArgs` is present so callers can inspect argv today.

## Cloud

HTTP and SSE against `https://api.cursor.com/v1/agents`. Package
`pkg/cursor/cloud` is landing with v0.1.
