# Changelog

All notable changes to this project are documented here. The project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) before v1.0, so minor
releases may include API changes.

## [Unreleased]

### Added

- `acp.Client` exposes the child process: `PID` returns the live pid or zero
  once it has exited, `Done` is closed when the child exits whether the caller
  asked for it or not, and `ExitErr` reports how it ended. Without `Done` the
  first sign of a process that died on its own was a request failing with
  `ErrClosed`, a turn later than the exit.

### Changed

- `cmd.Wait` is now owned by a single waiter goroutine started with the
  process, rather than called from `Close`. `Close` returns the same error as
  before and stays idempotent.

## [0.1.0] - 2026-09-05

First tagged release. The local CLI compatibility target is
`TestedAgentVersion`; behavior notes below were captured against
`cursor-agent 2026.08.31-4057e58` and `2026.09.02-c22c1a3`.

### Added

#### Local print mode

- `Client` over the installed `cursor-agent` binary: `Ask`, `AskCtx`,
  `Version`, and a `LoginCommand` builder that leaves stdio to the caller.
- `LocateBinary` prefers `cursor-agent` and accepts a bare `agent` only when it
  resolves back to cursor-agent, so an unrelated `agent` on PATH is ignored.
- `AskResult.Usage` carries the real counters the CLI reports: `inputTokens`,
  `outputTokens`, `cacheReadTokens`, and `cacheWriteTokens`. They are never
  estimated from cost.
- Admin wrappers `About`, `Status`, and `Models`. `Status.UserInfo` holds
  personal data and should not be logged or captured into fixtures.
- Guarded `pkg/cursor/dangerous` force and yolo entry points, which refuse
  unless `CURSOR_GO_ENABLE_DANGEROUS=i-accept-all-risks` and refuse outright
  when `GO_ENV` or `NODE_ENV` is `production`.

#### ACP session client (`pkg/cursor/acp`)

- Long-lived `cursor-agent acp` process speaking newline-delimited JSON-RPC 2.0
  over stdio: `Start`, `Initialize`, `NewSession`, `Prompt`, `Ask`, `Cancel`,
  and `Close`.
- `CollectPrompt` and `CollectAsk` aggregate one turn into a `Transcript`,
  merging every frame for a tool call id rather than keeping only the first.
- `Handler`, `HandlerFuncs`, `DenyAll`, and `PolicyHandler` for updates and
  permission requests, with `PermissionPolicy` matching on option kind rather
  than agent-defined ids.
- Aggregation is scoped per session. Concurrent collections on different
  sessions are isolated; a second collection on the same session returns
  `ErrCollectionInProgress`, because `session/update` carries no turn
  correlation and two overlapping turns are indistinguishable on the wire.

#### Cloud Agents client (`pkg/cursor/cloud`)

- Agent and run lifecycle against `api.cursor.com`: create, fetch, list,
  delete, archive, and cancel, plus `WaitRun`, `AgentUsage`, `ListModels`,
  `ListArtifacts`, and `DownloadArtifact`.
- SSE streaming with `Last-Event-ID` resume, tolerating comment lines,
  multi-line data, and events without an id.
- `APIError` with `IsRetryable`, `IsNotFound`, `IsUnauthorized`, `IsBusy`,
  `IsStreamExpired`, and `IsInvalidLastEventID`.

#### Testing and docs

- Mock agent binary with print, admin, and ACP scenarios; `mockcloud` httptest
  server; integration lanes behind the `integration` build tag, with real-agent
  lanes gated on `CURSOR_INTEGRATION_REAL=1`.
- `docs/ACP.md`, `docs/CLOUD.md`, `docs/CLI_REFERENCE.md`, and runnable
  examples for ask, ACP streaming, cloud create, and models.

### Fixed

- `--mode agent` is no longer emitted. The CLI accepts only `plan` and `ask`
  and exits 1 with `Allowed choices are plan, ask`, on the `acp` subcommand as
  well as in print mode, even though the ACP `session/new` reply advertises an
  agent mode. Agent is the CLI default, so `ModeAgent` and `ModeUnset` both
  render as an absent flag.
- A run the CLI refuses now reports the CLI's own message. A rejected flag
  writes nothing to stdout and puts the reason on stderr, which was previously
  reported as `cursor-agent ask produced no output on stdout` while the real
  cause sat in a field `Error()` never prints. Empty stdout with a failed exit
  now classifies from stderr; empty stdout with a clean exit is still reported
  as missing output.
- The workspace trust gate is classified as `KindWorkspaceUntrusted` instead of
  failing as a JSON parse error. The CLI prints that gate as human text on
  stdout even under `--output-format json`.
- `Close` no longer leaks the read loop or the collector registration.

### Changed

- README leads with cursor-agent itself. Sibling Go SDKs, including
  claude-code-go, are listed at the end.

### Known behavior of the CLI

These are properties of `cursor-agent`, not of this SDK. They are recorded here
because they change how a caller must be written.

- **The ACP permission callback is not a containment boundary.**
  `session/request_permission` was never observed being sent. Probed four ways,
  the agent auto-approved every time: editing inside the session `cwd`, running
  a shell command, writing outside the session `cwd`, and writing outside the
  session `cwd` with `--sandbox enabled`. The session `cwd` is a starting
  directory and `--sandbox enabled` does not make it a boundary. Confine the
  agent with OS-level isolation.
- **`stopReason` and tool status are not evidence that work happened.** Across
  three identical live runs an edit tool call never reached a terminal status,
  and one run resolved `end_turn` with the reply `DONE.` and no file on disk.
  Verify side effects instead.
- **A tool call is announced before its arguments exist.** The first
  `tool_call` frame carries `status: pending` and an empty `rawInput`; the
  arguments and locations arrive on later `tool_call_update` frames sharing the
  id.
- **A `toolCallId` can contain a literal newline.** Anything treating it as a
  single-line token will corrupt it.
- **The ACP transport reports no token usage.** `session/prompt` returns only a
  `stopReason`. Print mode is the transport that reports counters.
