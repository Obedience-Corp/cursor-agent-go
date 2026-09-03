package cursor

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAskCtxRejectsBadInput(t *testing.T) {
	client := mockClient(t, "ask-success")
	tests := []struct {
		name    string
		run     func() error
		wantSub string
	}{
		{
			name:    "empty prompt",
			run:     func() error { _, err := client.AskCtx(context.Background(), "   ", nil); return err },
			wantSub: "prompt must not be empty",
		},
		{
			name: "force without gate is rejected before spawning",
			run: func() error {
				_, err := client.AskCtx(context.Background(), "hi", &AskOptions{Force: true})
				return err
			},
			wantSub: "AllowDangerousMode",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sdkErr := requireCursorError(t, tc.run(), KindValidation)
			if !strings.Contains(sdkErr.Message, tc.wantSub) {
				t.Fatalf("message %q does not contain %q", sdkErr.Message, tc.wantSub)
			}
		})
	}
}

func TestAskCtxSuccess(t *testing.T) {
	client, record := recordingClient(t, "ask-success")
	client.APIKey = "test-key"
	result, err := client.AskCtx(context.Background(), "Summarize README.md", &AskOptions{
		Model: "composer-2.5",
		Trust: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Result == "" {
		t.Fatal("expected result text")
	}
	if result.SessionID == "" {
		t.Fatal("expected session id")
	}
	rec := record()
	joined := strings.Join(rec.Argv, " ")
	if !strings.Contains(joined, "-p") || !strings.Contains(joined, "--output-format json") {
		t.Fatalf("print json flags missing: %v", rec.Argv)
	}
	if rec.Env["NO_OPEN_BROWSER"] != "1" {
		t.Fatalf("NO_OPEN_BROWSER=%q", rec.Env["NO_OPEN_BROWSER"])
	}
	if rec.Env["CURSOR_API_KEY"] != "test-key" {
		t.Fatalf("CURSOR_API_KEY=%q", rec.Env["CURSOR_API_KEY"])
	}
}

func TestAskCtxAuthFailure(t *testing.T) {
	client := mockClient(t, "ask-auth")
	_, err := client.AskCtx(context.Background(), "hello", nil)
	requireCursorError(t, err, KindAuth)
}

func TestVersion(t *testing.T) {
	client := mockClient(t, "ask-success")
	got, err := client.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != TestedAgentVersion {
		t.Fatalf("version %q, want %q", got, TestedAgentVersion)
	}
}

func TestClassifyAuth(t *testing.T) {
	err := Classify(nil, "Unauthorized: invalid API key", 1, errors.New("exit"))
	sdkErr := requireCursorError(t, err, KindAuth)
	if !strings.Contains(sdkErr.Message, "Unauthorized") {
		t.Fatalf("message %q", sdkErr.Message)
	}
}

func TestAskCtxParsesUsage(t *testing.T) {
	client := mockClient(t, "ask-success")
	result, err := client.AskCtx(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("AskCtx: %v", err)
	}
	want := Usage{InputTokens: 19833, OutputTokens: 5}
	if result.Usage != want {
		t.Fatalf("usage = %+v, want %+v", result.Usage, want)
	}
}

func TestAskCtxWorkspaceTrustGate(t *testing.T) {
	client := mockClient(t, "ask-untrusted")
	_, err := client.AskCtx(context.Background(), "hello", nil)
	sdkErr := requireCursorError(t, err, KindWorkspaceUntrusted)
	if !strings.Contains(sdkErr.Message, "Trust") {
		t.Fatalf("message %q does not mention Trust", sdkErr.Message)
	}
}
