// Package cursor is a Go SDK for the Cursor Agent CLI and Cloud Agents API.
//
// It wraps the installed cursor-agent binary (not the editor launcher cursor,
// and not a generic agent command that collides with other tools):
//
//   - one-shot requests through "cursor-agent -p --output-format json"
//   - a long-lived streaming session through "cursor-agent acp"
//   - cloud agents through HTTP and SSE against api.cursor.com
//
// Auth is an explicit API key or CURSOR_API_KEY. Spawned processes always get
// NO_OPEN_BROWSER=1. The SDK never scrapes Cursor IDE cookies.
//
// The local CLI compatibility target is TestedAgentVersion. The API is unstable
// until v1.0.
package cursor
