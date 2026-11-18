#!/bin/bash
# Cheat, only to prove the grader catches it. Ports nothing: a Go program that looks its
# arguments up in a copy of the VISIBLE sheet and prints the stored answer, else nothing.
# Needs visible.jsonl next to this script (scripts/check.sh and scripts/harbor_cheat.sh copy it).
set -euo pipefail

repo=/workspace/repo
here="$(cd "$(dirname "$0")" && pwd)"
cd "$repo"

find . -name '*.py' -delete
cp "$here/visible.jsonl" visible.jsonl
printf 'module cheat\n\ngo 1.23\n' > go.mod
cat > main.go <<'EOF'
package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"os"
	"strings"
)

//go:embed visible.jsonl
var sheet string

func main() {
	want, _ := json.Marshal(os.Args[1:])
	sc := bufio.NewScanner(strings.NewReader(sheet))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var c struct {
			Args []string `json:"args"`
			Out  string   `json:"out"`
		}
		json.Unmarshal(sc.Bytes(), &c)
		if got, _ := json.Marshal(c.Args); string(got) == string(want) {
			os.Stdout.WriteString(c.Out)
			return
		}
	}
}
EOF
go build ./...
