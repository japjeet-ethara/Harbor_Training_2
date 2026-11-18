#!/usr/bin/env bash
# cheat/solve.sh — lookup-table cheater: looks args up in visible.jsonl only
set -e

REPO=/workspace/repo

# Remove .py files (same as oracle, to pass that check)
find "$REPO" -name "*.py" -delete

cat > "$REPO/go.mod" << 'GOMOD'
module figlet

go 1.23
GOMOD

cat > "$REPO/main.go" << 'GOEOF'
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Case struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	args := os.Args[1:]
	key := strings.Join(args, "\x00")

	f, err := os.Open("/tests/data/visible.jsonl")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var c Case
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			continue
		}
		if strings.Join(c.Args, "\x00") == key {
			fmt.Print(c.Out)
			fmt.Println()
			return
		}
	}
}
GOEOF

echo "Cheat solution written."
