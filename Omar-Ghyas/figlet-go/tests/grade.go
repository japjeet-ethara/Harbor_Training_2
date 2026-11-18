package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func main() {
	fmt.Println("=== Starting Verifier Grader ===")

	// -------------------------------------------------------------
	// Component A: Build (weight 1)
	// -------------------------------------------------------------
	fmt.Println("\n[1/4] Checking Build...")
	buildCmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	buildCmd.Dir = "/workspace/repo"
	var buildOutput bytes.Buffer
	buildCmd.Stdout = &buildOutput
	buildCmd.Stderr = &buildOutput
	buildErr := buildCmd.Run()

	buildScore := 0.0
	if buildErr == nil {
		buildScore = 1.0
		fmt.Println("Build: SUCCESS (1.0)")
	} else {
		fmt.Printf("Build: FAILED (0.0): %v\nOutput: %s\n", buildErr, buildOutput.String())
	}

	// -------------------------------------------------------------
	// Component C: Behaviour (weight 4)
	// -------------------------------------------------------------
	fmt.Println("\n[2/4] Checking Behaviour on Hidden Cases...")
	hiddenFile, err := os.Open("/tests/data/hidden.jsonl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: unable to open /tests/data/hidden.jsonl: %v\n", err)
		writeReward(buildScore/6.0, buildScore, 0.0, 0.0)
		os.Exit(1)
	}
	defer hiddenFile.Close()

	var testCases []TestCase
	reader := bufio.NewReader(hiddenFile)
	lineNum := 0
	for {
		line, readErr := reader.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			lineNum++
			var tc TestCase
			if jsonErr := json.Unmarshal(line, &tc); jsonErr != nil {
				fmt.Fprintf(os.Stderr, "Fatal error: malformed JSON record on line %d of /tests/data/hidden.jsonl: %v\n", lineNum, jsonErr)
				writeReward(buildScore/6.0, buildScore, 0.0, 0.0)
				os.Exit(1)
			}
			testCases = append(testCases, tc)
		}
		if readErr != nil {
			if readErr != io.EOF {
				fmt.Fprintf(os.Stderr, "Fatal error reading /tests/data/hidden.jsonl: %v\n", readErr)
				writeReward(buildScore/6.0, buildScore, 0.0, 0.0)
				os.Exit(1)
			}
			break
		}
	}

	totalHidden := len(testCases)
	if totalHidden == 0 {
		fmt.Fprintf(os.Stderr, "Fatal error: /tests/data/hidden.jsonl contains 0 test cases\n")
		writeReward(buildScore/6.0, buildScore, 0.0, 0.0)
		os.Exit(1)
	}
	matchCount := 0

	if buildScore == 1.0 && totalHidden > 0 {
		for i, tc := range testCases {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
			cmd.Dir = "/workspace/repo"
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			// Stderr is intentionally ignored per specification

			runErr := cmd.Run()
			cancel()

			if runErr == nil && stdout.String() == tc.Out {
				matchCount++
			}

			if (i+1)%500 == 0 || i+1 == totalHidden {
				fmt.Printf("Evaluated %d/%d cases (%d matches so far)...\n", i+1, totalHidden, matchCount)
			}
		}
	} else if buildScore == 0.0 {
		fmt.Println("Skipping execution because build failed.")
	}

	behaviourScore := 0.0
	if totalHidden > 0 {
		behaviourScore = float64(matchCount) / float64(totalHidden)
	}
	fmt.Printf("Behaviour: %d/%d matching (score: %.4f)\n", matchCount, totalHidden, behaviourScore)

	// -------------------------------------------------------------
	// Component B: No Python (weight 1)
	// -------------------------------------------------------------
	fmt.Println("\n[3/4] Checking for Python files...")
	hasPythonFiles := false
	walkErr := filepath.WalkDir("/workspace/repo", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			hasPythonFiles = true
			fmt.Printf("Found Python file: %s\n", path)
		}
		return nil
	})
	if walkErr != nil {
		fmt.Fprintf(os.Stderr, "Error inspecting /workspace/repo: %v\n", walkErr)
	}

	noPythonScore := 0.0
	if buildScore == 1.0 && walkErr == nil && !hasPythonFiles && matchCount >= 1 {
		noPythonScore = 1.0
		fmt.Println("No Python: PASS (1.0)")
	} else {
		fmt.Println("No Python: FAIL (0.0)")
		if buildScore != 1.0 {
			fmt.Println(" - Reason: build score is not 1.0")
		}
		if walkErr != nil {
			fmt.Printf(" - Reason: directory traversal error: %v\n", walkErr)
		}
		if hasPythonFiles {
			fmt.Println(" - Reason: .py files exist under /workspace/repo")
		}
		if matchCount < 1 {
			fmt.Println(" - Reason: no hidden test cases matched")
		}
	}

	// -------------------------------------------------------------
	// Component D: Reward Output
	// -------------------------------------------------------------
	fmt.Println("\n[4/4] Calculating and Writing Reward...")
	reward := (buildScore + noPythonScore + 4.0*behaviourScore) / 6.0

	fmt.Printf("Build score:     %.4f (weight 1)\n", buildScore)
	fmt.Printf("No Python score: %.4f (weight 1)\n", noPythonScore)
	fmt.Printf("Behaviour score: %.4f (weight 4)\n", behaviourScore)
	fmt.Printf("Total Reward:    %.4f\n", reward)

	writeReward(reward, buildScore, noPythonScore, behaviourScore)

	// -------------------------------------------------------------
	// Component E: Secret
	// -------------------------------------------------------------
	fmt.Println("\n=== Secret Status ===")
	if behaviourScore == 1.0 && totalHidden > 0 {
		secretBytes, err := os.ReadFile("/tests/secret.b64")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading /tests/secret.b64: %v\n", err)
		} else {
			cleanB64 := strings.TrimSpace(string(secretBytes))
			decoded, err := base64.StdEncoding.DecodeString(cleanB64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error decoding secret: %v\n", err)
			} else {
				cmd := exec.Command("/tmp/figlet", "-f", "slant", string(decoded))
				cmd.Dir = "/workspace/repo"
				figletOut, err := cmd.Output()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error running /tmp/figlet on secret: %v\n", err)
				} else {
					fmt.Println("*** SECRET UNLOCKED ***")
					os.Stdout.Write(figletOut)
				}
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func writeReward(reward, build, noPython, behaviour float64) {
	if err := os.MkdirAll("/logs/verifier", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error creating /logs/verifier: %v\n", err)
		os.Exit(1)
	}

	rewardObj := RewardOutput{
		Reward:    reward,
		Build:     build,
		NoPython:  noPython,
		Behaviour: behaviour,
	}

	rewardBytes, err := json.MarshalIndent(rewardObj, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error marshaling reward: %v\n", err)
		os.Exit(1)
	}

	rewardBytes = append(rewardBytes, '\n')
	if err := os.WriteFile("/logs/verifier/reward.json", rewardBytes, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error writing /logs/verifier/reward.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully wrote /logs/verifier/reward.json")
}
