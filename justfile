# cursor-agent-go - Modular Justfile
# Run `just` to see all available commands

set dotenv-load

mod build '.justfiles/build.just'
mod test '.justfiles/test.just'
mod coverage '.justfiles/coverage.just'
mod mock '.justfiles/mock.just'
mod util '.justfiles/util.just'
mod release '.justfiles/releases.just'
mod docs '.justfiles/docs.just'

PROJECT := "cursor-agent-go"
BIN_DIR := "./bin"
COVERAGE_DIR := "./coverage"

[private]
@default:
    @echo "cursor-agent-go"
    @echo "==============="
    @echo ""
    @just --list --unsorted

# Full pipeline: clean -> deps -> build -> test -> coverage
all: clean deps build-all test-all coverage-report

# Clean build artifacts
clean:
    just util clean

# Download dependencies
deps:
    just util deps

# Tidy all modules
tidy-all:
    just util tidy-all

# Run linters (gofmt + go vet)
lint:
    just util lint

# Generate coverage report
coverage-report:
    just coverage report

# Run the release gate in both supported Go versions
gate:
    just test container 1.24
    just test container 1.26

[private]
build-all:
    just build all

[private]
test-all:
    just test all
