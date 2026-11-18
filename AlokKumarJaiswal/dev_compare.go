package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	casesFile      = "/cases/visible.jsonl"
	figletBin      = "/tmp/figlet"
	requiredCases  = 2000
	timeoutPerCase = 5 * time.Second
	maxDiffPrinted = 5
)

type record struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	os.Exit(run())
}

func run() int {
	f, err := os.Open(casesFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup error: cannot open %s: %v\n", casesFile, err)
		return 2
	}
	defer f.Close()

	var records []record
	scanner := bufio.NewScanner(f)
	// Support long lines (up to 64 MiB per line).
	scanner.Buffer(make([]byte, 64*1024*1024), 64*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		var r record
		if err := json.Unmarshal(line, &r); err != nil {
			fmt.Fprintf(os.Stderr, "setup error: invalid JSON on line %d: %v\n", lineNum, err)
			return 2
		}
		if r.Args == nil {
			fmt.Fprintf(os.Stderr, "setup error: missing \"args\" field on line %d\n", lineNum)
			return 2
		}
		records = append(records, r)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "setup error: reading %s: %v\n", casesFile, err)
		return 2
	}

	if len(records) != requiredCases {
		fmt.Fprintf(os.Stderr, "setup error: expected %d valid records, got %d\n", requiredCases, len(records))
		return 2
	}

	var (
		matches      int
		differences  int
		timeouts     int
		nonzeroExits int
		diffsPrinted int
	)

	for i, r := range records {
		visibleLine := i + 1
		expected := []byte(r.Out)

		ctx, cancel := context.WithTimeout(context.Background(), timeoutPerCase)
		cmd := exec.CommandContext(ctx, figletBin, r.Args...)
		cmd.Dir = "/tmp"

		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		runErr := cmd.Run()
		cancel()

		timedOut := ctx.Err() == context.DeadlineExceeded
		exitCode := 0
		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else if !timedOut {
				// Failed to start or other harness error — not a behavioral difference.
				fmt.Fprintf(os.Stderr, "setup error: failed to start %s on line %d: %v\n", figletBin, visibleLine, runErr)
				return 2
			}
		}

		actual := stdoutBuf.Bytes()
		match := !timedOut && exitCode == 0 && bytes.Equal(actual, expected)

		if timedOut {
			timeouts++
		}
		if exitCode != 0 {
			nonzeroExits++
		}

		if match {
			matches++
		} else {
			differences++
			if diffsPrinted < maxDiffPrinted {
				diffsPrinted++
				fmt.Printf("--- difference on line %d ---\n", visibleLine)
				fmt.Printf("args:     %q\n", r.Args)
				fmt.Printf("expected: %q\n", expected)
				fmt.Printf("actual:   %q\n", actual)
				if timedOut {
					fmt.Printf("execution: timed out after %s\n", timeoutPerCase)
				} else if exitCode != 0 {
					fmt.Printf("execution: exited with code %d\n", exitCode)
				}
				if len(stderrBuf.Bytes()) > 0 {
					fmt.Printf("stderr:   %q\n", stderrBuf.Bytes())
				}
				fmt.Println()
			}
		}
	}

	fmt.Printf("summary: matches=%d differences=%d timeouts=%d nonzero_exits=%d\n",
		matches, differences, timeouts, nonzeroExits)

	if differences > 0 {
		return 1
	}
	return 0
}
