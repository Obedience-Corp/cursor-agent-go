package cursor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocateBinaryUsesExplicitEnv(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "cursor-agent")
	writeExec(t, binary)
	t.Setenv(binEnvName, binary)
	t.Setenv("PATH", t.TempDir())
	got, err := LocateBinary()
	if err != nil {
		t.Fatal(err)
	}
	if got != binary {
		t.Fatalf("located %q, want %q", got, binary)
	}
}

func TestLocateBinaryPrefersCursorAgentOnPATH(t *testing.T) {
	dir := t.TempDir()
	cursorBin := filepath.Join(dir, "cursor-agent")
	agentBin := filepath.Join(dir, "agent")
	writeExec(t, cursorBin)
	writeExec(t, agentBin)
	t.Setenv(binEnvName, "")
	t.Setenv(installDirEnv, t.TempDir())
	t.Setenv("PATH", dir)
	t.Setenv("HOME", t.TempDir())
	got, err := LocateBinary()
	if err != nil {
		t.Fatal(err)
	}
	if got != cursorBin {
		t.Fatalf("located %q, want %q", got, cursorBin)
	}
}

func TestLocateBinaryIgnoresUnrelatedAgent(t *testing.T) {
	dir := t.TempDir()
	writeExec(t, filepath.Join(dir, "agent"))
	t.Setenv(binEnvName, "")
	t.Setenv(installDirEnv, t.TempDir())
	t.Setenv("PATH", dir)
	t.Setenv("HOME", t.TempDir())
	if _, err := os.Stat("/usr/bin/cursor-agent"); err == nil {
		t.Skip("this machine has a system-wide cursor-agent install")
	}
	if _, err := os.Stat("/usr/local/bin/cursor-agent"); err == nil {
		t.Skip("this machine has a system-wide cursor-agent install")
	}
	if _, err := os.Stat("/opt/homebrew/bin/cursor-agent"); err == nil {
		t.Skip("this machine has a homebrew cursor-agent install")
	}
	_, err := LocateBinary()
	requireCursorError(t, err, KindTransport)
}

func TestLocateBinaryAcceptsAgentSymlinkToCursorAgent(t *testing.T) {
	realDir := t.TempDir()
	realBin := filepath.Join(realDir, "cursor-agent")
	writeExec(t, realBin)
	pathDir := t.TempDir()
	alias := filepath.Join(pathDir, "agent")
	if err := os.Symlink(realBin, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv(binEnvName, "")
	t.Setenv(installDirEnv, t.TempDir())
	t.Setenv("PATH", pathDir)
	t.Setenv("HOME", t.TempDir())
	got, err := LocateBinary()
	if err != nil {
		t.Fatal(err)
	}
	if got != alias {
		t.Fatalf("located %q, want %q", got, alias)
	}
}

func TestLocateBinaryIgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "cursor-agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(binEnvName, "")
	t.Setenv(installDirEnv, dir)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if _, err := os.Stat("/usr/bin/cursor-agent"); err == nil {
		t.Skip("this machine has a system-wide cursor-agent install")
	}
	if got, err := LocateBinary(); err == nil && got == filepath.Join(dir, "cursor-agent") {
		t.Fatal("a directory named cursor-agent must not be selected")
	}
}
