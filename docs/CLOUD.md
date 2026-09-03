# Cloud Agents

`pkg/cursor/cloud` targets the Cursor Cloud Agents v1 API at
`https://api.cursor.com`, built against the OpenAPI document snapshotted in the
design package as `evidence/cloud-agents-openapi.yaml`.

## Model

An **agent** is durable. Each prompt submission creates a **run** on it. Only
one run is active per agent at a time, and streaming and cancellation are scoped
to a run, not the agent.

```go
c := cloud.New(os.Getenv("CURSOR_API_KEY"))

created, _ := c.CreateAgent(ctx, cloud.CreateAgentRequest{
    Prompt: cloud.Prompt{Text: "Add a README with setup instructions"},
})
// created.Agent is durable, created.Run is the initial run
```

Auth is a bearer token. An empty `APIKey` is rejected before any request goes
out.

## Operations

| Area | Methods |
| --- | --- |
| Agents | `CreateAgent`, `Agent`, `ListAgents`, `DeleteAgent`, `ArchiveAgent`, `UnarchiveAgent` |
| Runs | `CreateRun`, `Run`, `ListRuns`, `CancelRun`, `WaitRun` |
| Streaming | `StreamRun` |
| Other | `AgentUsage`, `ListArtifacts`, `DownloadArtifact`, `ListModels` |

Listing takes `limit` and `cursor` and returns `NextCursor`.

## Following a run

Two options. `StreamRun` when the intermediate events matter:

```go
stream, _ := c.StreamRun(ctx, agentID, runID, nil)
defer stream.Close()
for {
    event, err := stream.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    switch event.Type {
    case cloud.EventAssistant: // incremental reply text
    case cloud.EventResult:    // terminal payload
    }
}
```

`WaitRun` when only the final state matters. It polls to a terminal status
instead of holding a connection open, stops immediately on a non-retryable
error, and honours context cancellation.

`IsTerminal` centralises the FINISHED / ERROR / CANCELLED / EXPIRED set.

## Stream event types

`status`, `assistant`, `thinking`, `tool_call`, `interaction_update`,
`heartbeat`, `result`, `error`, `done`.

`interaction_update` is the richer SDK-shaped event emitted alongside the
simplified ones sharing an id. Handle the simplified events and ignore it, or
handle it alone. Do not double-count.

## Resuming

Reconnect with the last event id:

```go
stream, err := c.StreamRun(ctx, agentID, runID,
    &cloud.StreamOptions{LastEventID: previous.LastEventID()})
```

`status` and `done` events carry **no** id, so they must not become the resume
point. `LastEventID()` only advances on events that actually have one.

The server reports its retention window in
`X-Cursor-Stream-Retention-Seconds`, surfaced as `RunStream.RetentionSeconds`.
Past that window the endpoint returns 410.

## Errors

Non-2xx responses are `*cloud.APIError` carrying status, code, message, body,
and a parsed `Retry-After`.

| Predicate | Condition | Recovery |
| --- | --- | --- |
| `IsRetryable` | 429 or 5xx | back off and retry |
| `IsNotFound` | 404 | the agent or run is gone |
| `IsUnauthorized` | 401 or 403 | the API key was rejected |
| `IsBusy` | 409 `agent_busy` | a run is already active; wait for it |
| `IsStreamExpired` | 410 `stream_expired` | retention passed; refetch state instead of resuming |
| `IsInvalidLastEventID` | 400 `invalid_last_event_id` | resume id is not from this run; restart the stream |

A 409 `agent_id_conflict`, raised by re-POSTing a client-supplied `agentId`, is
deliberately **not** `IsBusy`.

## Testing

`test/mockcloud` provides an httptest server covering this surface, including
the one-active-run rule and failure injection. `just test integration` uses it
and needs no API key.
