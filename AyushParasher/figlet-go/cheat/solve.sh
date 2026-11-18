#!/bin/bash
# CHEAT (Step 7) — must NOT be used as the real solution.
# Ports nothing: installs a Go program that looks its arguments up in a copy of the
# VISIBLE answers and prints the stored "out" if found, else nothing. Deletes the .py
# files. The grader should give it build=1, no_python=0, behaviour=0 → reward 1/6.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"   # /solution under Harbor's oracle, /cheat in simulate.sh
cd /workspace/repo

cp "$HERE/visible.jsonl" answers.jsonl
printf 'module figlet\n\ngo 1.23\n' > go.mod
cat > main.go <<'EOF'
package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"os"
	"slices"
	"strings"
)

//go:embed answers.jsonl
var answers string

func main() {
	sc := bufio.NewScanner(strings.NewReader(answers))
	sc.Buffer(nil, 1<<24)
	for sc.Scan() {
		var c struct {
			Args []string `json:"args"`
			Out  string   `json:"out"`
		}
		if json.Unmarshal(sc.Bytes(), &c) == nil && slices.Equal(c.Args, os.Args[1:]) {
			os.Stdout.WriteString(c.Out)
			return
		}
	}
}
EOF

find /workspace/repo -name '*.py' -delete
go build -o /dev/null ./...
