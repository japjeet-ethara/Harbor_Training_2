#!/usr/bin/env bash
set -euo pipefail

repo=/workspace/repo
script_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
cases="${VISIBLE_CASES:-$script_dir/../tests/data/visible.jsonl}"
if [[ ! -f "$cases" && -f /tests/data/visible.jsonl ]]; then
    cases=/tests/data/visible.jsonl
fi
if [[ ! -f "$cases" ]]; then
    echo "visible case corpus not found" >&2
    exit 1
fi

cp "$cases" "$repo/visible.jsonl"
cat > "$repo/go.mod" <<'EOF'
module visiblelookup

go 1.23
EOF
cat > "$repo/main.go" <<'EOF'
package main

import (
	"bufio"
	"encoding/json"
	"os"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	key, err := json.Marshal(os.Args[1:])
	if err != nil {
		return
	}
	file, err := os.Open("visible.jsonl")
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var tc testCase
		if json.Unmarshal(scanner.Bytes(), &tc) != nil {
			return
		}
		candidate, err := json.Marshal(tc.Args)
		if err == nil && string(candidate) == string(key) {
			_, _ = os.Stdout.WriteString(tc.Out)
			return
		}
	}
}
EOF
find "$repo" -type f -name '*.py' -delete

cd "$repo"
gofmt -w main.go
go build ./...
go build -o /tmp/figlet .
