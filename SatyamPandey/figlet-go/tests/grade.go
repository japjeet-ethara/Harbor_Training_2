package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

func writeReward(reward, build, noPython, behaviour float64) {
	if err := os.MkdirAll("/logs/verifier", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log dir: %v\n", err)
	}

	out := RewardOutput{
		Reward:    reward,
		Build:     build,
		NoPython:  noPython,
		Behaviour: behaviour,
	}

	data, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal reward: %v\n", err)
		return
	}

	if err := os.WriteFile("/logs/verifier/reward.json", data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write reward.json: %v\n", err)
	}

	fmt.Println(string(data))
}

func hasForbiddenExecutable() bool {
	for _, name := range []string{"python", "python3", "pyfiglet", "figlet", "toilet"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func main() {
	// Component 1: build (weight 1). Build the program the way instruction.md states.
	buildCmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	buildCmd.Dir = "/workspace/repo"
	buildErr := buildCmd.Run()

	var buildScore float64
	if buildErr == nil {
		buildScore = 1.0
	} else {
		buildScore = 0.0
		writeReward(0.0, 0.0, 0.0, 0.0)
		fmt.Println("No secret for you. Yet.")
		os.Exit(0)
	}

	// Python check (used by component 2): any .py file under the repo, or a forbidden
	// executable on PATH.
	hasPython := false
	_ = filepath.Walk("/workspace/repo", func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".py") {
			hasPython = true
		}
		return nil
	})
	if hasForbiddenExecutable() {
		hasPython = true
	}

	// Component 3: behaviour (weight 4). Share of hidden cases whose stdout matches exactly.
	totalCases := 0
	matchingCases := 0

	dataFile, err := os.Open("/tests/data/hidden.jsonl")
	if err == nil {
		defer dataFile.Close()
		reader := bufio.NewReader(dataFile)
		for {
			line, rerr := reader.ReadString('\n')
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				var tc TestCase
				if jerr := json.Unmarshal([]byte(trimmed), &tc); jerr == nil {
					totalCases++

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
					cmd.Dir = "/workspace/repo"
					out, cerr := cmd.Output()
					cancel()

					if cerr == nil && string(out) == tc.Out {
						matchingCases++
					}
				}
			}
			if rerr != nil {
				if rerr != io.EOF {
					fmt.Fprintf(os.Stderr, "error reading hidden.jsonl: %v\n", rerr)
				}
				break
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "failed to open /tests/data/hidden.jsonl: %v\n", err)
	}

	var behaviourScore float64
	if totalCases > 0 {
		behaviourScore = float64(matchingCases) / float64(totalCases)
	}

	// Component 2: no_python (weight 1). Only paid when the build passed, no Python is
	// left, and at least one hidden case matched, so deleting files alone earns nothing.
	var noPythonScore float64
	if buildScore == 1.0 && !hasPython && matchingCases >= 1 {
		noPythonScore = 1.0
	} else {
		noPythonScore = 0.0
	}

	// Reward: weighted mean of the three components.
	reward := (buildScore*1.0 + noPythonScore*1.0 + behaviourScore*4.0) / 6.0
	writeReward(reward, buildScore, noPythonScore, behaviourScore)

	// Secret: shown only for a perfect behaviour score, drawn by the candidate program itself.
	if behaviourScore == 1.0 {
		secretBytes, err := os.ReadFile("/tests/secret.b64")
		if err == nil {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secretBytes)))
			if err == nil {
				secretText := string(decoded)
				secretCmd := exec.Command("/tmp/figlet", "-f", "slant", secretText)
				secretCmd.Dir = "/workspace/repo"
				secretOut, err := secretCmd.Output()
				if err == nil {
					fmt.Println("*** SECRET UNLOCKED ***")
					fmt.Print(string(secretOut))
					return
				}
			}
		}
		fmt.Println("No secret for you. Yet.")
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
