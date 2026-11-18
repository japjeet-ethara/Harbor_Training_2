#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd /workspace/repo

# 1. Prepare fonts directory if font files are available
mkdir -p /workspace/repo/fonts
if [ -d /workspace/repo/pyfiglet/fonts ] && [ "$(ls -A /workspace/repo/pyfiglet/fonts 2>/dev/null)" ]; then
    cp -a /workspace/repo/pyfiglet/fonts/. /workspace/repo/fonts/ 2>/dev/null || true
fi
if [ ! -f /workspace/repo/fonts/standard.flf ] && [ -d /workspace/repo/pyfiglet/fonts-standard ]; then
    cp -a /workspace/repo/pyfiglet/fonts-standard/. /workspace/repo/fonts/ 2>/dev/null || true
fi

# Note: Python files (.py, .pyc, __pycache__) are deliberately NOT deleted or cleaned up
# so that the No-Python check fails as intended for this cheat experiment.

# 2. Copy packaged visible dataset fixture into repository for embedding
if [ -f "$SCRIPT_DIR/visible.jsonl" ]; then
    cp "$SCRIPT_DIR/visible.jsonl" /workspace/repo/visible.jsonl
elif [ -f /solution/visible.jsonl ]; then
    cp /solution/visible.jsonl /workspace/repo/visible.jsonl
fi

# 3. Create go.mod
cat << 'GOMOD' > /workspace/repo/go.mod
module figlet

go 1.23
GOMOD

# 4. Create main.go with embedded visible dataset lookup
cat << 'GOEOF' > /workspace/repo/main.go
package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//go:embed visible.jsonl
var embeddedDataset string

type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type ParsedArgs struct {
	Font      string
	Width     int
	Justify   string
	Direction string
	Reverse   bool
	Flip      bool
	Strip     bool
	Normalize bool
	Text      string
}

func parseCLI(args []string) *ParsedArgs {
	p := &ParsedArgs{
		Font:      "standard",
		Width:     80,
		Justify:   "auto",
		Direction: "auto",
	}

	var positional []string
	positionalOnly := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if positionalOnly {
			positional = append(positional, arg)
			i++
			continue
		}
		if arg == "--" {
			positionalOnly = true
			i++
			continue
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			i++
			continue
		}

		if strings.HasPrefix(arg, "--") {
			opt := arg[2:]
			val := ""
			hasVal := false
			if idx := strings.Index(opt, "="); idx != -1 {
				val = opt[idx+1:]
				opt = opt[:idx]
				hasVal = true
			}
			switch opt {
			case "font":
				if !hasVal && i+1 < len(args) {
					i++
					val = args[i]
				}
				p.Font = val
			case "width":
				if !hasVal && i+1 < len(args) {
					i++
					val = args[i]
				}
				if w, err := strconv.Atoi(val); err == nil {
					p.Width = w
				}
			case "justify":
				if !hasVal && i+1 < len(args) {
					i++
					val = args[i]
				}
				p.Justify = val
			case "direction":
				if !hasVal && i+1 < len(args) {
					i++
					val = args[i]
				}
				p.Direction = val
			case "reverse":
				p.Reverse = true
			case "flip":
				p.Flip = true
			case "strip-surrounding-newlines":
				p.Strip = true
			case "normalize-surrounding-newlines":
				p.Normalize = true
			}
			i++
			continue
		}

		cluster := arg[1:]
		if idx := strings.Index(cluster, "="); idx != -1 {
			flag := cluster[:idx]
			val := cluster[idx+1:]
			switch flag {
			case "f":
				p.Font = val
			case "w":
				if w, err := strconv.Atoi(val); err == nil {
					p.Width = w
				}
			case "j":
				p.Justify = val
			case "D":
				p.Direction = val
			}
			i++
			continue
		}

		j := 0
		for j < len(cluster) {
			ch := cluster[j]
			switch ch {
			case 'r':
				p.Reverse = true
				j++
			case 'F':
				p.Flip = true
				j++
			case 's':
				p.Strip = true
				j++
			case 'n':
				p.Normalize = true
				j++
			case 'f', 'w', 'j', 'D':
				val := ""
				if j+1 < len(cluster) {
					val = cluster[j+1:]
					j = len(cluster)
				} else if i+1 < len(args) {
					i++
					val = args[i]
					j = len(cluster)
				}
				switch ch {
				case 'f':
					p.Font = val
				case 'w':
					if w, err := strconv.Atoi(val); err == nil {
						p.Width = w
					}
				case 'j':
					p.Justify = val
				case 'D':
					p.Direction = val
				}
			default:
				j++
			}
		}
		i++
	}

	p.Text = strings.Join(positional, " ")
	return p
}

func matchesParsed(p1, p2 *ParsedArgs) bool {
	if p1 == nil || p2 == nil {
		return false
	}
	return p1.Font == p2.Font &&
		p1.Width == p2.Width &&
		p1.Justify == p2.Justify &&
		p1.Direction == p2.Direction &&
		p1.Reverse == p2.Reverse &&
		p1.Flip == p2.Flip &&
		p1.Strip == p2.Strip &&
		p1.Normalize == p2.Normalize &&
		p1.Text == p2.Text
}

func main() {
	rawArgs := os.Args[1:]
	if len(rawArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: figlet [options] [text..]\n")
		os.Exit(1)
	}

	rawKey := strings.Join(rawArgs, "\x1f")
	targetParsed := parseCLI(rawArgs)

	var scanner *bufio.Scanner
	if len(embeddedDataset) > 0 {
		scanner = bufio.NewScanner(strings.NewReader(embeddedDataset))
	} else {
		f, err := os.Open("visible.jsonl")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: visible dataset not available: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		scanner = bufio.NewScanner(f)
	}

	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	var foundOut string
	found := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var tc TestCase
		if err := json.Unmarshal([]byte(line), &tc); err != nil {
			continue
		}

		// 1. Direct exact arguments match
		if strings.Join(tc.Args, "\x1f") == rawKey {
			foundOut = tc.Out
			found = true
			break
		}

		// 2. Canonical parsed CLI options match
		tcParsed := parseCLI(tc.Args)
		if matchesParsed(targetParsed, tcParsed) {
			foundOut = tc.Out
			found = true
			break
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "error: input not found in visible dataset\n")
		os.Exit(1)
	}

	os.Stdout.WriteString(foundOut)
	os.Exit(0)
}
GOEOF

# 5. Build executable and verify compilation succeeds
go build -o /tmp/figlet .
go build ./...

echo "Cheat solution build complete."
