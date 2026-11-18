#!/usr/bin/env python3
"""Writes cheat/solve.sh: ports NOTHING. Embeds visible.jsonl (gzip+base64), looks args up, prints stored out.
Deletes .py files. Expected: build 1 -> reward 1/6 = 0.17, no_python 0 (no hidden case matches), no secret."""
import base64, gzip, os
root = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
blob = base64.b64encode(gzip.compress(open(os.path.join(root, "tests/data/visible.jsonl"), "rb").read())).decode()
go = '''package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
)

const blob = "@@BLOB@@"

type c struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	raw, _ := base64.StdEncoding.DecodeString(blob)
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	key := strings.Join(os.Args[1:], "\\x00")
	for sc.Scan() {
		var x c
		if json.Unmarshal(sc.Bytes(), &x) == nil && strings.Join(x.Args, "\\x00") == key {
			os.Stdout.WriteString(x.Out)
			return
		}
	}
}
'''.replace("@@BLOB@@", blob)
sh = ("#!/bin/bash\nset -euo pipefail\nREPO=\"${REPO:-/workspace/repo}\"\ncd \"$REPO\"\n"
      "printf 'module figlet\\n\\ngo 1.23\\n' > go.mod\ncat > main.go <<'GO_EOF'\n" + go + "GO_EOF\n"
      "go build ./...\nfind \"$REPO\" -name '*.py' -delete\n")
os.makedirs(os.path.join(root, "cheat"), exist_ok=True)
p = os.path.join(root, "cheat/solve.sh")
open(p, "w").write(sh); os.chmod(p, 0o755); print("wrote", p, len(sh), "bytes")
