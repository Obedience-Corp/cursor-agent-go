package cursor

import "strings"

const (
	envNoOpenBrowser = "NO_OPEN_BROWSER=1"
	envAPIKey        = "CURSOR_API_KEY"
	envEndpoint      = "CURSOR_API_ENDPOINT"
)

// BuildPrintArgs renders argv for a print-mode run. The binary name is not
// included. The prompt is always separated with "--" so a prompt starting with
// a dash cannot be parsed as a flag.
func BuildPrintArgs(prompt string, opts *AskOptions) []string {
	if opts == nil {
		opts = &AskOptions{}
	}
	args := make([]string, 0, 16)
	args = append(args, "-p")
	format := opts.OutputFormat
	if format == "" {
		format = "json"
	}
	args = append(args, "--output-format", format)
	if opts.StreamPartial {
		args = append(args, "--stream-partial-output")
	}
	args = append(args, printFlagArgs(opts)...)
	if prompt != "" {
		args = append(args, "--", prompt)
	}
	return args
}

// BuildACPArgs renders argv for "cursor-agent acp".
func BuildACPArgs(opts *AskOptions) []string {
	args := []string{"acp"}
	if opts == nil {
		return args
	}
	return append(args, printFlagArgs(opts)...)
}

// BuildEnv renders the process environment the SDK manages for a run.
// NO_OPEN_BROWSER is always set. API key and endpoint are set when non-empty.
func BuildEnv(apiKey, endpoint string, extra []string) []string {
	managed := []string{envNoOpenBrowser}
	if apiKey != "" {
		managed = append(managed, envAPIKey+"="+apiKey)
	}
	if endpoint != "" {
		managed = append(managed, envEndpoint+"="+endpoint)
	}
	env := stripEnvKeys(append([]string(nil), extra...), envKeys(managed)...)
	return append(env, managed...)
}

func printFlagArgs(opts *AskOptions) []string {
	if opts == nil {
		return nil
	}
	args := make([]string, 0, 12)
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	switch opts.Mode {
	case ModePlan:
		args = append(args, "--mode", "plan")
	case ModeAsk:
		args = append(args, "--mode", "ask")
	case ModeAgent:
		args = append(args, "--mode", "agent")
	}
	if opts.Force || opts.Yolo {
		args = append(args, "--force")
	}
	if opts.AutoReview {
		args = append(args, "--auto-review")
	}
	if opts.Sandbox != "" {
		args = append(args, "--sandbox", opts.Sandbox)
	}
	if opts.ApproveMCPs {
		args = append(args, "--approve-mcps")
	}
	if opts.Trust {
		args = append(args, "--trust")
	}
	if opts.Workspace != "" {
		args = append(args, "--workspace", opts.Workspace)
	}
	for _, dir := range opts.AddDirs {
		args = append(args, "--add-dir", dir)
	}
	if opts.Resume != "" {
		args = append(args, "--resume", opts.Resume)
	}
	if opts.Continue {
		args = append(args, "--continue")
	}
	for _, header := range opts.Headers {
		args = append(args, "-H", header)
	}
	return args
}

func envKeys(env []string) []string {
	keys := make([]string, 0, len(env))
	for _, entry := range env {
		if name, _, ok := strings.Cut(entry, "="); ok {
			keys = append(keys, name)
		}
	}
	return keys
}

func stripEnvKeys(env []string, keys ...string) []string {
	if len(env) == 0 || len(keys) == 0 {
		return env
	}
	drop := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		drop[key] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, found := drop[name]; found {
				continue
			}
		}
		out = append(out, entry)
	}
	return out
}
