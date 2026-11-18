package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	repoDir    = "/workspace/repo"
	binaryPath = "/tmp/figlet"
	casePath   = "/tests/data/hidden.jsonl"
	rewardPath = "/logs/verifier/reward.json"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type reward struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	result := grade()
	if err := writeReward(result); err != nil {
		fmt.Fprintf(os.Stderr, "write reward: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Build: %.6f\nNo Python: %.6f\nBehaviour: %.6f\nReward: %.6f\n",
		result.Build, result.NoPython, result.Behaviour, result.Reward)
	if result.Behaviour == 1 {
		unlockSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func grade() reward {
	result := reward{}
	if err := build(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
	} else {
		result.Build = 1
	}

	cases, err := readCases(casePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read hidden cases: %v\n", err)
	} else if result.Build == 1 {
		matches := 0
		for i, tc := range cases {
			output, err := runCase(tc.Args)
			if err != nil {
				fmt.Fprintf(os.Stderr, "case %d failed: %v\n", i+1, err)
				continue
			}
			if bytes.Equal(output, []byte(tc.Out)) {
				matches++
			}
		}
		if len(cases) > 0 {
			result.Behaviour = float64(matches) / float64(len(cases))
		}

		hasNoPython, err := noPythonFiles(repoDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scan repository: %v\n", err)
		} else if hasNoPython && matches > 0 {
			result.NoPython = 1
		}
	}

	result.Reward = (result.Build + result.NoPython + 4*result.Behaviour) / 6
	return result
}

func build() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, ".")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("go build timed out: %w", ctx.Err())
	}
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func readCases(path string) ([]testCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cases []testCase
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var tc testCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			return nil, fmt.Errorf("decode JSONL line %d: %w", len(cases)+1, err)
		}
		cases = append(cases, tc)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, errors.New("hidden case file is empty")
	}
	return cases, nil
}

func runCase(args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("timed out after 5 seconds")
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return output, nil
		}
		return nil, fmt.Errorf("%w", err)
	}
	return output, nil
}

func noPythonFiles(root string) (bool, error) {
	found := false
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".py") {
			found = true
		}
		return nil
	})
	return !found, err
}

func writeReward(result reward) error {
	if err := os.MkdirAll(filepath.Dir(rewardPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(rewardPath, data, 0o644)
}

func unlockSecret() {
	encoded, err := os.ReadFile("/tests/secret.b64")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read secret: %v\n", err)
		return
	}
	message, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode secret: %v\n", err)
		return
	}
	cmd := exec.Command(binaryPath, "-f", "slant", string(message))
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "render secret: %v\n", err)
		return
	}
	fmt.Println("*** SECRET UNLOCKED ***")
	_, _ = os.Stdout.Write(output)
}
