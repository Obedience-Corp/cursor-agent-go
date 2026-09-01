package dangerous_test

import (
	"testing"

	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor/dangerous"
)

func TestEnabledRequiresEnv(t *testing.T) {
	t.Setenv(dangerous.EnableEnv, "")
	if err := dangerous.Enabled(); err == nil {
		t.Fatal("expected error")
	}
}

func TestEnabledRefusesProduction(t *testing.T) {
	t.Setenv(dangerous.EnableEnv, dangerous.EnableValue)
	t.Setenv("GO_ENV", "production")
	if err := dangerous.Enabled(); err == nil {
		t.Fatal("expected production refusal")
	}
}

func TestAskOptionsSetsForce(t *testing.T) {
	t.Setenv(dangerous.EnableEnv, dangerous.EnableValue)
	t.Setenv("GO_ENV", "")
	t.Setenv("NODE_ENV", "")
	opts, err := dangerous.AskOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Force || !opts.Yolo || !opts.AllowDangerousMode {
		t.Fatalf("force flags not set: %+v", opts)
	}
}

func TestWrapNilClient(t *testing.T) {
	t.Setenv(dangerous.EnableEnv, dangerous.EnableValue)
	t.Setenv("GO_ENV", "")
	t.Setenv("NODE_ENV", "")
	if _, err := dangerous.Wrap(nil); err == nil {
		t.Fatal("expected error")
	}
}
