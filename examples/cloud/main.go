// Command cloud creates a Cursor Cloud Agent and follows its first run over
// Server-Sent Events. Requires CURSOR_API_KEY.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor/cloud"
)

func main() {
	apiKey := os.Getenv("CURSOR_API_KEY")
	if apiKey == "" {
		log.Fatal("CURSOR_API_KEY is required")
	}
	prompt := "Add a README with setup instructions"
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	client := cloud.New(apiKey)
	created, err := client.CreateAgent(ctx, cloud.CreateAgentRequest{
		Prompt: cloud.Prompt{Text: prompt},
	})
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}
	fmt.Printf("agent %s (%s)\nrun   %s\n\n", created.Agent.ID, created.Agent.URL, created.Run.ID)

	stream, err := client.StreamRun(ctx, created.Agent.ID, created.Run.ID, nil)
	if err != nil {
		log.Fatalf("stream: %v", err)
	}
	defer func() { _ = stream.Close() }()

	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatalf("recv: %v", err)
		}
		switch event.Type {
		case cloud.EventAssistant:
			var d cloud.TextData
			if err := json.Unmarshal(event.Data, &d); err == nil {
				fmt.Print(d.Text)
			}
		case cloud.EventStatus:
			var d cloud.StatusData
			if err := json.Unmarshal(event.Data, &d); err == nil {
				fmt.Fprintf(os.Stderr, "[status %s]\n", d.Status)
			}
		case cloud.EventResult:
			var d cloud.ResultData
			if err := json.Unmarshal(event.Data, &d); err == nil {
				fmt.Printf("\n\n[%s in %dms]\n", d.Status, d.DurationMs)
			}
		}
	}

	usage, err := client.AgentUsage(ctx, created.Agent.ID)
	if err == nil {
		fmt.Printf("usage: in=%d out=%d cacheRead=%d\n",
			usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens)
	}
}
