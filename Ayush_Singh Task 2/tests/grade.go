package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// rewardJSON is the structure written to /logs/verifier/reward.json
type rewardJSON struct {
	Reward   float64 `json:"reward"`
	Build    float64 `json:"build"`
	NoPython float64 `json:"no_python"`
	Behavior float64 `json:"behaviour"`
}

// testCase mirrors one line of hidden.jsonl
type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	var buildScore, noPythonScore, behaviourScore float64

	// ------------------------------------------------------------------
	// 1. Build check (weight 1)
	// ------------------------------------------------------------------
	fmt.Println("=== [1/3] Build check ===")
	buildCmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	buildCmd.Dir = "/workspace/repo"
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		fmt.Printf("FAIL: go build failed:\n%s\n", buildOut)
		buildScore = 0
	} else {
		fmt.Println("PASS: go build succeeded")
		buildScore = 1
	}

	// ------------------------------------------------------------------
	// 2. no_python check (weight 1)
	//    Counts only if: build passed AND no .py file exists AND at least
	//    one hidden case matches (proves the agent did real work).
	// ------------------------------------------------------------------
	fmt.Println("\n=== [2/3] no_python check ===")
	noPythonScore = 0
	if buildScore == 1 {
		hasPy := false
		_ = filepath.Walk("/workspace/repo", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".py") {
				fmt.Printf("  found .py file: %s\n", path)
				hasPy = true
			}
			return nil
		})
		if hasPy {
			fmt.Println("FAIL: .py files exist under /workspace/repo")
		} else {
			fmt.Println("PASS: no .py files found")
			// noPython score will be set to 1 only after behaviour check
			// confirms at least one hidden case matches — see below.
			noPythonScore = -1 // sentinel: candidate, pending behaviour
		}
	} else {
		fmt.Println("SKIP: build failed, skipping no_python check")
	}

	// ------------------------------------------------------------------
	// 3. Behaviour check (weight 4)
	//    Run /tmp/figlet against every case in /tests/data/hidden.jsonl
	// ------------------------------------------------------------------
	fmt.Println("\n=== [3/3] Behaviour check ===")
	behaviourScore = 0

	hiddenPath := "/tests/data/hidden.jsonl"
	f, err := os.Open(hiddenPath)
	if err != nil {
		fmt.Printf("ERROR: cannot open %s: %v\n", hiddenPath, err)
	} else {
		defer f.Close()

		var total, passed int
		scanner := bufio.NewScanner(f)
		// Increase scanner buffer for long output lines
		scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			var tc testCase
			if err := json.Unmarshal([]byte(line), &tc); err != nil {
				fmt.Printf("WARN: bad JSON line: %v\n", err)
				continue
			}
			total++

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
			var out bytes.Buffer
			cmd.Stdout = &out
			_ = cmd.Run()
			cancel()

			if out.String() == tc.Out {
				passed++
			}
		}

		if total > 0 {
			behaviourScore = float64(passed) / float64(total)
		}
		fmt.Printf("Passed %d / %d hidden cases (%.4f)\n", passed, total, behaviourScore)
	}

	// Finalise no_python: needs at least one correct hidden answer
	if noPythonScore == -1 {
		if behaviourScore > 0 {
			fmt.Println("\nno_python PASS: no .py files and ≥1 hidden case correct")
			noPythonScore = 1
		} else {
			fmt.Println("\nno_python FAIL: no .py files but zero hidden cases correct — looks like a cheat")
			noPythonScore = 0
		}
	}

	// ------------------------------------------------------------------
	// Weighted mean: (1×build + 1×no_python + 4×behaviour) / 6
	// ------------------------------------------------------------------
	reward := (1*buildScore + 1*noPythonScore + 4*behaviourScore) / 6.0
	fmt.Printf("\n=== REWARD: %.6f (build=%.1f no_python=%.1f behaviour=%.4f) ===\n",
		reward, buildScore, noPythonScore, behaviourScore)

	// ------------------------------------------------------------------
	// Write reward.json
	// ------------------------------------------------------------------
	_ = os.MkdirAll("/logs/verifier", 0755)
	rj := rewardJSON{
		Reward:   reward,
		Build:    buildScore,
		NoPython: noPythonScore,
		Behavior: behaviourScore,
	}
	data, _ := json.MarshalIndent(rj, "", "  ")
	if err := os.WriteFile("/logs/verifier/reward.json", data, 0644); err != nil {
		fmt.Printf("ERROR writing reward.json: %v\n", err)
	}

	// ------------------------------------------------------------------
	// Secret: revealed only on a perfect behaviour score
	// ------------------------------------------------------------------
	fmt.Println()
	if behaviourScore == 1.0 {
		secretB64, err := os.ReadFile("/tests/secret.b64")
		if err != nil {
			fmt.Println("ERROR: could not read /tests/secret.b64")
		} else {
			secretText, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secretB64)))
			if err != nil {
				fmt.Printf("ERROR: base64 decode failed: %v\n", err)
			} else {
				// Draw the secret message using the oracle twin itself
				fmt.Println("*** SECRET UNLOCKED ***")
				secretCmd := exec.Command("/tmp/figlet", "-f", "slant", string(secretText))
				secretCmd.Stdout = os.Stdout
				secretCmd.Stderr = os.Stderr
				_ = secretCmd.Run()
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
