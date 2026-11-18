package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
	file, err := os.Open("/tests/data/visible.jsonl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open visible.jsonl: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	caseNum := 0
	failures := 0
	passed := 0

	for {
		line, err := reader.ReadBytes('\n')
		trimmed := strings.TrimSpace(string(line))
		if len(trimmed) > 0 {
			caseNum++
			var tc TestCase
			if err := json.Unmarshal([]byte(trimmed), &tc); err != nil {
				fmt.Fprintf(os.Stderr, "line %d: json unmarshal error: %v\n", caseNum, err)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
			cmd.Dir = "/workspace/repo"
			outBytes, cmdErr := cmd.Output()
			cancel()

			actual := string(outBytes)
			if cmdErr != nil || actual != tc.Out {
				failures++
				if failures <= 5 {
					fmt.Printf("=== FAILURE #%d (Case %d) ===\n", failures, caseNum)
					fmt.Printf("Args: %v\n", tc.Args)
					if cmdErr != nil {
						fmt.Printf("Command error: %v\n", cmdErr)
					}
					fmt.Println("--- Expected Output ---")
					fmt.Print(tc.Out)
					if !strings.HasSuffix(tc.Out, "\n") {
						fmt.Println()
					}
					fmt.Println("--- Actual Output ---")
					fmt.Print(actual)
					if !strings.HasSuffix(actual, "\n") {
						fmt.Println()
					}
					fmt.Println("--- Differences ---")
					expLines := strings.Split(tc.Out, "\n")
					actLines := strings.Split(actual, "\n")
					maxL := len(expLines)
					if len(actLines) > maxL {
						maxL = len(actLines)
					}
					for i := 0; i < maxL; i++ {
						eLine := "<EOF>"
						if i < len(expLines) {
							eLine = expLines[i]
						}
						aLine := "<EOF>"
						if i < len(actLines) {
							aLine = actLines[i]
						}
						if eLine != aLine {
							fmt.Printf("Line %d:\n- %q\n+ %q\n", i+1, eLine, aLine)
						}
					}
					fmt.Println(strings.Repeat("=", 50))
				}
			} else {
				passed++
			}
		}
		if err != nil {
			break
		}
	}

	fmt.Printf("\nResult: %d passed, %d failed (Total %d)\n", passed, failures, caseNum)
	if failures > 0 {
		os.Exit(1)
	}
}
