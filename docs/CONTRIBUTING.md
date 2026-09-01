# Contributing

## Gates

Everything below must pass before a change lands:

```bash
just lint
just test all
just test race
just build all
```

`just gate` runs the complete isolated gate under Go 1.24 and Go 1.26.

## House rules

- Standard library only in non-test code.
- `context.Context` is the first parameter of anything that spawns a process.
- Never `fmt.Errorf` for SDK failures. Build errors through `validationError`,
  `transportError`, or `processError` so `errors.Is` and `errors.As` keep working.
- Watch the typed-nil trap. A function that returns `error` must not tail-call
  something that returns `*Error`: a nil `*Error` in an `error` interface is not
  nil.
- Files stay under 500 lines and functions under 50.
- No comments inside function bodies. Doc comments on exported identifiers only.
- No em dashes anywhere in code, docs, or commit messages.
- Wrap `cursor-agent`, never a colliding `agent` binary, unless locate has
  verified the file is cursor-agent.

## Testing

Tests are table driven and list the error cases before the happy path.

Unit tests never touch the real `cursor-agent`. They point `Client.BinPath` at
the mock binary built from `test/mockagent`.

## Fixtures

`test/testdata` holds sanitized contract fixtures. Update them only from a
verified CLI capture with secrets and emails redacted.

## Regenerating the README hero

The still is produced with the grok CLI Imagine tool, same recipe as
`technical_videos/docs/guides/generating-images-with-grok-cli.md`:

```bash
just docs hero
just docs hero-check
```

Do not pass `--effort` to grok. Judge success by whether `docs/images/hero.png`
is a 1280x720 PNG, then eyeball it for garbled letters (the committed prompt
asks for none).
