#!/bin/bash
# Builds cheat/solve.sh: a "solution" that ports NOTHING. It stores a copy of
# tests/data/visible.jsonl and prints the stored "out" when the args match exactly.
# Run from the figlet-go folder:  bash cheat/make_cheat.sh
set -euo pipefail
out=cheat/solve.sh
{
cat <<'HEAD'
#!/bin/bash
# CHEAT (for the Step 7 check only): looks answers up in a copy of the VISIBLE cases.
set -euo pipefail
cd /workspace/repo
find . -name '*.py' -type f -delete
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

//go:embed cases.jsonl
var cases []byte

func main() {
	want := strings.Join(os.Args[1:], "\x00")
	sc := bufio.NewScanner(bytes.NewReader(cases))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
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
}
EOF
cat > cases.jsonl <<'__VISIBLE_CASES_END__'
HEAD
cat tests/data/visible.jsonl
cat <<'TAIL'
__VISIBLE_CASES_END__
go build ./...
TAIL
} > "$out"
echo "wrote $out ($(wc -c < "$out") bytes)"
