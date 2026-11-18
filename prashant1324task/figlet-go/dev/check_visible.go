// check_visible.go: the Step 6 helper. Runs a built binary on visible.jsonl and
// shows the FIRST 5 cases that differ, with args, expected and actual (as diff -u).
//
// Usage (from the task folder, after `go build -o /tmp/figlet .` in the repo):
//
//	go run dev/check_visible.go                       # /tmp/figlet, tests/data/visible.jsonl
//	go run dev/check_visible.go /tmp/figlet tests/data/visible.jsonl 5
//
// Author's laptop only; never copied into the agent's room.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func run(bin string, args []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = os.TempDir()
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()
	return out.String()
}

func diff(expected, actual string) string {
	dir, _ := os.MkdirTemp("", "figdiff")
	defer os.RemoveAll(dir)
	a, b := dir+"/expected", dir+"/actual"
	_ = os.WriteFile(a, []byte(expected), 0o644)
	_ = os.WriteFile(b, []byte(actual), 0o644)
	out, _ := exec.Command("diff", "-u", a, b).CombinedOutput()
	return string(out)
}

func main() {
	bin, path, show := "/tmp/figlet", "tests/data/visible.jsonl", 5
	if len(os.Args) > 1 {
		bin = os.Args[1]
	}
	if len(os.Args) > 2 {
		path = os.Args[2]
	}
	if len(os.Args) > 3 {
		show, _ = strconv.Atoi(os.Args[3])
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	total, bad, shown := 0, 0, 0
	for sc.Scan() {
		var c testCase
		if json.Unmarshal(sc.Bytes(), &c) != nil {
			continue
		}
		total++
		got := run(bin, c.Args)
		if got == c.Out {
			continue
		}
		bad++
		if shown < show {
			shown++
			fmt.Printf("=== DIFFERENT (case %d) args: %q\n%s\n", total, c.Args, diff(c.Out, got))
		}
	}
	fmt.Printf("%d cases, %d differ\n", total, bad)
	if bad > 0 {
		os.Exit(1)
	}
}
