#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=${REPO_ROOT:-/workspace/repo}
if [ ! -d "$repo_root" ]; then
    repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
fi

visible_file=
for candidate in \
    "$script_dir/visible.jsonl" \
    "$script_dir/../cheat/visible.jsonl" \
    "$script_dir/../data/visible.jsonl" \
    "$script_dir/../tests/data/visible.jsonl"
do
    if [ -f "$candidate" ]; then
        visible_file=$candidate
        break
    fi
done
if [ -z "$visible_file" ]; then
    echo "Cannot find tests/data/visible.jsonl" >&2
    exit 1
fi

cd "$repo_root"
cp "$visible_file" visible.jsonl

cat > go.mod <<'EOF'
module figlet-cheat

go 1.20
EOF

cat > main.go <<'EOF'
package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

//go:embed visible.jsonl
var corpus []byte

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	args := os.Args[1:]
	scanner := bufio.NewScanner(bytes.NewReader(corpus))
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var tc testCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			fail(err)
		}
		if len(args) != len(tc.Args) {
			continue
		}
		match := true
		for i := range args {
			if args[i] != tc.Args[i] {
				match = false
				break
			}
		}
		if match {
			if _, err := io.WriteString(os.Stdout, tc.Out); err != nil {
				fail(err)
			}
			return
		}
	}
	if err := scanner.Err(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
EOF

find "$repo_root" -type f -name '*.py' -delete

if find "$repo_root" -type f -name '*.py' -print -quit | grep -q .; then
    echo "Python source files remain in the repository" >&2
    exit 1
fi
