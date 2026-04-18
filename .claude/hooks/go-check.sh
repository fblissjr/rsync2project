#!/usr/bin/env bash
# PostToolUse hook: on .go file edits, run go vet + go build.
# Non-zero exit surfaces compile/vet errors to Claude so it fixes them
# before moving on. Runs in under a second on this codebase.
#
# stdin: { "tool_input": { "file_path": "..." }, ... }

set -euo pipefail

input=$(cat)
path=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')

case "$path" in
  *.go) ;;
  *) exit 0 ;;
esac

cd "${CLAUDE_PROJECT_DIR:-.}"

if ! go build ./... 2>&1; then
  echo "go build failed after edit to $path" >&2
  exit 2
fi
if ! go vet ./... 2>&1; then
  echo "go vet failed after edit to $path" >&2
  exit 2
fi
exit 0
