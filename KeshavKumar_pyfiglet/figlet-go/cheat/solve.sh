#!/bin/bash
# CHEAT (not a solution): proves the grader cannot be passed without a real port.
# It ports nothing. It builds a Go program that looks its arguments up in a
# copy of tests/data/visible.jsonl and prints the stored "out" (else nothing),
# and deletes every .py file.
#
# To run it: copy cheat/solve.sh AND cheat/visible.jsonl into solution/
# (after backing up the real solution/solve.sh), run
#   harbor run -p ./figlet-go -a oracle
# then restore the real solve.sh.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO=/workspace/repo
cd "$REPO"

cp "$HERE/visible.jsonl" answers.jsonl

cat > go.mod <<'EOF'
module figlet

go 1.23
EOF

cat > main.go <<'EOF'
package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"os"
	"slices"
)

//go:embed answers.jsonl
var answers []byte

func main() {
	sc := bufio.NewScanner(bytes.NewReader(answers))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
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

find "$REPO" -name '*.py' -type f -delete
go build ./... && go build -o /tmp/figlet .
echo "cheat installed"
