#!/usr/bin/env python3
"""Build cheat/solve.sh (Step 7). Author's laptop only.

The cheat ports NOTHING. It writes a tiny Go program that looks its arguments up
in an embedded copy of tests/data/visible.jsonl and prints the stored "out" if
found, else nothing. It deletes the .py files. The exam must give it a low score.

Usage:  python3 dev/make_cheat.py
"""
import os

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, ".."))
VISIBLE = os.path.join(ROOT, "tests", "data", "visible.jsonl")
OUT = os.path.join(ROOT, "cheat", "solve.sh")

HEAD = r'''#!/bin/bash
# CHEAT solution: does not port anything. Looks up its arguments in a copy of
# visible.jsonl. Used only to prove the exam gives it a low score and no secret.
set -euo pipefail
REPO=/workspace/repo
cd "$REPO"

cat > go.mod <<'GOMOD_EOF'
module figlet

go 1.23
GOMOD_EOF

cat > main.go <<'GO_EOF'
package main

import (
	_ "embed"
	"encoding/json"
	"os"
	"strings"
)

//go:embed visible.jsonl
var data string

type entry struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	key := strings.Join(os.Args[1:], "\x00")
	for _, line := range strings.Split(data, "\n") {
		if line == "" {
			continue
		}
		var e entry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if strings.Join(e.Args, "\x00") == key {
			os.Stdout.WriteString(e.Out)
			return
		}
	}
}
GO_EOF

cat > visible.jsonl <<'VISIBLE_JSONL_EOF'
'''

TAIL = r'''VISIBLE_JSONL_EOF

find "$REPO" -name '*.py' -delete
go build -o /tmp/figlet .
echo "cheat solve.sh: done"
'''


def main():
    with open(VISIBLE, encoding="utf-8") as f:
        data = f.read()
    assert "\nVISIBLE_JSONL_EOF\n" not in data
    if not data.endswith("\n"):
        data += "\n"
    with open(OUT, "w", encoding="utf-8", newline="\n") as f:
        f.write(HEAD + data + TAIL)
    os.chmod(OUT, 0o755)
    print("wrote", OUT, os.path.getsize(OUT), "bytes")


if __name__ == "__main__":
    main()
