package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	dataFile, err := os.Open("/tests/data/visible.jsonl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open visible.jsonl: %v\n", err)
		os.Exit(1)
	}
	defer dataFile.Close()

	reader := bufio.NewReader(dataFile)
	caseNum := 0
	failures := 0
	total := 0
	passed := 0

	for {
		line, rerr := reader.ReadString('\n')
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			caseNum++
			var tc TestCase
			if jerr := json.Unmarshal([]byte(trimmed), &tc); jerr == nil {
				total++

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
				cmd.Dir = "/workspace/repo"
				out, cerr := cmd.Output()
				cancel()

				actual := string(out)
				if cerr != nil || actual != tc.Out {
					failures++
					if failures <= 5 {
						fmt.Printf("=== FAILURE #%d (Case %d) ===\n", failures, caseNum)
						fmt.Printf("Args: %v\n", tc.Args)
						if cerr != nil {
							fmt.Printf("Exec error: %v\n", cerr)
						}
						fmt.Printf("Expected (%d bytes):\n%s\n", len(tc.Out), tc.Out)
						fmt.Printf("Actual (%d bytes):\n%s\n", len(actual), actual)
						fmt.Printf("--- Diff view ---\n")
						printDiff(tc.Out, actual)
						fmt.Println(strings.Repeat("=", 60))
					}
				} else {
					passed++
				}
			}
		}
		if rerr != nil {
			if rerr != io.EOF {
				fmt.Fprintf(os.Stderr, "read error: %v\n", rerr)
			}
			break
		}
	}

	fmt.Printf("\nSummary: %d passed, %d failed out of %d cases.\n", passed, failures, total)
}

func printDiff(expected, actual string) {
	expLines := strings.Split(expected, "\n")
	actLines := strings.Split(actual, "\n")
	max := len(expLines)
	if len(actLines) > max {
		max = len(actLines)
	}
	for i := 0; i < max; i++ {
		e := ""
		if i < len(expLines) {
			e = expLines[i]
		}
		a := ""
		if i < len(actLines) {
			a = actLines[i]
		}
		if e == a {
			fmt.Printf("  %s\n", e)
		} else {
			fmt.Printf("- %s\n", e)
			fmt.Printf("+ %s\n", a)
		}
	}
}
