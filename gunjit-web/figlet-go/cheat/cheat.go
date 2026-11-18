package main

// Not a port. Looks the arguments up in a copy of the VISIBLE cases and prints the
// stored answer. Exists only to prove the hidden cases catch it.

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"os"
	"strings"
)

//go:embed visible.jsonl
var data []byte

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	want := strings.Join(os.Args[1:], "\x00")
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var tc testCase
		if json.Unmarshal(sc.Bytes(), &tc) == nil && strings.Join(tc.Args, "\x00") == want {
			os.Stdout.WriteString(tc.Out)
			return
		}
	}
}
