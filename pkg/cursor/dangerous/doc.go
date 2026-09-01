// Package dangerous exposes cursor-agent modes that disable permission checks.
//
// Every entry point refuses unless CURSOR_GO_ENABLE_DANGEROUS is set to
// "i-accept-all-risks", and refuses outright when GO_ENV or NODE_ENV is
// "production". Force/yolo turns off command permission prompts; use it only
// in disposable workspaces. The production refusal inspects only those two
// variables, so it is a best-effort guard rather than a sandbox.
package dangerous
