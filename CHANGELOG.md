# Changelog

All notable changes to this project are documented here. The project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) before v1.0, so minor
releases may include API changes.

## [Unreleased]

### Changed

- README leads with cursor-agent itself. Sibling Go SDKs, including
  claude-code-go, are listed at the end.

### Added

- Local `cursor-agent` client: locate, print-json `Ask`/`AskCtx`, version probe,
  and login command builder.
- `LocateBinary` prefers `cursor-agent` and ignores an unrelated `agent` on PATH.
- Guarded `pkg/cursor/dangerous` force/yolo entry points.
- Mock binary, fixtures, examples, CI, and README hero still.
