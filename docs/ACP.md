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
waits for the process. It is idempotent and asserted leak-free. The child does
not exit zero when its stdin closes, and a context cancel kills it outright, so
a non-nil error from `Close` is usually just how it ended.

## Process identity

The client owns a real child process, and a host that manages sessions needs to
see it:

```go
pid := client.PID()        // live pid, or 0 once it has exited

select {
case <-client.Done():      // closed when the child exits, asked for or not
    log.Printf("agent exited: %v", client.ExitErr())
default:
}
```

`Done` is the part worth wiring up. A `cursor-agent` process can die on its own,
and without watching it the first sign is a request failing with `ErrClosed`,
one turn later than the process actually went away. `PID` returns 0 rather than
a stale pid after exit, so zero always means "no process" and never "unknown".
`ExitErr` blocks until the child has exited and reports the same error `Close`
returns.

Note for anyone writing a test against a real child: `Start` always passes `acp`
as the first argument, so `BinPath: "cat"` runs `cat acp`, reads a file that
does not exist, and exits at once. A process that has to stay alive needs a
binary that ignores its arguments.

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

Aggregation is scoped to the session id, and the concurrency contract follows
from what the wire actually carries:

| Case | Behaviour |
| --- | --- |
| Different sessions, same connection | Run concurrently, fully isolated |
| Same session, overlapping | Second call returns `ErrCollectionInProgress` |

The refusal is not a limitation of this implementation, it is forced by the
protocol. Across every captured `session/update` frame the only correlation
field is `sessionId`:

```
params keys: {"sessionId", "update"}
update keys: {"sessionUpdate", "content", "toolCallId", "title",
              "kind", "status", "rawInput", "rawOutput", "locations",
              "availableCommands"}
```

There is no turn id, prompt id, or request id anywhere in the payload, so the
updates of two overlapping turns on one session are indistinguishable. Handing
back a silently mixed transcript would be worse than refusing, so collection is
serialized per session and the second caller is told why.

The client's own `Handler` keeps receiving every update throughout, and one
session finishing never detaches another's collector.

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
