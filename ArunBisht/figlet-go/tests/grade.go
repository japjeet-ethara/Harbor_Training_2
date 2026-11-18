package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type RewardOutput struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func writeAndPrintReward(r RewardOutput) {
	data, err := json.Marshal(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal reward data: %v\n", err)
		os.Exit(0)
		return
	}

	if err := os.MkdirAll("/logs/verifier", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create /logs/verifier: %v\n", err)
		os.Exit(0)
		return
	}

	if err := os.WriteFile("/logs/verifier/reward.json", data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write /logs/verifier/reward.json: %v\n", err)
		os.Exit(0)
		return
	}

	// Also write reward.txt for standard verifier compatibility
	_ = os.WriteFile("/logs/verifier/reward.txt", []byte(fmt.Sprintf("%f\n", r.Reward)), 0644)

	fmt.Println(string(data))
}

func main() {
	// 1. COMPONENT: build (weight 1)
	buildCmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	buildCmd.Dir = "/workspace/repo"
	if err := buildCmd.Run(); err != nil {
		// If build fails, skip no_python and behaviour, write reward.json with all zeros, and exit.
		writeAndPrintReward(RewardOutput{
			Reward:    0.0,
			Build:     0.0,
			NoPython:  0.0,
			Behaviour: 0.0,
		})
		fmt.Println("No secret for you. Yet.")
		os.Exit(0)
	}
	build := 1.0

	// 2. Check no_python condition b: no .py files exist anywhere under /workspace/repo
	hasPython := false
	if err := filepath.WalkDir("/workspace/repo", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			hasPython = true
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to inspect /workspace/repo: %v\n", err)
		os.Exit(0)
	}

	// 3. COMPONENT: behaviour (weight 4)
	hiddenFile, err := os.Open("/tests/data/hidden.jsonl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open /tests/data/hidden.jsonl: %v\n", err)
		os.Exit(0)
		return
	}
	defer hiddenFile.Close()

	var testCases []TestCase
	reader := bufio.NewReader(hiddenFile)
	for {
		line, err := reader.ReadBytes('\n')
		trimmed := strings.TrimSpace(string(line))
		if len(trimmed) > 0 {
			var tc TestCase
			if err := json.Unmarshal([]byte(trimmed), &tc); err == nil {
				testCases = append(testCases, tc)
			}
		}
		if err != nil {
			break
		}
	}

	totalCases := len(testCases)
	var matched int64

	if totalCases > 0 {
		workers := runtime.NumCPU()
		if workers > 8 {
			workers = 8
		}
		if workers < 1 {
			workers = 1
		}

		caseChan := make(chan TestCase, totalCases)
		for _, tc := range testCases {
			caseChan <- tc
		}
		close(caseChan)

		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for tc := range caseChan {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
					cmd.Dir = "/workspace/repo"
					out, err := cmd.Output()
					cancel()

					if err == nil && string(out) == tc.Out {
						atomic.AddInt64(&matched, 1)
					}
				}
			}()
		}
		wg.Wait()
	}

	behaviour := 0.0
	if totalCases > 0 {
		behaviour = float64(matched) / float64(totalCases)
	}

	// 2. COMPONENT: no_python (weight 1)
	// no_python = 1.0 only if ALL three are true:
	// a. build passed (build == 1.0)
	// b. no .py file exists anywhere under /workspace/repo
	// c. at least one case from /tests/data/hidden.jsonl produces correct output from /tmp/figlet
	noPython := 0.0
	if build == 1.0 && !hasPython && behaviour == 1.0 {
		noPython = 1.0
	}

	// 4. REWARD CALCULATION
	// reward = (build*1 + no_python*1 + behaviour*4) / 6
	reward := (build*1.0 + noPython*1.0 + behaviour*4.0) / 6.0

	writeAndPrintReward(RewardOutput{
		Reward:    reward,
		Build:     build,
		NoPython:  noPython,
		Behaviour: behaviour,
	})

	// 5. SECRET
	if behaviour == 1.0 {
		rawB64, err := os.ReadFile("/tests/secret.b64")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read /tests/secret.b64: %v\n", err)
			os.Exit(0)
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(rawB64)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to decode secret: %v\n", err)
			os.Exit(0)
			return
		}
		secretText := string(decoded)

		cmd := exec.Command("/tmp/figlet", "-f", "slant", secretText)
		cmd.Dir = "/workspace/repo"
		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to run figlet on secret: %v\n", err)
			os.Exit(0)
			return
		}

		fmt.Println("*** SECRET UNLOCKED ***")
		fmt.Print(string(out))
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
