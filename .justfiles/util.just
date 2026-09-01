# Utility Commands
# Run `just util` to see all utility commands

set dotenv-load

PROJECT_DIR := justfile_directory()
BIN_DIR := PROJECT_DIR / "bin"
COVERAGE_DIR := PROJECT_DIR / "coverage"

[private]
@default:
    @echo "Utility Commands"
    @echo "================"
    @just --list --justfile {{ source_file() }}

# Clean build artifacts
clean:
    @echo "Cleaning build artifacts..."
    rm -rf {{ BIN_DIR }}
    rm -rf {{ COVERAGE_DIR }}
    rm -rf {{ PROJECT_DIR }}/test/mockagent/bin
    @echo "Clean complete"

# Download and verify dependencies
deps:
    @echo "Downloading dependencies..."
    cd {{ PROJECT_DIR }} && go mod download
    @echo "Dependencies ready"

# Tidy dependencies
tidy:
    @echo "Tidying dependencies..."
    cd {{ PROJECT_DIR }} && go mod tidy
    @echo "Dependencies tidied"

# Tidy all modules
tidy-all: tidy

# Run linters (gofmt check + go vet)
lint:
    #!/usr/bin/env bash
    set -euo pipefail
    cd {{ PROJECT_DIR }}
    echo "Checking gofmt..."
    unformatted=$(gofmt -l . || true)
    if [ -n "$unformatted" ]; then
        echo "Files need gofmt:"
        echo "$unformatted"
        exit 1
    fi
    echo "gofmt clean"
    echo "Running go vet..."
    go vet ./...
    echo "go vet clean"
    echo "Linting complete"

# Format sources
fmt:
    cd {{ PROJECT_DIR }} && gofmt -w .

# Fix common issues (format + tidy)
fix: fmt tidy
    @echo "Fixes applied"

# Show version info
version:
    @echo "Go version:"
    @go version
    @echo ""
    @echo "Module info:"
    @cd {{ PROJECT_DIR }} && go list -m

# Check the installed cursor-agent CLI
check-cursor-agent:
    #!/usr/bin/env bash
    set -euo pipefail
    if ! command -v cursor-agent &> /dev/null; then
        echo "cursor-agent not found in PATH"
        exit 1
    fi
    echo "cursor-agent found: $(command -v cursor-agent)"
    cursor-agent --version
