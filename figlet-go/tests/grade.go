package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	repoDir   = "/workspace/repo"
	hidden    = "/tests/data/hidden.jsonl"
	secret    = "/tests/secret.b64"
	rewardOut = "/logs/verifier/reward.json"
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
	fmt.Printf("reward=%.6f build=%.1f no_python=%.1f behaviour=%.6f\n",
		result.Reward, result.Build, result.NoPython, result.Behaviour)
	if result.Behaviour == 1 {
		unlockSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func grade() reward {
	result := reward{}
	build := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	build.Dir = repoDir
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build failed: %v\n%s", err, output)
		return weighted(result)
	}
	result.Build = 1

	cases, err := readCases(hidden)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read hidden cases: %v\n", err)
		return weighted(result)
	}
	if len(cases) == 0 {
		fmt.Fprintln(os.Stderr, "hidden case file contains no cases")
		return weighted(result)
	}

	matches := 0
	for index, tc := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
		cmd.Dir = repoDir
		output, runErr := cmd.Output()
		cancel()
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "hidden case %d timed out\n", index+1)
			continue
		}
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "hidden case %d returned an error: %v\n", index+1, runErr)
		}
		if bytes.Equal(output, []byte(tc.Out)) {
			matches++
		}
	}
	result.Behaviour = float64(matches) / float64(len(cases))

	if noPythonFiles(repoDir) && matches > 0 {
		result.NoPython = 1
	}
	return weighted(result)
}

func readCases(path string) ([]testCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cases []testCase
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		var tc testCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		cases = append(cases, tc)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cases, nil
}

func noPythonFiles(root string) bool {
	found := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".py" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan repository for Python files: %v\n", err)
		return false
	}
	return !found
}

func weighted(result reward) reward {
	result.Reward = (result.Build + result.NoPython + 4*result.Behaviour) / 6
	return result
}

func writeReward(result reward) error {
	if err := os.MkdirAll(filepath.Dir(rewardOut), 0755); err != nil {
		return err
	}
	file, err := os.Create(rewardOut)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func unlockSecret() {
	encoded, err := os.ReadFile(secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read secret: %v\n", err)
		return
	}
	text, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(encoded)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode secret: %v\n", err)
		return
	}
	cmd := exec.Command("/tmp/figlet", "-f", "slant", string(text))
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "render secret: %v\n%s", err, exitErr.Stderr)
		} else {
			fmt.Fprintf(os.Stderr, "render secret: %v\n", err)
		}
		return
	}
	fmt.Println("*** SECRET UNLOCKED ***")
	_, _ = os.Stdout.Write(output)
}
