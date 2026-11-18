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
	"strings"
	"time"
)

const (
	repoDir       = "/workspace/repo"
	hiddenPath    = "/tests/data/hidden.jsonl"
	rewardPath    = "/logs/verifier/reward.json"
	figletPath    = "/tmp/figlet"
	caseTimeout   = 5 * time.Second
	secretTimeout = 5 * time.Second
)

type testCase struct {
	Args []string
	Out  string
	Line int
	Err  error
}

type reward struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func readCases(path string) ([]testCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var cases []testCase
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		current := testCase{Line: lineNumber}
		var record struct {
			Args []string `json:"args"`
			Out  *string  `json:"out"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			current.Err = fmt.Errorf("invalid JSON: %w", err)
		} else if record.Args == nil {
			current.Err = errors.New("args is missing or null")
		} else if record.Out == nil {
			current.Err = errors.New("out is missing or null")
		} else {
			current.Args = record.Args
			current.Out = *record.Out
		}
		cases = append(cases, current)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cases, nil
}

func buildProgram() (bool, error) {
	command := exec.Command("go", "build", "-o", figletPath, ".")
	command.Dir = repoDir
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	fmt.Printf("Building in %s: go build -o %s .\n", repoDir, figletPath)
	if err := command.Run(); err != nil {
		fmt.Printf("Build failed: %v\n", err)
		if stdout.Len() > 0 {
			fmt.Printf("Build stdout:\n%s", stdout.String())
		}
		if stderr.Len() > 0 {
			fmt.Printf("Build stderr:\n%s", stderr.String())
		}
		return false, nil
	}
	if stdout.Len() > 0 {
		fmt.Printf("Build stdout:\n%s", stdout.String())
	}
	if stderr.Len() > 0 {
		fmt.Printf("Build stderr:\n%s", stderr.String())
	}
	fmt.Println("Build succeeded.")
	return true, nil
}

func runProgram(args []string, timeout time.Duration) ([]byte, []byte, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command := exec.CommandContext(ctx, figletPath, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return stdout.Bytes(), stderr.Bytes(), true, nil
	}
	return stdout.Bytes(), stderr.Bytes(), false, err
}

func firstDifference(expected, actual []byte) int {
	limit := len(expected)
	if len(actual) < limit {
		limit = len(actual)
	}
	for i := 0; i < limit; i++ {
		if expected[i] != actual[i] {
			return i
		}
	}
	if len(expected) != len(actual) {
		return limit
	}
	return -1
}

func evaluateCases(cases []testCase, buildSucceeded bool, readErr error) (float64, int) {
	if readErr != nil {
		fmt.Printf("Cannot evaluate hidden cases: %v\n", readErr)
		return 0, 0
	}
	if !buildSucceeded {
		fmt.Println("Skipping hidden-case execution because the build failed.")
		return 0, 0
	}
	if len(cases) == 0 {
		fmt.Println("Hidden dataset contains no cases; behaviour score is 0.")
		return 0, 0
	}

	passed := 0
	const diagnosticLimit = 20
	diagnostics := 0
	diagnosticsSuppressed := 0
	for _, current := range cases {
		if current.Err != nil {
			if diagnostics < diagnosticLimit {
				fmt.Printf("Case at JSONL line %d: invalid record: %v\n", current.Line, current.Err)
				diagnostics++
			} else {
				diagnosticsSuppressed++
			}
			continue
		}

		stdout, stderr, timedOut, err := runProgram(current.Args, caseTimeout)
		switch {
		case timedOut:
			if diagnostics < diagnosticLimit {
				fmt.Printf("Case at JSONL line %d timed out after %s; args=%q\n",
					current.Line, caseTimeout, current.Args)
				diagnostics++
			} else {
				diagnosticsSuppressed++
			}
		case err != nil:
			if diagnostics < diagnosticLimit {
				fmt.Printf("Case at JSONL line %d execution failed: %v; args=%q; stderr=%q\n",
					current.Line, err, current.Args, stderr)
				diagnostics++
			} else {
				diagnosticsSuppressed++
			}
		case !bytes.Equal(stdout, []byte(current.Out)):
			if diagnostics < diagnosticLimit {
				fmt.Printf(
					"Case at JSONL line %d stdout mismatch: expected %d bytes, got %d; first differing byte %d; args=%q\n",
					current.Line, len(current.Out), len(stdout),
					firstDifference([]byte(current.Out), stdout), current.Args,
				)
				diagnostics++
			} else {
				diagnosticsSuppressed++
			}
		default:
			passed++
		}
	}

	if diagnosticsSuppressed > 0 {
		fmt.Printf(
			"Suppressed %d additional case diagnostics; limit is %d.\n",
			diagnosticsSuppressed,
			diagnosticLimit,
		)
	}
	score := float64(passed) / float64(len(cases))
	fmt.Printf("Behaviour: %d/%d cases passed (%.6f).\n", passed, len(cases), score)
	return score, passed
}

func containsPythonFiles(root string) (bool, error) {
	found := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".py") {
			found = true
		}
		return nil
	})
	return found, err
}

func unlockSecret() {
	encoded, err := os.ReadFile("/tests/secret.b64")
	if err != nil {
		fmt.Printf("Secret could not be read: %v\n", err)
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		fmt.Printf("Secret could not be base64-decoded: %v\n", err)
		return
	}

	stdout, stderr, timedOut, err := runProgram(
		[]string{"-f", "slant", string(decoded)},
		secretTimeout,
	)
	if timedOut {
		fmt.Printf("Secret rendering timed out after %s; stderr=%q\n", secretTimeout, stderr)
		return
	}
	if err != nil {
		fmt.Printf("Secret rendering failed: %v; stderr=%q\n", err, stderr)
		return
	}

	if _, err := os.Stdout.Write(stdout); err != nil {
		fmt.Printf("Could not write secret output: %v\n", err)
	}
}

func writeReward(result reward) error {
	if err := os.MkdirAll(filepath.Dir(rewardPath), 0755); err != nil {
		return fmt.Errorf("create reward directory: %w", err)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode reward JSON: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(rewardPath, encoded, 0644); err != nil {
		return fmt.Errorf("write reward file: %w", err)
	}
	fmt.Printf("Wrote reward to %s\n", rewardPath)
	return nil
}

func grade() error {
	cases, readErr := readCases(hiddenPath)
	if readErr != nil {
		fmt.Printf("Could not read hidden dataset %s: %v\n", hiddenPath, readErr)
	} else {
		fmt.Printf("Loaded %d hidden cases from %s.\n", len(cases), hiddenPath)
	}

	buildSucceeded, err := buildProgram()
	if err != nil {
		return err
	}
	behaviourScore, passed := evaluateCases(cases, buildSucceeded, readErr)

	hasPython, scanErr := containsPythonFiles(repoDir)
	if scanErr != nil {
		fmt.Printf("Could not scan %s for .py files: %v\n", repoDir, scanErr)
	} else if hasPython {
		fmt.Printf("Found at least one .py file under %s.\n", repoDir)
	} else {
		fmt.Printf("No .py files found under %s.\n", repoDir)
	}

	noPythonScore := 0.0
	if buildSucceeded && scanErr == nil && !hasPython && passed > 0 {
		noPythonScore = 1
	}

	buildScore := 0.0
	if buildSucceeded {
		buildScore = 1
	}
	result := reward{
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}
	result.Reward = (result.Build + result.NoPython + 4*result.Behaviour) / 6
	fmt.Printf(
		"Scores: build=%.0f no_python=%.0f behaviour=%.6f reward=%.6f\n",
		result.Build, result.NoPython, result.Behaviour, result.Reward,
	)
	if err := writeReward(result); err != nil {
		return err
	}

	if result.Behaviour == 1 {
		fmt.Println("*** SECRET UNLOCKED ***")
		unlockSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
	return nil
}

func main() {
	if err := grade(); err != nil {
		fmt.Fprintf(os.Stderr, "Grader error: %v\n", err)
		os.Exit(1)
	}
}
