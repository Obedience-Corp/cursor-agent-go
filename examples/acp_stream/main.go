// Command acp opens a local cursor-agent ACP session, streams one turn, and
// prints the assembled reply.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor/acp"
)

func main() {
	prompt := "Summarize what this directory contains in one sentence."
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var reply strings.Builder
	client, err := acp.Start(ctx, acp.Options{
		WorkingDirectory: ".",
		Stderr:           os.Stderr,
		Handler: acp.HandlerFuncs{
			Update: func(_ context.Context, _ string, u acp.Update) {
				switch u.SessionUpdate {
				case acp.UpdateAgentMessageChunk:
					if u.Content != nil {
						reply.WriteString(u.Content.Text)
					}
				case acp.UpdateToolCall:
					fmt.Fprintf(os.Stderr, "tool: %s (%s)\n", u.Title, u.Kind)
				}
			},
			// Approve the first offered option. Real clients should prompt.
			Permission: func(_ context.Context, r acp.PermissionRequest) acp.PermissionOutcome {
				if len(r.Options) == 0 {
					return acp.CancelPermission()
				}
				return acp.AllowPermission(r.Options[0].OptionID)
			},
		},
	})
	if err != nil {
		log.Fatalf("start acp: %v", err)
	}
	defer func() { _ = client.Close() }()

	if _, err := client.Initialize(ctx); err != nil {
		log.Fatalf("initialize: %v", err)
	}
	session, err := client.NewSession(ctx, "")
	if err != nil {
		log.Fatalf("new session: %v", err)
	}

	result, err := client.Ask(ctx, session.SessionID, prompt)
	if err != nil {
		log.Fatalf("prompt: %v", err)
	}
	fmt.Printf("stop reason: %s\n\n%s\n", result.StopReason, strings.TrimSpace(reply.String()))
}
