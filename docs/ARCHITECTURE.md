# Architecture

## Shape

The SDK is a process wrapper. It never speaks to `api2.cursor.sh` itself: it
spawns the installed `cursor-agent` binary, passes overrides through argv and
the environment, and decodes what the CLI writes to stdout.

```
your code
   |
   |  cursor.Client
   v
exec.CommandContext ---> cursor-agent -p --output-format json   (one shot)
                    \--> cursor-agent acp                       (JSON-RPC stdio; landing)
                    \--> cursor-agent --version / login         (admin)
                    \--> HTTPS api.cursor.com/v1/agents         (cloud; landing)
```

`LocateBinary` searches for `cursor-agent` first. The `agent` alias is accepted
only when the executable resolves to cursor-agent (symlink or install path).
A colliding `agent` command from another tool is ignored.

## Packages

| Path | Contents |
| --- | --- |
| `pkg/cursor` | local client surface |
| `pkg/cursor/dangerous` | force/yolo, behind an explicit opt-in |
| `pkg/cursor/cloud` | Cloud Agents HTTP/SSE client (landing) |
| `test/mockagent` | a Go binary that impersonates cursor-agent for tests |
| `test/testdata` | sanitized print-json fixtures |
| `examples` | small programs that compile against the public API |

Inside `pkg/cursor`:

| File | Responsibility |
| --- | --- |
| `client.go` | `Client`, the `execCommand` seam, cwd and env plumbing |
| `locate.go` | finding the cursor-agent binary |
| `options.go` | `AskOptions`, validation, deep clone |
| `args.go` | `BuildPrintArgs`, `BuildACPArgs`, `BuildEnv` |
| `ask.go` | `Ask`, `AskCtx`, `AskResult` |
| `errors.go` | `Error`, `Kind`, `Classify` |
| `login.go` | `LoginCommand` (`*exec.Cmd` for a TTY) |
| `version.go` | `TestedAgentVersion` and `Version` |

## Process model

Every call builds its own `exec.Cmd`, so `Client` is safe for concurrent Ask
use after configuration fields are frozen. `cmd.Dir` is always set.

The environment is layered so the safety overrides cannot be defeated:

```
os.Environ()  +  Client.Env  +  AskOptions.Env  +  typed overrides  +  NO_OPEN_BROWSER=1
```

The last `NO_OPEN_BROWSER=1` wins because a later `KEY=value` shadows an
earlier one. `CURSOR_API_KEY` and `CURSOR_API_ENDPOINT` are set the same way
when the client has an explicit key or endpoint.

## Result handling

Print mode writes a JSON object even when the turn fails. `AskCtx` parses
stdout first and classifies afterwards, so a failure still returns the parsed
`AskResult` when the payload is present.

`Classify` looks at auth markers, permission denials, exit code 130, then a
generic process failure.
