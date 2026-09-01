package cursor

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	mockOnce sync.Once
	mockDir  string
	mockPath string
	mockErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if mockDir != "" {
		_ = os.RemoveAll(mockDir)
	}
	os.Exit(code)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, this, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller for repo root")
	}
	return filepath.Join(filepath.Dir(this), "..", "..")
}

func mockBin(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	mockOnce.Do(func() {
		mockDir, mockErr = os.MkdirTemp("", "cursor-agent-go-mock-")
		if mockErr != nil {
			return
		}
		out := filepath.Join(mockDir, "cursor-agent-mock")
		cmd := exec.Command("go", "build", "-o", out, "./test/mockagent")
		cmd.Dir = root
		if combined, err := cmd.CombinedOutput(); err != nil {
			mockErr = err
			t.Logf("go build mock:\n%s", combined)
			return
		}
		mockPath = out
	})
	if mockErr != nil {
		t.Fatalf("build mock binary: %v", mockErr)
	}
	return mockPath
}

func testdataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "test", "testdata")
}

func mockClient(t *testing.T, scenario string) *Client {
	t.Helper()
	client := NewClient(mockBin(t))
	client.WorkingDir = t.TempDir()
	client.Env = []string{
		"CURSOR_MOCK_TESTDATA=" + testdataDir(t),
		"CURSOR_MOCK_SCENARIO=" + scenario,
	}
	return client
}

type mockRecord struct {
	Argv []string          `json:"argv"`
	Env  map[string]string `json:"env"`
	Cwd  string            `json:"cwd"`
}

func recordingClient(t *testing.T, scenario string) (*Client, func() mockRecord) {
	t.Helper()
	client := mockClient(t, scenario)
	path := filepath.Join(t.TempDir(), "record.json")
	client.Env = append(client.Env, "CURSOR_MOCK_RECORD="+path)
	return client, func() mockRecord {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read mock record: %v", err)
		}
		var out mockRecord
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("decode mock record: %v", err)
		}
		return out
	}
}

func requireCursorError(t *testing.T, err error, kind Kind) *Error {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a %s error, got nil", kind)
	}
	var sdkErr *Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("expected *cursor.Error, got %T: %v", err, err)
	}
	if sdkErr.Kind != kind {
		t.Fatalf("expected kind %s, got %s (%v)", kind, sdkErr.Kind, sdkErr)
	}
	return sdkErr
}

func writeExec(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
