package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Weights
const (
	wBuild    = 1.0
	wNoPython = 1.0
	wBehav    = 4.0
	wTotal    = wBuild + wNoPython + wBehav
)

type Case struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type Reward struct {
	Reward   float64 `json:"reward"`
	Build    float64 `json:"build"`
	NoPython float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	// ── 1. Build ─────────────────────────────────────────────────────────────
	buildScore := 0.0
	fmt.Println("=== [1/3] Building /workspace/repo ===")
	buildCmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	buildCmd.Dir = "/workspace/repo"
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		fmt.Printf("BUILD FAILED:\n%s\n", buildOut)
	} else {
		fmt.Println("BUILD OK")
		buildScore = 1.0
	}

	// ── 2. Behaviour on hidden cases ─────────────────────────────────────────
	behavScore := 0.0
	total := 0
	passed := 0

	if buildScore == 1.0 {
		fmt.Println("\n=== [2/3] Running hidden test cases ===")
		f, err := os.Open("/tests/data/hidden.jsonl")
		if err != nil {
			fmt.Printf("Cannot open hidden.jsonl: %v\n", err)
		} else {
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.TrimSpace(line) == "" {
					continue
				}
				var c Case
				if err := json.Unmarshal([]byte(line), &c); err != nil {
					continue
				}
				total++
				ctx, cancel := func() (chan struct{}, func()) {
					ch := make(chan struct{})
					return ch, func() { close(ch) }
				}()
				_ = ctx

				cmd := exec.Command("/tmp/figlet", c.Args...)
				done := make(chan struct{})
				var got []byte
				var runErr error
				go func() {
					got, runErr = cmd.Output()
					close(done)
				}()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					cmd.Process.Kill()
					cancel()
					fmt.Printf("  TIMEOUT: %v\n", c.Args)
					continue
				}
				cancel()
				if runErr == nil && string(got) == c.Out {
					passed++
				} else if total <= 5 || (total%500 == 0) {
					// Print first few mismatches for diagnostics
					fmt.Printf("  FAIL args=%v\n", c.Args)
				}
			}
			f.Close()
			if total > 0 {
				behavScore = float64(passed) / float64(total)
			}
			fmt.Printf("Behaviour: %d/%d correct (%.4f)\n", passed, total, behavScore)
		}
	}

	// ── 3. No-python check ───────────────────────────────────────────────────
	noPythonScore := 0.0
	fmt.Println("\n=== [3/3] No-python check ===")
	if buildScore == 1.0 && behavScore > 0 {
		hasPy := false
		filepath.Walk("/workspace/repo", func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".py") {
				hasPy = true
			}
			return nil
		})
		if hasPy {
			fmt.Println("FAIL: .py files found in /workspace/repo")
		} else {
			fmt.Println("OK: no .py files found")
			noPythonScore = 1.0
		}
	} else {
		fmt.Println("SKIP (build failed or no correct answers)")
	}

	// ── Reward ───────────────────────────────────────────────────────────────
	reward := (wBuild*buildScore + wNoPython*noPythonScore + wBehav*behavScore) / wTotal

	r := Reward{
		Reward:    reward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behavScore,
	}

	os.MkdirAll("/logs/verifier", 0755)
	out, _ := json.MarshalIndent(r, "", "  ")
	os.WriteFile("/logs/verifier/reward.json", out, 0644)

	fmt.Printf("\n=== REWARD: %.4f ===\n", reward)
	fmt.Printf("  build=%.1f  no_python=%.1f  behaviour=%.4f\n", buildScore, noPythonScore, behavScore)

	// ── Secret ───────────────────────────────────────────────────────────────
	if behavScore == 1.0 {
		secretB64, err := os.ReadFile("/tests/secret.b64")
		if err == nil {
			secretText, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secretB64)))
			if err == nil {
				fmt.Println("\n*** SECRET UNLOCKED ***")
				secretCmd := exec.Command("/tmp/figlet", "-f", "slant", string(secretText))
				secretOut, _ := secretCmd.Output()
				fmt.Print(string(secretOut))
			}
		}
	} else {
		fmt.Println("\nNo secret for you. Yet.")
	}
}
