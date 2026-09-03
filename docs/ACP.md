# ACP sessions

`pkg/cursor/acp` drives `cursor-agent acp`: a long-lived local session that
survives many turns, streams incrementally, and can be cancelled mid-turn.

Everything here was verified against `cursor-agent 2026.08.31-4057e58` and
re-checked on `2026.09.02-c22c1a3`. Raw transcripts live in the design package
under `evidence/acp/`.

## Transport

Newline-delimited JSON-RPC 2.0 over the child process's stdin and stdout. Three
kinds of inbound message are demultiplexed:

| Inbound | Handling |
| --- | --- |
| response with `id` | resolves the waiting `call` |
| notification (`session/update`) | delivered to `Handler.OnUpdate` |
| request with `id` (`session/request_permission`) | answered via `Handler.OnPermission`; unknown methods get `-32601` |

## Lifecycle

```go
client, err := acp.Start(ctx, acp.Options{
    WorkingDirectory: ".",
    Handler:          acp.PolicyHandler(acp.PolicyAllowOnce, onUpdate),
})
defer client.Close()

init, _ := client.Initialize(ctx)          // protocolVersion, authMethods
session, _ := client.NewSession(ctx, "")   // sessionId, modes, models
result, _ := client.Ask(ctx, session.SessionID, "...")
```

`Close` shuts the stream down, fails every in-flight call with `ErrClosed`, and
waits for the process. It is idempotent and asserted leak-free.

## Update variants

`session/update` carries a `sessionUpdate` discriminator:

| Variant | Payload |
| --- | --- |
| `agent_message_chunk` | `content` (reply text, streamed in many pieces) |
| `agent_thought_chunk` | `content` (reasoning) |
| `tool_call` | `toolCallId`, `title`, `kind`, `status`, `rawInput` |
| `tool_call_update` | `toolCallId`, `title`, `status`, `rawInput`, `locations` |
| `session_info_update` | `title` |
| `available_commands_update` | `availableCommands` |

Unmodelled variants are still delivered, with the original bytes on
`Update.Raw`, so a CLI update does not silently drop events.

## Aggregating a turn

`CollectAsk` and `CollectPrompt` return a `Transcript` with assembled text,
thoughts, title, merged tool calls, and the stop reason:

```go
tr, _ := client.CollectAsk(ctx, session.SessionID, "Add a test for Parse")
```

Merging is not cosmetic. The CLI announces a call with `status: pending` and an
**empty** `rawInput`, then sends the real arguments and `locations` on a later
`tool_call_update` sharing the id. Keeping only the first frame loses the
arguments entirely.

## Cancellation

`Cancel` is a notification, so it does not block. The pending `Prompt` resolves
with `StopCancelled`:

```
-> sending session/cancel
stopReason: cancelled  after: 6.0s
```

## Permission

`PermissionPolicy.Decide` maps a policy onto whatever options the agent offered,
matching the protocol-defined `kind` first and the option id second. An allow
policy that finds no allow option cancels rather than selecting a reject option.

**Do not treat this as a sandbox.** In testing the CLI never sent
`session/request_permission` at all. It edited files, ran shell commands, and
wrote a file **outside** the directory passed as the session `cwd`, without
asking in any case. The session `cwd` is a starting directory, not a boundary.
Confine the agent with OS-level isolation.

The `permission` scenario in `test/mockagent` exists precisely because the real
CLI never exercises this path.

## Completion is not reported

Across three identical live runs asking the agent to create a file:

| Run | stopReason | Text | Last edit status | File created |
| --- | --- | --- | --- | --- |
| 1 | `end_turn` | `DONE.` | `in_progress` | no |
| 2 | `end_turn` | `DONE` | `in_progress` | yes |
| 3 | `end_turn` | `DONE` | `in_progress` | yes |

Edit tool calls never reach a terminal status, so status cannot signal
completion, and a clean `end_turn` with a confident reply does not prove the
work happened. Verify side effects.

## Traps

1. `toolCallId` can contain a **literal newline**. Do not use it as a key in a
   line-oriented format or log it unquoted.
2. Reply text arrives as many chunks; concatenate rather than taking the last.
3. The read loop is serialized, so a slow `Handler` stalls the whole stream.
