# Docs Commands
# Run `just docs` to see all documentation commands
#
# Hero image generation uses the grok CLI Imagine tool, following
# technical_videos/docs/guides/generating-images-with-grok-cli.md.

set dotenv-load

PROJECT_DIR := justfile_directory()
HERO := "docs/images/hero.png"

[private]
@default:
    @echo "Docs Commands"
    @echo "============="
    @just --list --justfile {{ source_file() }}

# Regenerate the README hero still with grok Imagine (1280x720 PNG)
hero:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{ PROJECT_DIR }}"
    grok -p "Call your image_gen (Imagine) tool ONCE to create a 16:9 image (1280x720): a dim copper-and-ink workshop at night, photoreal material study. A mechanical drafting table holds a CRT terminal. On the screen a single blinking amber block caret. Fine brass wire traces spill off the table and loop through the air like a lasso around the terminal, suggesting a Go program wrapping a CLI. Charcoal background, dust motes in one overhead lamp, shallow depth of field. No people, no logos, no letters, no watermarks, no UI chrome, no brand marks. Then save the PNG to {{ HERO }} (scale to exactly 1280x720 with ffmpeg if needed, cover crop, no letterboxing) and print the absolute file path." \
      --always-approve --max-turns 30
    ls -la "{{ HERO }}"
    file "{{ HERO }}"
    ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=p=0 "{{ HERO }}"

# Verify the committed hero image exists and is 1280x720
hero-check:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{ PROJECT_DIR }}"
    test -f "{{ HERO }}"
    dims=$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=p=0 "{{ HERO }}")
    echo "{{ HERO }} ${dims}"
    test "${dims}" = "1280,720"
