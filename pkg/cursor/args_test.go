package cursor

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBuildPrintArgs(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		opts   *AskOptions
		want   []string
	}{
		{
			name:   "nil options still force print json and separate the prompt",
			prompt: "hello",
			opts:   nil,
			want:   []string{"-p", "--output-format", "json", "--", "hello"},
		},
		{
			name:   "prompt starting with a dash stays a prompt",
			prompt: "--not-a-flag",
			opts:   &AskOptions{},
			want:   []string{"-p", "--output-format", "json", "--", "--not-a-flag"},
		},
		{
			name:   "model mode sandbox and workspace",
			prompt: "go",
			opts: &AskOptions{
				Model:     "composer-2.5",
				Mode:      ModePlan,
				Sandbox:   "enabled",
				Trust:     true,
				Workspace: "/repo",
				AddDirs:   []string{"/a", "/b"},
			},
			want: []string{
				"-p", "--output-format", "json", "--model", "composer-2.5",
				"--mode", "plan", "--sandbox", "enabled", "--trust",
				"--workspace", "/repo", "--add-dir", "/a", "--add-dir", "/b",
				"--", "go",
			},
		},
		{
			name:   "force is rendered when the caller accepted the risk",
			prompt: "go",
			opts:   &AskOptions{Force: true, AllowDangerousMode: true, Yolo: true},
			want:   []string{"-p", "--output-format", "json", "--force", "--", "go"},
		},
		{
			name:   "stream-json partial output",
			prompt: "go",
			opts:   &AskOptions{OutputFormat: "stream-json", StreamPartial: true, Continue: true},
			want: []string{
				"-p", "--output-format", "stream-json", "--stream-partial-output",
				"--continue", "--", "go",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildPrintArgs(tc.prompt, tc.opts)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v\nwant %v", got, tc.want)
			}
		})
	}
}

func TestBuildACPArgs(t *testing.T) {
	got := BuildACPArgs(&AskOptions{Model: "composer-2.5", Mode: ModeAsk})
	want := []string{"acp", "--model", "composer-2.5", "--mode", "ask"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// TestAgentModeRendersNoFlag pins the CLI contract both builders have to obey:
// --mode accepts only plan and ask. Against cursor-agent 2026.09.02-c22c1a3,
// "--mode agent" exits 1 with
//
//	error: option '--mode <mode>' argument 'agent' is invalid. Allowed choices are plan, ask.
//
// for print mode and for the acp subcommand alike, so emitting it made every
// agent-mode run fail before it started. Agent is the default: it is an absent
// flag, not a value.
func TestAgentModeRendersNoFlag(t *testing.T) {
	builders := map[string]func(*AskOptions) []string{
		"print": func(opts *AskOptions) []string { return BuildPrintArgs("hi", opts) },
		"acp":   BuildACPArgs,
	}
	for name, build := range builders {
		for _, mode := range []Mode{ModeAgent, ModeUnset} {
			t.Run(name+"/"+string(mode), func(t *testing.T) {
				args := build(&AskOptions{Model: "composer-2.5", Mode: mode})
				if slices.Contains(args, "--mode") {
					t.Fatalf("%v carries a --mode flag; agent is the CLI default", args)
				}
				if slices.Contains(args, "agent") {
					t.Fatalf("%v carries the value \"agent\", which the CLI rejects", args)
				}
			})
		}
	}
}

// TestAgentAndUnsetModeAreIdenticalOnTheWire states the equivalence directly,
// so a future change cannot make one of them mean something else by accident.
func TestAgentAndUnsetModeAreIdenticalOnTheWire(t *testing.T) {
	agent := BuildPrintArgs("hi", &AskOptions{Mode: ModeAgent})
	unset := BuildPrintArgs("hi", &AskOptions{Mode: ModeUnset})
	if !reflect.DeepEqual(agent, unset) {
		t.Fatalf("agent %v and unset %v must render identically", agent, unset)
	}
}

func TestBuildEnvAlwaysDisablesBrowser(t *testing.T) {
	env := BuildEnv("secret", "https://example.invalid", []string{
		"NO_OPEN_BROWSER=0",
		"CURSOR_API_KEY=old",
		"KEEP=1",
	})
	joined := strings.Join(env, ",")
	if !strings.Contains(joined, "NO_OPEN_BROWSER=1") {
		t.Fatalf("missing NO_OPEN_BROWSER=1 in %v", env)
	}
	if strings.Contains(joined, "NO_OPEN_BROWSER=0") {
		t.Fatalf("stale NO_OPEN_BROWSER survived: %v", env)
	}
	if strings.Contains(joined, "CURSOR_API_KEY=old") {
		t.Fatalf("stale API key survived: %v", env)
	}
	if !strings.Contains(joined, "CURSOR_API_KEY=secret") {
		t.Fatalf("missing API key in %v", env)
	}
	if !strings.Contains(joined, "KEEP=1") {
		t.Fatalf("caller env dropped: %v", env)
	}
}

func TestAskOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    AskOptions
		wantSub string
	}{
		{name: "force without gate", opts: AskOptions{Force: true}, wantSub: "AllowDangerousMode"},
		{name: "unknown mode", opts: AskOptions{Mode: "yolo"}, wantSub: "unknown mode"},
		{name: "bad sandbox", opts: AskOptions{Sandbox: "maybe"}, wantSub: "sandbox"},
		{name: "stream partial with json", opts: AskOptions{OutputFormat: "json", StreamPartial: true}, wantSub: "StreamPartial"},
		{name: "negative timeout", opts: AskOptions{Timeout: -time.Second}, wantSub: "Timeout"},
		{name: "resume and continue", opts: AskOptions{Resume: "abc", Continue: true}, wantSub: "mutually exclusive"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Validate()
			sdkErr := requireCursorError(t, err, KindValidation)
			if !strings.Contains(sdkErr.Message, tc.wantSub) {
				t.Fatalf("message %q does not contain %q", sdkErr.Message, tc.wantSub)
			}
		})
	}
}

func TestAskOptionsCloneDoesNotShareSlices(t *testing.T) {
	orig := &AskOptions{AddDirs: []string{"/a"}, Env: []string{"A=1"}, Headers: []string{"X: 1"}}
	cloned := orig.Clone()
	cloned.AddDirs[0] = "/b"
	cloned.Env[0] = "A=2"
	cloned.Headers[0] = "X: 2"
	if orig.AddDirs[0] != "/a" || orig.Env[0] != "A=1" || orig.Headers[0] != "X: 1" {
		t.Fatal("clone shared backing arrays")
	}
}
