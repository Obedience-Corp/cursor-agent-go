package cursor

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// Kind classifies a cursor-agent failure.
type Kind string

// Error kinds returned by Classify and the SDK helpers.
const (
	KindAuth                Kind = "auth"
	KindBusy                Kind = "busy"
	KindConflict            Kind = "conflict"
	KindInterrupted         Kind = "interrupted"
	KindNotFound            Kind = "not_found"
	KindPermissionBlocked   Kind = "permission_blocked"
	KindProviderUnavailable Kind = "provider_unavailable"
	KindProcess             Kind = "process"
	KindRateLimit           Kind = "rate_limit"
	KindTransport           Kind = "transport"
	KindValidation          Kind = "validation"
	KindUnknown             Kind = "unknown"
)

// Error is the single error type returned by every SDK call.
type Error struct {
	Kind       Kind
	Message    string
	Retryable  bool
	RetryAfter time.Duration
	StatusCode int
	Code       string
	ExitCode   int
	Stderr     string
	Original   error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Kind)
	}
	return string(e.Kind) + ": " + e.Message
}

// Unwrap exposes the wrapped cause to errors.Is and errors.As.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Original
}

// IsRetryable reports whether retrying the same call could succeed.
func (e *Error) IsRetryable() bool {
	if e == nil {
		return false
	}
	if e.Retryable {
		return true
	}
	switch e.Kind {
	case KindProviderUnavailable, KindRateLimit, KindTransport:
		return true
	}
	return false
}

// IsRetryable reports whether err is a retryable *Error.
func IsRetryable(err error) bool {
	var sdkErr *Error
	if errors.As(err, &sdkErr) {
		return sdkErr.IsRetryable()
	}
	return false
}

func validationError(message string) *Error {
	return &Error{Kind: KindValidation, Message: message}
}

func validationErrorWith(message string, cause error) *Error {
	return &Error{Kind: KindValidation, Message: message, Original: cause}
}

func transportError(message string, cause error) *Error {
	return &Error{Kind: KindTransport, Message: message, Original: cause}
}

func processError(message string, exitCode int, stderr string, cause error) *Error {
	return &Error{Kind: KindProcess, Message: message, ExitCode: exitCode, Stderr: stderr, Original: cause}
}

// Classify turns a print-mode outcome into a typed error, or nil on success.
func Classify(result *AskResult, stderr string, exitCode int, err error) *Error {
	if exitCode == 0 && err == nil && (result == nil || !result.IsError) {
		return nil
	}
	out := &Error{ExitCode: exitCode, Stderr: stderr, Original: err}
	if result != nil && result.IsError && result.Result != "" {
		out.Message = firstLine(result.Result)
	}
	if out.Message == "" {
		out.Message = firstLine(stderr)
	}
	if out.Message == "" {
		out.Message = "cursor-agent exited with code " + strconv.Itoa(exitCode)
	}
	out.Kind = classifyKind(result, stderr, exitCode)
	return out
}

func classifyKind(result *AskResult, stderr string, exitCode int) Kind {
	text := strings.ToLower(stderr)
	if result != nil {
		text = strings.ToLower(result.Result + "\n" + stderr)
	}
	if isAuthFailure(text) {
		return KindAuth
	}
	if strings.Contains(text, "permission") && (strings.Contains(text, "denied") || strings.Contains(text, "blocked")) {
		return KindPermissionBlocked
	}
	if exitCode == 130 {
		return KindInterrupted
	}
	if result != nil && result.IsError {
		return KindProcess
	}
	if exitCode != 0 {
		return KindProcess
	}
	return KindUnknown
}

func isAuthFailure(text string) bool {
	for _, marker := range []string{"unauthorized", "not authenticated", "invalid api key", "sign in", "cursor_api_key"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
