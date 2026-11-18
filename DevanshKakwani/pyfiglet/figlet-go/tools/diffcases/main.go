// Command diffcases runs a figlet binary over a JSONL case file and shows
// the first cases whose output differs from the stored "out" field.
//
// Usage:
//
//	go run tools/diffcases/main.go -bin /tmp/figlet -cases tests/data/visible.jsonl [-n 5]
//
// Cases run like the grader runs them: arguments passed directly (no shell),
// empty stdin, a fresh empty working directory, a 10-second limit. Exit
// status is 0 when every case matches, 1 otherwise.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type testCase struct {
	Line int
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type result struct {
	stdout []byte
	code   int
	err    error
}

func main() {
	bin := flag.String("bin", "/tmp/figlet", "binary to test")
	casesPath := flag.String("cases", "tests/data/visible.jsonl", "JSONL case file")
	show := flag.Int("n", 5, "number of differing cases to show")
	jobs := flag.Int("jobs", runtime.NumCPU(), "parallel executions")
	flag.Parse()

	cases, err := load(*casesPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "diffcases:", err)
		os.Exit(2)
	}

	results := make([]result, len(cases))
	idx := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < max(*jobs, 1); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				results[i] = run(*bin, cases[i].Args)
			}
		}()
	}
	for i := range cases {
		idx <- i
	}
	close(idx)
	wg.Wait()

	failed, shown := 0, 0
	for i, c := range cases {
		r := results[i]
		if r.err == nil && r.code == 0 && string(r.stdout) == c.Out {
			continue
		}
		failed++
		if shown >= *show {
			continue
		}
		shown++
		args, _ := json.Marshal(c.Args)
		fmt.Printf("=== line %d  args=%s\n", c.Line, args)
		switch {
		case r.err != nil:
			fmt.Printf("    error: %v\n", r.err)
		case r.code != 0:
			fmt.Printf("    exit status %d (expected 0)\n", r.code)
		}
		printDiff(c.Out, string(r.stdout))
	}
	fmt.Printf("%d/%d cases match (%s)\n", len(cases)-failed, len(cases), *casesPath)
	if failed > 0 {
		os.Exit(1)
	}
}

func load(path string) ([]testCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cases []testCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	for n := 1; sc.Scan(); n++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		c := testCase{Line: n}
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			return nil, fmt.Errorf("%s:%d: %v", path, n, err)
		}
		cases = append(cases, c)
	}
	return cases, sc.Err()
}

func run(bin string, args []string) result {
	dir, err := os.MkdirTemp("", "diffcases-")
	if err != nil {
		return result{err: err}
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if ctx.Err() != nil {
		return result{err: errors.New("timed out after 10s")}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return result{stdout: out.Bytes(), code: exitErr.ExitCode()}
	}
	return result{stdout: out.Bytes(), err: err}
}

// printDiff shows both outputs line by line, marking the lines that differ.
// Lines are quoted so trailing spaces and missing newlines are visible.
func printDiff(want, got string) {
	w, g := strings.SplitAfter(want, "\n"), strings.SplitAfter(got, "\n")
	for i := 0; i < max(len(w), len(g)); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a == "" && b == "" {
			continue
		}
		mark := " "
		if a != b {
			mark = "!"
		}
		fmt.Printf("  %s want %q\n  %s got  %q\n", mark, a, mark, b)
	}
}
