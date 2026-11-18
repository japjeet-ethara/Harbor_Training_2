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
	repoDir     = "/workspace/repo"
	binaryPath  = "/tmp/figlet"
	hiddenPath  = "/tests/data/hidden.jsonl"
	secretPath  = "/tests/secret.b64"
	rewardPath  = "/logs/verifier/reward.json"
	caseTimeout = 5 * time.Second
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

type cappedOutput struct {
	data  []byte
	limit int
}

func (c *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := c.limit - len(c.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		c.data = append(c.data, p...)
	}
	return n, nil
}

func buildProgram() bool {
	if err := os.Remove(binaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "could not remove prior binary: %v\n", err)
		return false
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = repoDir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, output.Bytes())
		return false
	}
	return true
}

func loadCases() ([]testCase, error) {
	file, err := os.Open(hiddenPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	cases := make([]testCase, 0)
	line := 0
	for scanner.Scan() {
		line++
		var tc testCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", hiddenPath, line, err)
		}
		if len(tc.Args) == 0 {
			return nil, fmt.Errorf("%s:%d: args must not be empty", hiddenPath, line)
		}
		cases = append(cases, tc)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("%s contains no cases", hiddenPath)
	}
	return cases, nil
}

func runCases(cases []testCase) (matches int) {
	for _, tc := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
		cmd := exec.CommandContext(ctx, binaryPath, tc.Args...)
		output := &cappedOutput{limit: len(tc.Out) + 1}
		cmd.Stdout = output
		cmd.Stderr = io.Discard
		err := cmd.Run()
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		cancel()

		var exitErr *exec.ExitError
		startFailed := err != nil && !errors.As(err, &exitErr) && !timedOut
		if !timedOut && !startFailed && bytes.Equal(output.data, []byte(tc.Out)) {
			matches++
		}
	}
	return matches
}

func hasPythonFile(root string) (bool, error) {
	found := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".py") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found, err
}

func writeReward(result reward) error {
	if err := os.MkdirAll(filepath.Dir(rewardPath), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(rewardPath, data, 0644)
}

func unlockSecret() error {
	encoded, err := os.ReadFile(secretPath)
	if err != nil {
		return err
	}
	text, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return fmt.Errorf("decode %s: %w", secretPath, err)
	}

	cmd := exec.Command(binaryPath, "-f", "slant", string(text))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("render secret: %w", err)
	}
	if _, err := fmt.Println("*** SECRET UNLOCKED ***"); err != nil {
		return err
	}
	_, err = os.Stdout.Write(output.Bytes())
	return err
}

func run() error {
	buildPassed := buildProgram()
	result := reward{}
	if buildPassed {
		result.Build = 1
	}

	cases, err := loadCases()
	matches := 0
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not load hidden cases: %v\n", err)
	} else {
		matches = runCases(cases)
		result.Behaviour = float64(matches) / float64(len(cases))
	}

	if buildPassed {
		pythonFound, err := hasPythonFile(repoDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not inspect repository: %v\n", err)
		} else if !pythonFound && matches > 0 {
			result.NoPython = 1
		}
	}

	result.Reward = (result.Build + result.NoPython + 4*result.Behaviour) / 6
	if err := writeReward(result); err != nil {
		return fmt.Errorf("write reward: %w", err)
	}

	if result.Behaviour == 1 {
		if err := unlockSecret(); err != nil {
			fmt.Fprintf(os.Stderr, "secret unlock failed: %v\n", err)
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
