# Build Commands
# Run `just build` to see all build commands

PROJECT_DIR := justfile_directory()
BIN_DIR := PROJECT_DIR / "bin"

[private]
@default:
    @echo "Build Commands"
    @echo "=============="
    @just --list --justfile {{ source_file() }}

# Build everything (library + dangerous + examples + mock)
all: lib dangerous examples mock

# Build core library
lib:
    @echo "Building core library..."
    cd {{ PROJECT_DIR }} && go build ./pkg/cursor ./pkg/cursor/cloud
    @echo "Core library built"

# Build dangerous subpackage
dangerous:
    @echo "Building dangerous subpackage..."
    cd {{ PROJECT_DIR }} && go build ./pkg/cursor/dangerous
    @echo "Dangerous subpackage built"

# Build the mock cursor-agent binary used by unit tests
mock:
    @echo "Building mock cursor-agent binary..."
    @mkdir -p {{ PROJECT_DIR }}/test/mockagent/bin
    cd {{ PROJECT_DIR }} && go build -o test/mockagent/bin/cursor-agent-mock ./test/mockagent
    @echo "Built: test/mockagent/bin/cursor-agent-mock"

# Build all examples
examples: ask
    @echo "All examples built"

# Build ask example
ask:
    @mkdir -p {{ BIN_DIR }}
    cd {{ PROJECT_DIR }} && go build -o {{ BIN_DIR }}/ask ./examples/ask
    @echo "Built: {{ BIN_DIR }}/ask"
