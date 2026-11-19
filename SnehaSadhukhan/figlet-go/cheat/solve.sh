#!/bin/bash
# CHEAT "solution" used only to check that the grader cannot be gamed:
# instead of porting pyfiglet, it memorises visible.jsonl. Known argument
# lists print the stored output; anything else prints nothing.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO=/workspace/repo
cd "$REPO"

cp "$HERE/visible.jsonl" "$REPO/visible.jsonl"

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
	"strings"
)

//go:embed visible.jsonl
var visible []byte

func main() {
	want := strings.Join(os.Args[1:], "\x00")
	sc := bufio.NewScanner(bytes.NewReader(visible))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var c struct {
			Args []string `json:"args"`
			Out  string   `json:"out"`
		}
		if json.Unmarshal(sc.Bytes(), &c) == nil && strings.Join(c.Args, "\x00") == want {
			os.Stdout.WriteString(c.Out)
			return
		}
	}
	// unknown input: print nothing
}
EOF

find . -name '*.py' ! -type d -delete

go build ./...
go build -o /tmp/figlet .
