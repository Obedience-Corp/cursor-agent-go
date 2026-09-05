package acp

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// longLivedBin writes a stand-in agent that ignores its arguments and blocks on
// stdin. Start always passes "acp" as the first argument, so a bare "cat" would
// try to read a file named acp and exit immediately.
func longLivedBin(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agent")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec cat\n"), 0o755); err != nil {
		t.Fatalf("write fake agent: %v", err)
	}
	return path
}

// TestPIDReportsTheLiveChild covers the identity a host needs to manage the
// process it spawned: a real pid while the child runs, and zero once it does
// not, so "no process" is never confused with an unknown pid.
func TestPIDReportsTheLiveChild(t *testing.T) {
	client, err := Start(t.Context(), Options{BinPath: longLivedBin(t)})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := client.PID()
	if pid <= 0 {
		t.Fatalf("PID = %d, want the live child's pid", pid)
	}
	if err := syscallAlive(pid); err != nil {
		t.Fatalf("pid %d is not a live process: %v", pid, err)
	}

	_ = client.Close()

	if got := client.PID(); got != 0 {
		t.Fatalf("PID after close = %d, want 0", got)
	}
}

// TestDoneClosesWhenTheChildExitsOnItsOwn is the case the SDK could not report
// before: the process dies without the caller asking, and the caller should not
// have to discover that by issuing a request that fails.
func TestDoneClosesWhenTheChildExitsOnItsOwn(t *testing.T) {
	// "true" exits immediately, standing in for a child that dies on its own.
	client, err := Start(t.Context(), Options{BinPath: "true"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case <-client.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("Done was never closed for a child that exited on its own")
	}
	if got := client.PID(); got != 0 {
		t.Fatalf("PID after exit = %d, want 0", got)
	}
	if err := client.ExitErr(); err != nil {
		t.Fatalf("ExitErr for a clean exit = %v, want nil", err)
	}
}

// TestDoneStaysOpenWhileTheChildRuns guards the other direction: a long-lived
// process must not look dead, or a host would retire a healthy session.
func TestDoneStaysOpenWhileTheChildRuns(t *testing.T) {
	client, err := Start(t.Context(), Options{BinPath: longLivedBin(t)})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case <-client.Done():
		t.Fatal("Done closed while the child was still running")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestExitErrReportsANonZeroExit keeps the exit status reachable, since Close
// memoizes the same error and a caller may want it without closing.
func TestExitErrReportsANonZeroExit(t *testing.T) {
	client, err := Start(t.Context(), Options{BinPath: "false"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.ExitErr(); err == nil {
		t.Fatal("ExitErr for a non-zero exit = nil, want the exit error")
	}
}

// TestCloseReturnsTheSameErrorAsExitErr pins that moving cmd.Wait into a single
// waiter did not change what Close reports.
func TestCloseReturnsTheSameErrorAsExitErr(t *testing.T) {
	client, err := Start(t.Context(), Options{BinPath: "false"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := client.ExitErr()
	if got := client.Close(); got != want {
		t.Fatalf("Close = %v, ExitErr = %v; they must agree", got, want)
	}
}

// syscallAlive checks that a pid names a live process. Signal 0 tests for
// existence without delivering anything.
func syscallAlive(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.Signal(0))
}
