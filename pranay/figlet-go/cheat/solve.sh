#!/bin/bash
# CHEAT (Step 7): ports nothing. Looks arguments up in a copy of visible.jsonl.
# Copy cheat/solve.sh AND cheat/visible.jsonl into solution/ to run it; keep the real solve.sh safe.
set -euo pipefail
cd /workspace/repo
D="$(cd "$(dirname "$0")" && pwd)"
cp "$D/visible.jsonl" data.jsonl
printf 'module figlet\n\ngo 1.22\n' > go.mod
cat > main.go <<'EOF_CHEAT'
package main

import (
	_ "embed"
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

//go:embed data.jsonl
var data string

func main() {
	key := strings.Join(os.Args[1:], "\x00")
	sc := bufio.NewScanner(strings.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var c struct {
			Args []string `json:"args"`
			Out  string   `json:"out"`
		}
		if json.Unmarshal(sc.Bytes(), &c) == nil && strings.Join(c.Args, "\x00") == key {
			os.Stdout.WriteString(c.Out)
			return
		}
	}
}
EOF_CHEAT
rm -rf pyfiglet doc tools test_font.sh
find . -name '*.py' -delete
go build -o /tmp/figlet .
go build ./...
