package dangerous

import (
	"context"
	"os"

	"github.com/Obedience-Corp/cursor-agent-go/pkg/cursor"
)

// EnableEnv must equal EnableValue before this package will do anything.
const (
	EnableEnv   = "CURSOR_GO_ENABLE_DANGEROUS"
	EnableValue = "i-accept-all-risks"
)

// Errors returned when the guard rails refuse.
var (
	ErrNotEnabled = &cursor.Error{Kind: cursor.KindValidation, Message: "dangerous: " + EnableEnv + " must be set to \"" + EnableValue + "\""}
	ErrProduction = &cursor.Error{Kind: cursor.KindValidation, Message: "dangerous: refusing to run in a production environment"}
)

// Client wraps a cursor-agent client with the guarded dangerous entry points.
type Client struct {
	inner *cursor.Client
}

// NewDangerousClient builds a guarded client for an explicit binary path.
func NewDangerousClient(binPath string) (*Client, error) {
	if err := Enabled(); err != nil {
		return nil, err
	}
	return &Client{inner: cursor.NewClient(binPath)}, nil
}

// NewDangerousClientFromPath locates cursor-agent and builds a guarded client.
func NewDangerousClientFromPath() (*Client, error) {
	if err := Enabled(); err != nil {
		return nil, err
	}
	inner, err := cursor.NewClientFromPath()
	if err != nil {
		return nil, err
	}
	return &Client{inner: inner}, nil
}

// Wrap guards an existing client.
func Wrap(inner *cursor.Client) (*Client, error) {
	if inner == nil {
		return nil, &cursor.Error{Kind: cursor.KindValidation, Message: "dangerous: client must not be nil"}
	}
	if err := Enabled(); err != nil {
		return nil, err
	}
	return &Client{inner: inner}, nil
}

// Unwrap returns the underlying client.
func (c *Client) Unwrap() *cursor.Client { return c.inner }

// Enabled reports whether the environment permits the dangerous entry points.
func Enabled() error {
	if err := checkEnabled(); err != nil {
		return err
	}
	return nil
}

func checkEnabled() *cursor.Error {
	if os.Getenv(EnableEnv) != EnableValue {
		return ErrNotEnabled
	}
	if os.Getenv("GO_ENV") == "production" || os.Getenv("NODE_ENV") == "production" {
		return ErrProduction
	}
	return nil
}

// AskOptions returns a copy of opts with force/yolo enabled.
func AskOptions(opts *cursor.AskOptions) (*cursor.AskOptions, error) {
	if err := checkEnabled(); err != nil {
		return nil, err
	}
	out := &cursor.AskOptions{}
	if opts != nil {
		out = opts.Clone()
	}
	out.Force = true
	out.Yolo = true
	out.AllowDangerousMode = true
	return out, nil
}

// Force runs one print-json turn with command permission prompts disabled.
func (c *Client) Force(ctx context.Context, prompt string, opts *cursor.AskOptions) (*cursor.AskResult, error) {
	prepared, err := AskOptions(opts)
	if err != nil {
		return nil, err
	}
	return c.inner.AskCtx(ctx, prompt, prepared)
}
