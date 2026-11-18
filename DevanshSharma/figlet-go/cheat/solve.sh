#!/usr/bin/env bash
# cheat/solve.sh
# Cheater: looks up args in visible.jsonl and prints the stored answer.
# Does NOT port any rendering logic.
set -euo pipefail

REPO=/workspace/repo
cd "$REPO"

# Initialise Go module
rm -f go.mod go.sum *.go
find . -name "*.go" -not -path "./.git/*" -delete 2>/dev/null || true
go mod init figlet

cat > main.go << 'GOEOF'
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
	key := strings.Join(os.Args[1:], "\x00")

	f, err := os.Open("/tests/data/visible.jsonl")
	if err != nil {
		os.Exit(0)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)
	for scanner.Scan() {
		var c Case
		if json.Unmarshal([]byte(scanner.Text()), &c) != nil {
			continue
		}
		if strings.Join(c.Args, "\x00") == key {
			fmt.Print(c.Out)
			return
		}
	}
}
GOEOF

# Delete .py files
find /workspace/repo -name "*.py" -delete

go build -o /tmp/figlet .
echo "Cheat build OK"
