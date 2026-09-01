package cursor

import (
	"strings"
	"testing"
)

func TestLoginCommandSetsNoOpenBrowser(t *testing.T) {
	client := NewClient("/bin/cursor-agent")
	cmd := client.LoginCommand(t.Context())
	if len(cmd.Args) < 2 || cmd.Args[1] != "login" {
		t.Fatalf("args %v", cmd.Args)
	}
	found := false
	for _, entry := range cmd.Env {
		if entry == "NO_OPEN_BROWSER=1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("NO_OPEN_BROWSER missing from %s", strings.Join(cmd.Env, ","))
	}
}
