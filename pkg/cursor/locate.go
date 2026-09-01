package cursor

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	binaryName     = "cursor-agent"
	binaryAlias    = "agent"
	binEnvName     = "CURSOR_AGENT_BIN"
	installDirEnv  = "CURSOR_AGENT_INSTALL_DIR"
	cursorAgentDir = "cursor-agent"
)

// LocateBinary finds the cursor-agent binary.
//
// Search order:
//  1. CURSOR_AGENT_BIN, if it points at an executable file
//  2. cursor-agent on PATH
//  3. standard install locations named cursor-agent
//  4. an agent executable only when it resolves to cursor-agent (symlink or
//     install path). A colliding agent command from another tool is ignored.
func LocateBinary() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv(binEnvName)); explicit != "" {
		if !isExecutablePath(explicit) {
			return "", &Error{Kind: KindTransport, Message: "CURSOR_AGENT_BIN is set but is not an executable file"}
		}
		return explicit, nil
	}
	if p, err := exec.LookPath(binaryName); err == nil && isCursorAgentBinary(p) {
		return p, nil
	}
	for _, candidate := range binaryCandidates(binaryName) {
		if isExecutablePath(candidate) {
			return candidate, nil
		}
	}
	if p, err := exec.LookPath(binaryAlias); err == nil && isCursorAgentBinary(p) {
		return p, nil
	}
	for _, candidate := range binaryCandidates(binaryAlias) {
		if isExecutablePath(candidate) && isCursorAgentBinary(candidate) {
			return candidate, nil
		}
	}
	return "", &Error{Kind: KindTransport, Message: "cursor-agent binary not found on PATH or in standard install locations"}
}

func isCursorAgentBinary(path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}
	lower := strings.ToLower(resolved)
	base := strings.ToLower(filepath.Base(resolved))
	if strings.Contains(base, binaryName) {
		return true
	}
	return strings.Contains(lower, cursorAgentDir)
}

func isExecutablePath(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0
}

func binaryCandidates(name string) []string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidates := make([]string, 0, 6)
	if dir := os.Getenv(installDirEnv); dir != "" {
		candidates = append(candidates, filepath.Join(dir, name))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "bin", name))
	}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, filepath.Join("/opt/homebrew/bin", name))
	}
	return append(candidates, filepath.Join("/usr/local/bin", name), filepath.Join("/usr/bin", name))
}
