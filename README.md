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

```bash
just build ask
./bin/ask "What is in go.mod?"
```

## Locate

`NewClientFromPath` calls `LocateBinary`:

1. `CURSOR_AGENT_BIN`
2. `cursor-agent` on `PATH`
3. `~/.local/bin/cursor-agent` and the usual Homebrew/usr paths
4. `agent` only if it resolves to cursor-agent (symlink or install tree)

```go
client := cursor.NewClient("/usr/local/bin/cursor-agent")
```

## Errors

`*cursor.Error` with `KindAuth` or `KindTransport` means the run never started.
`result.IsError` plus a classified process error means the CLI ran and failed.
Check `errors.As` and `err.IsRetryable()`.

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
```

## Related

Go SDKs for other coding-agent CLIs:

- [claude-code-go](https://github.com/lancekrogers/claude-code-go) wraps `claude`
- [grok-go-sdk](https://github.com/lancekrogers/grok-go-sdk) wraps `grok`
- [vercel-fx-go](https://github.com/Obedience-Corp/vercel-fx-go) wraps `fx`

## License

Apache-2.0. See [LICENSE](LICENSE).
