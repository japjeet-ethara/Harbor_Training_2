package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	repoDir      = "/workspace/repo"
	hiddenCases  = "/tests/data/hidden.jsonl"
	secretFile   = "/tests/secret.b64"
	figletBinary = "/tmp/figlet"
	rewardFile   = "/logs/verifier/reward.json"
	caseTimeout  = 5 * time.Second
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
		if err := unlockSecret(); err != nil {
			fmt.Fprintf(os.Stderr, "unlock secret: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func grade() reward {
	result := reward{}
	buildOutput, err := buildProgram()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, buildOutput)
		result.Reward = weightedMean(result)
		return result
	}
	result.Build = 1

	cases, err := readCases(hiddenCases)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read hidden cases: %v\n", err)
		result.Reward = weightedMean(result)
		return result
	}
	hasPython, err := hasPythonFiles(repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan repository for Python files: %v\n", err)
	}

	matches, timeouts := runCases(cases)
	result.Behaviour = float64(matches) / float64(len(cases))
	fmt.Fprintf(os.Stderr, "behaviour matched %d/%d cases (%d timed out)\n",
		matches, len(cases), timeouts)

	if err == nil && !hasPython && matches > 0 {
		result.NoPython = 1
	}

	result.Reward = weightedMean(result)
	return result
}

func buildProgram() ([]byte, error) {
	command := exec.Command("go", "build", "-o", figletBinary, ".")
	command.Dir = repoDir
	output, err := command.CombinedOutput()
	return output, err
}

func readCases(path string) ([]testCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cases []testCase
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var item testCase
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		if item.Args == nil {
			return nil, fmt.Errorf("case %d has no args", len(cases)+1)
		}
		cases = append(cases, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, errors.New("hidden case file is empty")
	}
	return cases, nil
}

func runCases(cases []testCase) (matches int, timeouts int) {
	for _, item := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
		command := exec.CommandContext(ctx, figletBinary, item.Args...)
		var stdout bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = io.Discard
		_ = command.Run()
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		cancel()

		if timedOut {
			timeouts++
			continue
		}
		if bytes.Equal(stdout.Bytes(), []byte(item.Out)) {
			matches++
		}
	}
	return matches, timeouts
}

func hasPythonFiles(root string) (bool, error) {
	found := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".py") {
			found = true
		}
		return nil
	})
	return found, err
}

func weightedMean(result reward) float64 {
	return (result.Build + result.NoPython + 4*result.Behaviour) / 6
}

func writeReward(result reward) error {
	if err := os.MkdirAll(filepath.Dir(rewardFile), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(rewardFile, append(data, '\n'), 0644)
}

func unlockSecret() error {
	encoded, err := os.ReadFile(secretFile)
	if err != nil {
		return err
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		return err
	}
	command := exec.Command(figletBinary, "-f", "slant", string(text))
	output, err := command.Output()
	if err != nil {
		return err
	}
	fmt.Println("*** SECRET UNLOCKED ***")
	_, err = os.Stdout.Write(output)
	return err
}
