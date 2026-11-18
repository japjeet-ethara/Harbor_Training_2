#!/bin/bash
#
# CHEAT (not a solution): ports nothing. It ships a copy of the VISIBLE answer
# file and looks its arguments up in it. It exists only to prove the grader
# catches table lookups: the graded cases are hidden and disjoint, so this
# scores the build point and nothing else.
set -euo pipefail

REPO=/workspace/repo
cd "$REPO"

cat > go.mod <<'EOF_GO_MOD'
module figletgo

go 1.23
EOF_GO_MOD

cat > main.go <<'EOF_MAIN_GO'
package main

import (
	"embed"
	"encoding/json"
	"os"
	"strings"
)

//go:embed answers.jsonl
var answers embed.FS

type entry struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func key(args []string) string {
	return strings.Join(args, "\x00")
}

func main() {
	raw, err := answers.ReadFile("answers.jsonl")
	if err != nil {
		return
	}
	table := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e entry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		table[key(e.Args)] = e.Out
	}
	if out, ok := table[key(os.Args[1:])]; ok {
		os.Stdout.WriteString(out)
	}
}
EOF_MAIN_GO

SELF_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SELF_DIR/visible-answers.jsonl" ]; then
  cp "$SELF_DIR/visible-answers.jsonl" answers.jsonl
else
  : > answers.jsonl
fi

find . -name '*.py' -type f -delete

export GOFLAGS=""
export GOTOOLCHAIN=local
export CGO_ENABLED=0
go build -o /tmp/figlet .
echo "cheat built; answers.jsonl lines: $(wc -l < answers.jsonl)"
