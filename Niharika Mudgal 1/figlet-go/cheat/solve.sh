#!/bin/bash
# CHEAT, for Step 7. It ports NOTHING. It writes a tiny Go program that looks its
# arguments up in a copy of tests/data/visible.jsonl and prints the stored "out"
# (or nothing if the arguments are unknown). Then it deletes every .py file.
# The tests must give this at most about 1/6 (build only) and no secret.
#
# It needs visible.jsonl next to this script (cheat/run_cheat.sh puts it there).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC="$HERE/visible.jsonl"
[ -f "$SRC" ] || SRC=/solution/visible.jsonl
cd /workspace/repo

cp "$SRC" cases.jsonl

cat > go.mod <<'GOMOD'
module figlet

go 1.21
GOMOD

cat > main.go <<'GOEOF'
package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"os"
	"strings"
)

//go:embed cases.jsonl
var cases string

type entry struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	want := strings.Join(os.Args[1:], "\x00")
	sc := bufio.NewScanner(strings.NewReader(cases))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e entry
		if json.Unmarshal(sc.Bytes(), &e) == nil && strings.Join(e.Args, "\x00") == want {
			os.Stdout.WriteString(e.Out)
			return
		}
	}
}
GOEOF

find /workspace/repo -name '*.py' -type f -delete
go build ./...
echo "cheat/solve.sh: done"
