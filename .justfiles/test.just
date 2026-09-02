# Test Commands
# Run `just test` to see all test commands

set dotenv-load

PROJECT_DIR := justfile_directory()

[private]
@default:
    @echo "Test Commands"
    @echo "============="
    @just --list --justfile {{ source_file() }}

# Run all unit tests (library + dangerous subpackage)
all: lib dangerous

# Run the complete local gate in an isolated Go container
container GO_VERSION="1.26":
    #!/usr/bin/env bash
    set -euo pipefail
    docker run --rm \
        -e GOFLAGS=-buildvcs=false \
        -v "{{ PROJECT_DIR }}:/src" \
        -w /src \
        "golang:{{ GO_VERSION }}-bookworm" \
        sh -c 'set -eu; test -z "$(gofmt -l .)"; go vet ./...; go test ./...; go test -race ./pkg/cursor/...; go build -o /tmp/examples-out/ ./examples/...'

# Run core library tests (all pkg/cursor packages, including subpackages)
lib:
    @echo "Testing core library..."
    cd {{ PROJECT_DIR }} && go test ./pkg/cursor/...
    @echo "Core library tests complete"

# Run dangerous package tests
dangerous:
    @echo "Testing dangerous package..."
    cd {{ PROJECT_DIR }} && go test ./pkg/cursor/dangerous
    @echo "Dangerous package tests complete"

# Run tests with race detection
race:
    @echo "Running tests with race detection..."
    cd {{ PROJECT_DIR }} && go test -race ./pkg/cursor/...
    @echo "Race detection tests complete"

# Run tests verbosely
verbose:
    cd {{ PROJECT_DIR }} && go test -v ./pkg/cursor/...

# Verify examples compile without dropping binaries in the repo
examples:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Compiling examples (build-only)..."
    out="$(mktemp -d)"
    trap 'rm -rf "$out"' EXIT
    cd "{{ PROJECT_DIR }}" && go build -o "$out/" ./examples/...
    echo "All examples compile"
