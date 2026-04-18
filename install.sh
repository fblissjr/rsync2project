#!/usr/bin/env bash
# Build rsync2project and install the binary to a suitable location on PATH.
# Works on macOS (arm64/amd64) and Linux (amd64/arm64). No arguments.
#
# Install location priority:
#   1. $HOME/.local/bin  (if it exists and is on PATH)
#   2. $HOME/bin         (if it exists and is on PATH)
#   3. /usr/local/bin    (system, will use sudo if needed)
#
# Requires: go (>=1.21), a shell with bash, write access to one of the above.

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_DIR"

OS="$(uname -s)"
ARCH="$(uname -m)"
case "$OS" in
    Darwin) PLATFORM="macOS ($ARCH)" ;;
    Linux)  PLATFORM="Linux ($ARCH)" ;;
    *)      echo "unsupported OS: $OS" >&2 ; exit 1 ;;
esac

echo "Platform: $PLATFORM"

if ! command -v go >/dev/null 2>&1; then
    echo "go not found on PATH. Install Go first (https://go.dev/dl/)." >&2
    exit 1
fi
echo "Go: $(go version)"

echo "Building..."
go build -o rsync2project .
echo "Built:    $REPO_DIR/rsync2project ($(wc -c <rsync2project | tr -d ' ') bytes)"

# Pick install dest: first candidate that exists AND is on PATH.
on_path() {
    case ":$PATH:" in
        *":$1:"*) return 0 ;;
        *)        return 1 ;;
    esac
}

DEST=""
for candidate in "$HOME/.local/bin" "$HOME/bin"; do
    if [ -d "$candidate" ] && on_path "$candidate"; then
        DEST="$candidate"
        break
    fi
done

if [ -z "$DEST" ]; then
    DEST="/usr/local/bin"
    echo "No user-local bin dir found on PATH; falling back to $DEST (may require sudo)."
fi

if [ -w "$DEST" ]; then
    cp rsync2project "$DEST/rsync2project"
else
    echo "Using sudo to install into $DEST"
    sudo cp rsync2project "$DEST/rsync2project"
fi

echo "Installed: $DEST/rsync2project"

# Confirm what a fresh shell would resolve.
if command -v rsync2project >/dev/null 2>&1; then
    RESOLVED="$(command -v rsync2project)"
    if [ "$RESOLVED" != "$DEST/rsync2project" ]; then
        echo "Warning: 'rsync2project' resolves to $RESOLVED, not the one just installed."
        echo "         Another binary or shell function is shadowing it."
    fi
    "$DEST/rsync2project" --version
else
    echo "Warning: '$DEST' is not on PATH in this shell. Add it or re-open your terminal."
fi
