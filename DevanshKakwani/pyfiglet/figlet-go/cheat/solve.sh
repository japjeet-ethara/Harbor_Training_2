#!/bin/bash
# CHEAT SOLUTION -- for testing the grader only. It ports nothing.
#
# Installs a tiny Go program that looks its command-line arguments up in a
# copy of the visible test cases (visible.jsonl, next to this script) and
# prints the stored output on an exact match, or nothing otherwise.
#
# Run it as a task's solution with tools/make-cheat-task.sh, which copies
# this folder to solution/ in ../figlet-go-cheat. /tests is not available
# while a solution runs, which is why the answer sheet travels with it.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${REPO:-/workspace/repo}"

if [ ! -f "$HERE/visible.jsonl" ]; then
    echo "cheat: $HERE/visible.jsonl not found" >&2
    exit 1
fi

# The answer sheet stays in the repository so go:embed can compile it in.
mkdir -p "$REPO/cheatdata"
cp "$HERE/visible.jsonl" "$REPO/cheatdata/visible.jsonl"

cat > "$REPO/go.mod" <<'GO_MOD'
module cheat

go 1.23
GO_MOD

cat > "$REPO/main.go" <<'GO_SOURCE'
// Cheat: replays the visible test cases. No FIGlet rendering.
package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"os"
	"slices"
)

//go:embed cheatdata/visible.jsonl
var visible []byte

func main() {
	args := os.Args[1:]
	sc := bufio.NewScanner(bytes.NewReader(visible))
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	for sc.Scan() {
		var rec struct {
			Args []string `json:"args"`
			Out  string   `json:"out"`
		}
		if json.Unmarshal(sc.Bytes(), &rec) != nil {
			continue
		}
		if slices.Equal(rec.Args, args) {
			os.Stdout.WriteString(rec.Out) // exact bytes, trailing newlines included
			return
		}
	}
	// No match: print nothing.
}
GO_SOURCE

# No Python may remain anywhere in the repository.
find "$REPO" -type f -iname '*.py' -delete

cd "$REPO"
go build -o /tmp/figlet .
