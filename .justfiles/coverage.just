# Coverage Commands
# Run `just coverage` to see all coverage commands

set dotenv-load

PROJECT_DIR := justfile_directory()
COVERAGE_DIR := PROJECT_DIR / "coverage"

[private]
@default:
    @echo "Coverage Commands"
    @echo "================="
    @just --list --justfile {{ source_file() }}

# Generate coverage data
run:
    @echo "Generating coverage data..."
    @mkdir -p {{ COVERAGE_DIR }}
    cd {{ PROJECT_DIR }} && go test -coverprofile={{ COVERAGE_DIR }}/coverage.out ./pkg/cursor/...
    @echo "Coverage data: {{ COVERAGE_DIR }}/coverage.out"

# Generate HTML coverage report
report: run
    @echo "Generating HTML coverage report..."
    cd {{ PROJECT_DIR }} && go tool cover -html={{ COVERAGE_DIR }}/coverage.out -o {{ COVERAGE_DIR }}/coverage.html
    @echo "Coverage report: {{ COVERAGE_DIR }}/coverage.html"

# Show coverage percentage
percent: run
    @echo "Coverage percentage:"
    cd {{ PROJECT_DIR }} && go tool cover -func={{ COVERAGE_DIR }}/coverage.out | tail -1

# Clean coverage files
clean:
    @echo "Cleaning coverage files..."
    rm -rf {{ COVERAGE_DIR }}
    @echo "Coverage files cleaned"
