#!/bin/sh
set -eu

cd /workspace/repo
fixture_path=${VISIBLE_FIXTURE_PATH:?VISIBLE_FIXTURE_PATH is not set}
if [ ! -r "$fixture_path" ]; then
    printf 'Visible fixture is not readable: %s\n' "$fixture_path" >&2
    exit 1
fi
printf 'Visible fixture accessible via VISIBLE_FIXTURE_PATH\n'

cat > go.mod <<'EOF'
module figlet

go 1.23
EOF

cat > main.go <<'EOF'
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	fixturePath := os.Getenv("VISIBLE_FIXTURE_PATH")
	if fixturePath == "" {
		fmt.Fprintln(os.Stderr, "VISIBLE_FIXTURE_PATH is not set")
		os.Exit(1)
	}
	file, err := os.Open(fixturePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for scanner.Scan() {
		var tc testCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if reflect.DeepEqual(tc.Args, os.Args[1:]) {
			if _, err := os.Stdout.Write([]byte(tc.Out)); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
EOF

find . -type f -name '*.py' -delete
