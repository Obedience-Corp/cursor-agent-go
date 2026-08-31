# cursor-agent-go

Private Go SDK for the [Cursor Agent](https://cursor.com/docs/cli/overview) CLI and
[Cloud Agents API](https://cursor.com/docs/cloud-agent/api/endpoints).

It wraps the installed `agent` / `cursor-agent` binary the same way
[`grok-go-sdk`](https://github.com/lancekrogers/grok-go-sdk) wraps `grok`,
[`claude-code-go`](https://github.com/lancekrogers/claude-code-go) wraps `claude`,
and [`vercel-fx-go`](https://github.com/Obedience-Corp/vercel-fx-go) wraps `fx`:

- **Local one-shot:** `agent -p --output-format json`
- **Local streaming / durable:** `agent acp` (Agent Client Protocol over stdio)
- **Cloud:** HTTP + SSE against `https://api.cursor.com/v1/agents`

Standard library only for production code. The SDK never scrapes Cursor IDE
cookies; auth is an explicit API key or `CURSOR_API_KEY`. Spawned CLI processes
always get `NO_OPEN_BROWSER=1`.

Design package (campaign): `workflow/design/cursor-agent-go-sdk-and-obey-provider/` (WI-5870d1).

Compatibility target and public API are unstable until `v1.0`. See design docs
before implementing against this scaffold.

This is an independent Obedience Corp project. It is not affiliated with or
endorsed by Anysphere / Cursor. Cursor names and marks belong to their respective
owners.

## Requirements

- Go 1.24 or newer
- Cursor Agent CLI installed and authenticated (`agent` on `PATH`, or set
  `CURSOR_API_KEY`)

## Install (consumers)

```bash
export GOPRIVATE=github.com/Obedience-Corp/*
go get github.com/Obedience-Corp/cursor-agent-go@latest
```

## Status

Scaffold only. Implementation tracks WI-5870d1 delivery plan.
