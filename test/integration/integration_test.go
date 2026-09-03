//go:build integration

// Package integration exercises the SDK end to end against the mock agent
// binary and the mock cloud server. Build tag: integration.
//
// Set CURSOR_INTEGRATION_REAL=1 to additionally run the lanes that talk to a
// real, installed cursor-agent. Those are skipped by default so the suite needs
// no credentials and makes no billable calls.
package integration

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor"
	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor/acp"
	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor/cloud"
	"github.com/Obedience-Corp/cursor-agent-go/test/mockcloud"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, this, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller")
	}
	return filepath.Join(filepath.Dir(this), "..", "..")
}

// buildMockAgent compiles the mock impersonator into a temp dir.
func buildMockAgent(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "cursor-agent")
	cmd := exec.Command("go", "build", "-o", bin, "./test/mockagent")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build mockagent: %v\n%s", err, out)
	}
	return bin
}

func realAgentLane(t *testing.T) {
	t.Helper()
	if os.Getenv("CURSOR_INTEGRATION_REAL") != "1" {
		t.Skip("set CURSOR_INTEGRATION_REAL=1 to run against a real cursor-agent")
	}
}

func TestPrintModeAgainstMockBinary(t *testing.T) {
	client := cursor.NewClient(buildMockAgent(t))
	t.Setenv("CURSOR_MOCK_TESTDATA", filepath.Join(repoRoot(t), "test", "testdata"))

	result, err := client.AskCtx(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("AskCtx: %v", err)
	}
	if result.Usage.InputTokens == 0 {
		t.Fatal("usage was not decoded from the fixture")
	}
	if result.SessionID == "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestACPSessionAgainstMockBinary(t *testing.T) {
	bin := buildMockAgent(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := acp.Start(ctx, acp.Options{BinPath: bin, WorkingDirectory: t.TempDir()})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = client.Close() }()

	init, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if init.ProtocolVersion != acp.ProtocolVersion {
		t.Fatalf("protocolVersion = %d", init.ProtocolVersion)
	}

	session, err := client.NewSession(ctx, "")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	transcript, err := client.CollectAsk(ctx, session.SessionID, "do the thing")
	if err != nil {
		t.Fatalf("CollectAsk: %v", err)
	}
	if transcript.Text != "Hello world" {
		t.Fatalf("Text = %q", transcript.Text)
	}
	if transcript.StopReason != acp.StopEndTurn {
		t.Fatalf("StopReason = %q", transcript.StopReason)
	}
	if len(transcript.ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v", transcript.ToolCalls)
	}
	// The announce frame carried an empty rawInput; the merge must pick up the
	// arguments that only arrive on the later update.
	if !strings.Contains(string(transcript.ToolCalls[0].RawInput), "/tmp/mock.txt") {
		t.Fatalf("rawInput not merged: %s", transcript.ToolCalls[0].RawInput)
	}
}

func TestACPPermissionRoundTripAgainstMockBinary(t *testing.T) {
	bin := buildMockAgent(t)
	t.Setenv("CURSOR_MOCK_ACP", "permission")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var granted string
	client, err := acp.Start(ctx, acp.Options{
		BinPath:          bin,
		WorkingDirectory: t.TempDir(),
		Handler: acp.HandlerFuncs{
			Permission: func(_ context.Context, req acp.PermissionRequest) acp.PermissionOutcome {
				out := acp.PolicyAllowOnce.Decide(req.Options)
				granted = out.OptionID
				return out
			},
		},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = client.Close() }()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	session, err := client.NewSession(ctx, "")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if _, err := client.Ask(ctx, session.SessionID, "edit a file"); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if granted != "o-once" {
		t.Fatalf("granted = %q, want o-once", granted)
	}
}

func TestACPToolCallIDWithNewlineAgainstMockBinary(t *testing.T) {
	bin := buildMockAgent(t)
	t.Setenv("CURSOR_MOCK_ACP", "toolnewline")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := acp.Start(ctx, acp.Options{BinPath: bin, WorkingDirectory: t.TempDir()})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = client.Close() }()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	session, err := client.NewSession(ctx, "")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	transcript, err := client.CollectAsk(ctx, session.SessionID, "edit")
	if err != nil {
		t.Fatalf("CollectAsk: %v", err)
	}
	if len(transcript.ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v", transcript.ToolCalls)
	}
	if !strings.Contains(transcript.ToolCalls[0].ID, "\n") {
		t.Fatalf("newline lost from tool call id: %q", transcript.ToolCalls[0].ID)
	}
}

func TestCloudLifecycleAgainstMockServer(t *testing.T) {
	srv := mockcloud.Start()
	defer srv.Close()
	c := &cloud.Client{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()}
	ctx := context.Background()

	created, err := c.CreateAgent(ctx, cloud.CreateAgentRequest{Prompt: cloud.Prompt{Text: "do work"}})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Only one run may be active per agent.
	_, err = c.CreateRun(ctx, created.Agent.ID, cloud.CreateRunRequest{Prompt: cloud.Prompt{Text: "again"}})
	var apiErr *cloud.APIError
	if !errors.As(err, &apiErr) || !apiErr.IsBusy() {
		t.Fatalf("second run err = %v, want agent_busy", err)
	}

	run, err := c.WaitRun(ctx, created.Agent.ID, created.Run.ID, time.Millisecond)
	if err != nil {
		t.Fatalf("WaitRun: %v", err)
	}
	if run.Status != cloud.RunFinished {
		t.Fatalf("run = %+v", run)
	}

	usage, err := c.AgentUsage(ctx, created.Agent.ID)
	if err != nil || usage.InputTokens == 0 {
		t.Fatalf("AgentUsage = %+v, %v", usage, err)
	}
	arts, err := c.ListArtifacts(ctx, created.Agent.ID)
	if err != nil || len(arts) != 1 {
		t.Fatalf("ListArtifacts = %+v, %v", arts, err)
	}
}

func TestCloudStreamAgainstMockServer(t *testing.T) {
	srv := mockcloud.Start()
	defer srv.Close()
	c := &cloud.Client{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()}
	ctx := context.Background()

	created, err := c.CreateAgent(ctx, cloud.CreateAgentRequest{Prompt: cloud.Prompt{Text: "stream"}})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	stream, err := c.StreamRun(ctx, created.Agent.ID, created.Run.ID, nil)
	if err != nil {
		t.Fatalf("StreamRun: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var text strings.Builder
	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		if event.Type == cloud.EventAssistant {
			var d cloud.TextData
			if err := json.Unmarshal(event.Data, &d); err == nil {
				text.WriteString(d.Text)
			}
		}
	}
	if text.String() != "Hello world" {
		t.Fatalf("text = %q", text.String())
	}
	if stream.LastEventID() != "evt-3" {
		t.Fatalf("LastEventID = %q, want evt-3 (id-less done must not clobber it)", stream.LastEventID())
	}
}

func TestCloudStreamExpiredIsTyped(t *testing.T) {
	srv := mockcloud.Start()
	defer srv.Close()
	c := &cloud.Client{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()}
	ctx := context.Background()

	created, err := c.CreateAgent(ctx, cloud.CreateAgentRequest{Prompt: cloud.Prompt{Text: "x"}})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	srv.FailNext, srv.FailNextCode = 410, cloud.CodeStreamExpired
	_, err = c.StreamRun(ctx, created.Agent.ID, created.Run.ID, &cloud.StreamOptions{LastEventID: "evt-1"})
	var apiErr *cloud.APIError
	if !errors.As(err, &apiErr) || !apiErr.IsStreamExpired() {
		t.Fatalf("err = %v, want stream_expired", err)
	}
}

func TestRealAgentPrintMode(t *testing.T) {
	realAgentLane(t)
	client, err := cursor.NewClientFromPath()
	if err != nil {
		t.Fatalf("NewClientFromPath: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	result, err := client.AskCtx(ctx, "Reply with exactly: PONG", &cursor.AskOptions{Trust: true})
	if err != nil {
		t.Fatalf("AskCtx: %v", err)
	}
	if !strings.Contains(result.Result, "PONG") {
		t.Fatalf("result = %q", result.Result)
	}
	if result.Usage.InputTokens == 0 {
		t.Fatal("real run reported no input tokens")
	}
}

func TestRealAgentACPSession(t *testing.T) {
	realAgentLane(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client, err := acp.Start(ctx, acp.Options{WorkingDirectory: t.TempDir()})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = client.Close() }()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	session, err := client.NewSession(ctx, "")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	transcript, err := client.CollectAsk(ctx, session.SessionID, "Reply with exactly the word: PONG")
	if err != nil {
		t.Fatalf("CollectAsk: %v", err)
	}
	if !strings.Contains(transcript.Text, "PONG") {
		t.Fatalf("text = %q", transcript.Text)
	}
}
