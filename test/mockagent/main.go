// Command cursor-agent-mock impersonates cursor-agent for SDK tests.
//
// It records argv and Cursor-related environment into $CURSOR_MOCK_RECORD and
// selects a print-json fixture with $CURSOR_MOCK_SCENARIO.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]
	if err := writeRecord(args); err != nil {
		fmt.Fprintln(os.Stderr, "cursor-agent-mock: record:", err)
	}
	os.Exit(route(args))
}

type Record struct {
	Argv []string          `json:"argv"`
	Env  map[string]string `json:"env"`
	Cwd  string            `json:"cwd"`
}

func writeRecord(args []string) error {
	path := os.Getenv("CURSOR_MOCK_RECORD")
	if path == "" {
		return nil
	}
	cwd, _ := os.Getwd()
	record := Record{Argv: args, Env: cursorEnv(), Cwd: cwd}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func cursorEnv() map[string]string {
	out := map[string]string{}
	for _, entry := range os.Environ() {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if name == "NO_OPEN_BROWSER" || strings.HasPrefix(name, "CURSOR_") {
			out[name] = value
		}
	}
	return out
}

// agentVersion mirrors cursor.TestedAgentVersion. It is duplicated rather than
// imported so the mock stays a standalone binary with no SDK dependency.
const agentVersion = "2026.08.31-4057e58"

func route(args []string) int {
	for _, arg := range args {
		if arg == "--version" || arg == "-v" {
			fmt.Println(agentVersion)
			return 0
		}
	}
	scenario := os.Getenv("CURSOR_MOCK_SCENARIO")
	if scenario == "" {
		scenario = "ask-success"
	}
	// Admin subcommands serve their own fixture so a single scenario can cover
	// both an ask and the admin surface.
	if len(args) > 0 {
		switch args[0] {
		case "acp":
			return serveACP()
		case "about", "status", "models":
			scenario = "admin-" + args[0]
		}
	}
	// A rejected flag is the CLI's own argument parser refusing: nothing on
	// stdout, the complaint on stderr, exit 1. It has no fixture because the
	// empty stdout is the point.
	if scenario == "ask-bad-flag" {
		fmt.Fprintln(os.Stderr, "error: option '--mode <mode>' argument 'agent' is invalid. Allowed choices are plan, ask.")
		return 1
	}
	data, err := os.ReadFile(fixturePath(scenario))
	if err != nil {
		fmt.Fprintln(os.Stderr, "cursor-agent-mock:", err)
		return 2
	}
	fmt.Print(string(data))
	if scenario == "ask-auth" {
		fmt.Fprintln(os.Stderr, "Unauthorized: invalid API key")
		return 1
	}
	return 0
}

func fixturePath(scenario string) string {
	dir := os.Getenv("CURSOR_MOCK_TESTDATA")
	if dir == "" {
		dir = "test/testdata"
	}
	jsonPath := filepath.Join(dir, scenario+".json")
	if _, err := os.Stat(jsonPath); err == nil {
		return jsonPath
	}
	return filepath.Join(dir, scenario+".txt")
}
