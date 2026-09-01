# Release Commands
# Run `just release` to see all release commands

set dotenv-load

PROJECT_DIR := justfile_directory()

[private]
@default:
    @echo "Release Commands"
    @echo "================"
    @just --list --justfile {{ source_file() }}

# Show current version from latest git tag
version:
    @echo "Current version:"
    @git -C {{ PROJECT_DIR }} describe --tags --abbrev=0 2>/dev/null || echo "No tags found"

# Show commits since last release
changelog:
    #!/usr/bin/env bash
    cd {{ PROJECT_DIR }}
    LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || true)
    if [ -z "$LAST_TAG" ]; then
        echo "No previous tags found. Showing all commits:"
        git log --oneline
    else
        echo "Changes since $LAST_TAG:"
        git log --oneline "$LAST_TAG"..HEAD
    fi

# Show release readiness status
status:
    #!/usr/bin/env bash
    cd {{ PROJECT_DIR }}
    echo "=== Release Status ==="
    echo ""
    echo "Branch: $(git branch --show-current)"
    echo "Latest tag: $(git describe --tags --abbrev=0 2>/dev/null || echo 'none')"
    echo ""
    echo "Uncommitted changes:"
    if git diff --quiet && git diff --staged --quiet; then
        echo "  None (working tree clean)"
    else
        git status --short
    fi

# Run all pre-release checks (lint, test, build)
check:
    @echo "=== Pre-release Checks ==="
    just gate
    @echo ""
    @echo "All checks passed"

# Create annotated git tag
tag VERSION:
    @echo "Creating tag {{ VERSION }}..."
    git -C {{ PROJECT_DIR }} tag -a {{ VERSION }} -m "Release {{ VERSION }}"
    @echo "Tag {{ VERSION }} created"
    @echo "Run 'just release push' to push to remote"

# Push tags to remote
push:
    @echo "Pushing tags to remote..."
    git -C {{ PROJECT_DIR }} push --tags
    @echo "Tags pushed"

# Full release prep: check + tag
prepare VERSION: check
    just release tag {{ VERSION }}

# Generate release notes template
notes VERSION:
    #!/usr/bin/env bash
    cd {{ PROJECT_DIR }}
    LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || true)
    echo "# Release {{ VERSION }}"
    echo ""
    echo "## What's Changed"
    echo ""
    if [ -z "$LAST_TAG" ]; then
        git log --oneline --pretty=format:"- %s" | head -20
    else
        git log --oneline --pretty=format:"- %s" "$LAST_TAG"..HEAD
    fi
    echo ""
