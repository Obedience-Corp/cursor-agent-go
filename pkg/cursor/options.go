package cursor

import (
	"slices"
	"strconv"
	"time"
)

// Mode is the Cursor Agent execution mode.
type Mode string

// Execution modes.
//
// Only ModePlan and ModeAsk are accepted by the --mode flag: the CLI rejects
// "--mode agent" with "Allowed choices are plan, ask" and exit code 1, even
// though the ACP session/new reply advertises an agent mode alongside the
// other two. Agent is the CLI default, so ModeAgent and ModeUnset both render
// as an absent flag and mean the same thing on the wire. ModeAgent exists so a
// caller can say "agent" explicitly rather than expressing it as an empty
// string.
const (
	ModeUnset Mode = ""
	ModeAgent Mode = "agent"
	ModePlan  Mode = "plan"
	ModeAsk   Mode = "ask"
)

// AskOptions configures a single print-mode run.
type AskOptions struct {
	Model              string
	Mode               Mode
	Force              bool
	Yolo               bool
	AllowDangerousMode bool
	AutoReview         bool
	Sandbox            string
	ApproveMCPs        bool
	Trust              bool
	Workspace          string
	AddDirs            []string
	Resume             string
	Continue           bool
	OutputFormat       string
	StreamPartial      bool
	WorkingDirectory   string
	Timeout            time.Duration
	Env                []string
	Headers            []string
}

// Validate reports configuration that the SDK refuses before spawning.
func (o *AskOptions) Validate() error {
	if err := o.validate(); err != nil {
		return err
	}
	return nil
}

func (o *AskOptions) validate() *Error {
	if o == nil {
		return nil
	}
	if err := o.validateDangerous(); err != nil {
		return err
	}
	if err := validateMode(o.Mode); err != nil {
		return err
	}
	if err := validateSandbox(o.Sandbox); err != nil {
		return err
	}
	if err := validateOutputFormat(o.OutputFormat); err != nil {
		return err
	}
	if o.StreamPartial && o.OutputFormat != "" && o.OutputFormat != "stream-json" {
		return validationError("StreamPartial requires OutputFormat stream-json")
	}
	if o.Timeout < 0 {
		return validationError("Timeout must not be negative")
	}
	if o.Resume != "" && o.Continue {
		return validationError("Resume and Continue are mutually exclusive")
	}
	return nil
}

func (o *AskOptions) validateDangerous() *Error {
	if o.AllowDangerousMode {
		return nil
	}
	if o.Force || o.Yolo {
		return validationError("Force/Yolo requires AllowDangerousMode; use the dangerous subpackage")
	}
	return nil
}

func validateMode(mode Mode) *Error {
	switch mode {
	case ModeUnset, ModeAgent, ModePlan, ModeAsk:
		return nil
	}
	return validationError("unknown mode " + strconv.Quote(string(mode)))
}

func validateSandbox(sandbox string) *Error {
	switch sandbox {
	case "", "enabled", "disabled":
		return nil
	}
	return validationError("sandbox must be \"enabled\", \"disabled\", or empty")
}

func validateOutputFormat(format string) *Error {
	switch format {
	case "", "text", "json", "stream-json":
		return nil
	}
	return validationError("unknown output format " + strconv.Quote(format))
}

func cloneAskOptions(o *AskOptions) *AskOptions {
	if o == nil {
		return &AskOptions{}
	}
	out := *o
	out.AddDirs = slices.Clone(o.AddDirs)
	out.Env = slices.Clone(o.Env)
	out.Headers = slices.Clone(o.Headers)
	return &out
}

// Clone returns a deep copy of the options.
func (o *AskOptions) Clone() *AskOptions {
	return cloneAskOptions(o)
}
