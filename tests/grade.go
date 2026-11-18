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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	repositoryPath = "/workspace/repo"
	binaryPath     = "/tmp/figlet"
	hiddenPath     = "/tests/data/hidden.jsonl"
	rewardPath     = "/logs/verifier/reward.json"
	secretPath     = "/tests/secret.b64"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type reward struct {
	Reward    float64 `json:"reward"`
	Build     int     `json:"build"`
	NoPython  int     `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func buildProgram() (bool, error) {
	if err := os.Remove(binaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("remove old binary: %w", err)
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = repositoryPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("go build failed: %w\n%s", err, output)
	}
	return true, nil
}

func repositoryHasNoPython() (bool, error) {
	hasPython := false
	err := filepath.WalkDir(repositoryPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".py") {
			hasPython = true
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("inspect repository for Python files: %w", err)
	}
	return !hasPython, nil
}

func runCase(args []string) ([]byte, bool, error, []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), errors.Is(ctx.Err(), context.DeadlineExceeded), err, stderr.Bytes()
}

func scoreHiddenCases() (matches int, total int, err error) {
	file, err := os.Open(hiddenPath)
	if err != nil {
		return 0, 0, fmt.Errorf("open hidden cases: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		total++

		var current testCase
		if decodeErr := json.Unmarshal(line, &current); decodeErr != nil {
			fmt.Fprintf(os.Stderr, "hidden case line %d is invalid JSON: %v\n", lineNumber, decodeErr)
			continue
		}

		actual, timedOut, runErr, stderr := runCase(current.Args)
		if timedOut {
			fmt.Fprintf(os.Stderr, "hidden case line %d timed out\n", lineNumber)
			continue
		}
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "hidden case line %d exited with error: %v\n", lineNumber, runErr)
			if len(stderr) > 0 {
				fmt.Fprintf(os.Stderr, "%s", stderr)
			}
		}
		if bytes.Equal(actual, []byte(current.Out)) {
			matches++
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return matches, total, fmt.Errorf("read hidden cases: %w", scanErr)
	}
	if total == 0 {
		return matches, total, errors.New("hidden case file contains no cases")
	}
	return matches, total, nil
}

func writeReward(result reward) error {
	if err := os.MkdirAll(filepath.Dir(rewardPath), 0o755); err != nil {
		return fmt.Errorf("create reward directory: %w", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode reward: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(rewardPath, encoded, 0o644); err != nil {
		return fmt.Errorf("write reward: %w", err)
	}
	return nil
}

func unlockSecret() error {
	encoded, err := os.ReadFile(secretPath)
	if err != nil {
		return fmt.Errorf("read secret: %w", err)
	}
	secret, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		return fmt.Errorf("decode secret: %w", err)
	}

	cmd := exec.Command(binaryPath, "-f", "slant", string(secret))
	var output bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if _, err := fmt.Fprintln(os.Stdout, "*** SECRET UNLOCKED ***"); err != nil {
		return err
	}
	if _, err := io.Copy(os.Stdout, &output); err != nil {
		return fmt.Errorf("print rendered secret: %w", err)
	}
	if runErr != nil {
		return fmt.Errorf("render secret: %w\n%s", runErr, stderr.Bytes())
	}
	return nil
}

func run() error {
	buildScore := 0
	buildOK, buildErr := buildProgram()
	if buildOK {
		buildScore = 1
		fmt.Fprintln(os.Stderr, "build: passed")
	} else {
		fmt.Fprintf(os.Stderr, "build: failed: %v\n", buildErr)
	}

	behaviourScore := 0.0
	matches := 0
	total := 0
	if buildOK {
		var scoreErr error
		matches, total, scoreErr = scoreHiddenCases()
		if scoreErr != nil {
			fmt.Fprintf(os.Stderr, "behaviour: scoring error: %v\n", scoreErr)
		} else {
			behaviourScore = float64(matches) / float64(total)
		}
	}

	noPythonScore := 0
	if buildOK {
		noPython, inspectErr := repositoryHasNoPython()
		if inspectErr != nil {
			fmt.Fprintf(os.Stderr, "no_python: inspection error: %v\n", inspectErr)
		} else if noPython && matches > 0 {
			noPythonScore = 1
		}
	}

	result := reward{
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}
	result.Reward = (float64(result.Build+result.NoPython) + 4*result.Behaviour) / 6
	if err := writeReward(result); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "reward: %.6f (build=%d no_python=%d behaviour=%.6f, %d/%d cases)\n",
		result.Reward, result.Build, result.NoPython, result.Behaviour, matches, total)

	if result.Behaviour == 1.0 {
		if err := unlockSecret(); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(os.Stdout, "No secret for you. Yet.")
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "grader error: %v\n", err)
		os.Exit(1)
	}
}
