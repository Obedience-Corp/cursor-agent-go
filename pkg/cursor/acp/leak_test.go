package acp

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// waitForGoroutines polls until the count settles at or below want, so a
// scheduler that has not yet reaped an exited goroutine does not flake.
func waitForGoroutines(t *testing.T, want int) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	got := runtime.NumGoroutine()
	for time.Now().Before(deadline) {
		got = runtime.NumGoroutine()
		if got <= want {
			return got
		}
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}
	return got
}

// Close must stop the read loop. Without a dedicated leak-detection dependency
// this compares goroutine counts across a start/close cycle.
func TestStartAndCloseDoesNotLeakGoroutines(t *testing.T) {
	baseline := waitForGoroutines(t, runtime.NumGoroutine())

	for range 5 {
		client, err := Start(t.Context(), Options{BinPath: "cat"})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if err := client.Close(); err != nil {
			// cat exits non-zero on a closed stdin on some platforms; the
			// goroutine assertion below is the point of this test.
			t.Logf("Close returned %v", err)
		}
	}

	if got := waitForGoroutines(t, baseline); got > baseline {
		t.Fatalf("goroutines grew from %d to %d across 5 start/close cycles", baseline, got)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	client, err := Start(t.Context(), Options{BinPath: "cat"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	first := client.Close()
	if second := client.Close(); second != first {
		t.Fatalf("second Close returned %v, want the memoized %v", second, first)
	}
}

func TestCallAfterCloseFails(t *testing.T) {
	client, err := Start(t.Context(), Options{BinPath: "cat"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	_ = client.Close()
	if _, err := client.Initialize(context.Background()); err == nil {
		t.Fatal("Initialize succeeded after Close")
	}
}
