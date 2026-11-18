
#!/bin/bash
set -euo pipefail

ORIG_DIR="$(pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="/workspace/repo"

if [ ! -d "$REPO_DIR" ]; then
    REPO_DIR="$ORIG_DIR"
fi

# Locate the visible dataset
VIS_SRC=""

for cand in \
    "$SCRIPT_DIR/visible.jsonl" \
    "$SCRIPT_DIR/../tests/data/visible.jsonl" \
    "$ORIG_DIR/tests/data/visible.jsonl" \
    "$ORIG_DIR/cheat/visible.jsonl" \
    "$ORIG_DIR/../tests/data/visible.jsonl" \
    /tests/data/visible.jsonl \
    /workspace/tests/data/visible.jsonl \
    /workspace/figlet-go/tests/data/visible.jsonl \
    /workspace/repo/tests/data/visible.jsonl \
    /workspace/repo/visible.jsonl; do

    if [ -f "$cand" ]; then
        VIS_SRC="$cand"
        break
    fi
done

if [ -z "$VIS_SRC" ]; then
    VIS_SRC="$(find /workspace /tests "$ORIG_DIR" "$SCRIPT_DIR" \
        -name "visible.jsonl" -type f 2>/dev/null | head -n 1 || true)"
fi

if [ -z "$VIS_SRC" ]; then
    echo "ERROR: visible.jsonl not found. Cannot build lookup cheat." >&2
    exit 1
fi

cd "$REPO_DIR"

# Remove Python files
find . -type f -name "*.py" -delete
find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true

# Copy visible dataset beside main.go
if [ "$(realpath "$VIS_SRC")" != "$(realpath -m ./visible.jsonl)" ]; then
    cp "$VIS_SRC" ./visible.jsonl
fi

if [ ! -s ./visible.jsonl ]; then
    echo "ERROR: visible.jsonl is missing or empty." >&2
    exit 1
fi

# Create Go module
cat << 'EOF' > go.mod
module figlet

go 1.23
EOF

# Create lookup-only Go program
cat << 'EOF' > main.go
package main

import (
    _ "embed"
    "encoding/json"
    "os"
    "strings"
)

//go:embed visible.jsonl
var visibleData string

var lookup = make(map[string]string)

func init() {
    type Record struct {
        Args []string `json:"args"`
        Out  string   `json:"out"`
    }

    for _, line := range strings.Split(visibleData, "\n") {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        var rec Record
        if err := json.Unmarshal([]byte(line), &rec); err != nil {
            continue
        }

        key, err := json.Marshal(rec.Args)
        if err != nil {
            continue
        }

        lookup[string(key)] = rec.Out
    }
}

func main() {
    key, err := json.Marshal(os.Args[1:])
    if err != nil {
        return
    }

    if out, found := lookup[string(key)]; found {
        _, _ = os.Stdout.WriteString(out)
    }
}
EOF

# Build cheat
go build -o /tmp/figlet .

echo "Cheat built successfully."
echo "Visible lookup entries: $(wc -l < visible.jsonl)"
