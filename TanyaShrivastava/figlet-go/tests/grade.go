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
	repoDir    = "/workspace/repo"
	binaryPath = "/tmp/figlet"
	dataPath   = "/tests/data/hidden.jsonl"
	rewardPath = "/logs/verifier/reward.json"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type rewardReport struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	n := len(data)
	remaining := b.limit - b.buffer.Len()
	if remaining > n {
		remaining = n
	}
	if remaining > 0 {
		_, _ = b.buffer.Write(data[:remaining])
	}
	return n, nil
}

func buildProgram() bool {
	if err := os.Remove(binaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "could not remove old binary: %v\n", err)
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, output)
		return false
	}
	return true
}

func hasNoPythonFiles(root string) (bool, error) {
	found := false
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".py") {
			found = true
		}
		return nil
	})
	return !found, err
}

func runCase(args []string, expectedLength int) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	output := &boundedBuffer{limit: expectedLength + 1}
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return output.buffer.Bytes(), err == nil
}

func runHiddenCases() (behaviour float64, matches, total int, err error) {
	file, err := os.Open(dataPath)
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for scanner.Scan() {
		total++
		var tc testCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			fmt.Fprintf(os.Stderr, "hidden case %d has invalid JSON: %v\n", total, err)
			continue
		}
		got, ok := runCase(tc.Args, len(tc.Out))
		if ok && bytes.Equal(got, []byte(tc.Out)) {
			matches++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, matches, total, err
	}
	if total > 0 {
		behaviour = float64(matches) / float64(total)
	}
	return behaviour, matches, total, nil
}

func unlockSecret() error {
	encoded, err := os.ReadFile("/tests/secret.b64")
	if err != nil {
		return err
	}
	secret, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		return err
	}

	cmd := exec.Command(binaryPath, "-f", "slant", string(secret))
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if _, err := fmt.Println(" SECRET UNLOCKED "); err != nil {
		return err
	}
	_, err = os.Stdout.Write(output)
	return err
}

func run() error {
	build := buildProgram()
	behaviour, matches, _, caseErr := runHiddenCases()
	if caseErr != nil {
		fmt.Fprintf(os.Stderr, "could not score hidden cases: %v\n", caseErr)
		behaviour = 0
	}

	noPython := 0.0
	if build {
		present, err := hasNoPythonFiles(repoDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not inspect repository for Python files: %v\n", err)
		} else if present && matches > 0 {
			noPython = 1
		}
	}

	buildScore := 0.0
	if build {
		buildScore = 1
	}
	report := rewardReport{
		Reward:    (buildScore + noPython + 4*behaviour) / 6,
		Build:     buildScore,
		NoPython:  noPython,
		Behaviour: behaviour,
	}
	rewardJSON, err := json.Marshal(report)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(rewardPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(rewardPath, append(rewardJSON, '\n'), 0644); err != nil {
		return err
	}

	if behaviour == 1 {
		return unlockSecret()
	}
	_, err = fmt.Println("No secret for you. Yet.")
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
