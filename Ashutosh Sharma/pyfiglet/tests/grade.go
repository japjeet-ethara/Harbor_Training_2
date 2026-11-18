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
	testDir    = "/tests"
	binaryPath = "/tmp/figlet"
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

func readCases(path string) ([]testCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cases []testCase
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var tc testCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if tc.Args == nil {
			return nil, fmt.Errorf("%s:%d: missing args", path, line)
		}
		cases = append(cases, tc)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("%s contains no test cases", path)
	}
	return cases, nil
}

func runCase(args []string, expected string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || cmd.ProcessState == nil {
		return false
	}
	return bytes.Equal(stdout.Bytes(), []byte(expected))
}

func hasPythonFiles(root string) (bool, error) {
	found := false
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".py") {
			found = true
		}
		return nil
	})
	return found, err
}

func writeReward(result reward) error {
	if err := os.MkdirAll(filepath.Dir(rewardPath), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(rewardPath, data, 0o644)
}

func unlockSecret() error {
	encoded, err := os.ReadFile(filepath.Join(testDir, "secret.b64"))
	if err != nil {
		return err
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		return err
	}

	cmd := exec.Command(binaryPath, "-f", "slant", string(text))
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	fmt.Println("*** SECRET UNLOCKED ***")
	_, err = os.Stdout.Write(output)
	return err
}

func run() error {
	cases, err := readCases(filepath.Join(testDir, "data", "hidden.jsonl"))
	if err != nil {
		return err
	}

	build := 0.0
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = repoDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, output)
	} else {
		build = 1
	}

	behaviour := 0.0
	matchedAny := false
	if build == 1 {
		matches := 0
		for _, tc := range cases {
			if runCase(tc.Args, tc.Out) {
				matches++
				matchedAny = true
			}
		}
		behaviour = float64(matches) / float64(len(cases))
	}

	noPython := 0.0
	if build == 1 {
		found, err := hasPythonFiles(repoDir)
		if err != nil {
			return err
		}
		if !found && matchedAny {
			noPython = 1
		}
	}

	result := reward{
		Reward:    (build + noPython + 4*behaviour) / 6,
		Build:     build,
		NoPython:  noPython,
		Behaviour: behaviour,
	}
	if err := writeReward(result); err != nil {
		return err
	}

	if behaviour == 1 {
		if err := unlockSecret(); err != nil {
			return err
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
