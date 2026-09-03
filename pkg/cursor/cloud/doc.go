// Package cloud is the HTTP and SSE client for Cursor Cloud Agents
// (https://api.cursor.com/v1/agents).
//
// The v1 API separates a durable agent from its runs: creating an agent
// enqueues an initial run, and later prompts create additional runs. Only one
// run is active per agent at a time. Streaming and cancellation are scoped to
// a run.
//
//	c := cloud.New(os.Getenv("CURSOR_API_KEY"))
//	created, err := c.CreateAgent(ctx, cloud.CreateAgentRequest{
//		Prompt: cloud.Prompt{Text: "Add a README"},
//	})
//
// Then follow the run with StreamRun, or poll Run until IsTerminal reports the
// status is final. Non-2xx responses are returned as *APIError, which
// classifies retryable, not-found, and unauthorized conditions.
//
// Import pkg/cursor for local CLI wrapping and pkg/cursor/acp for long-lived
// local sessions.
package cloud
