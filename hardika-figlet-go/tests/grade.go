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

const (
	repoPath    = "/workspace/repo"
	programPath = "/tmp/figlet"
	casePath    = "/tests/data/hidden.jsonl"
	rewardPath  = "/logs/verifier/reward.json"
)

type testCase struct {
	Args []string
	Out  string
}

type evaluation struct {
	Built     bool
	HasPython bool
	Matched   int
	Total     int
}

type score struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func loadCases(r io.Reader) ([]testCase, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var cases []testCase
	seen := make(map[string]bool)
	for scanner.Scan() {
		var record struct {
			Args []string `json:"args"`
			Out  *string  `json:"out"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("case %d: %w", len(cases)+1, err)
		}
		if len(record.Args) == 0 || record.Out == nil {
			return nil, fmt.Errorf("case %d must have args and out", len(cases)+1)
		}
		key, err := json.Marshal(record.Args)
		if err != nil {
			return nil, err
		}
		if seen[string(key)] {
			return nil, fmt.Errorf("case %d repeats an argument vector", len(cases)+1)
		}
		seen[string(key)] = true
		cases = append(cases, testCase{record.Args, *record.Out})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("no grading cases")
	}
	return cases, nil
}

func compile(repo, target string) error {
	cmd := exec.Command("go", "build", "-o", target, ".")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build: %w\n%s", err, out)
	}
	return nil
}

func hasPython(repo string) (bool, error) {
	found := false
	err := filepath.WalkDir(repo, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".py" {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found, err
}

func runCase(target string, args []string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, target, args...)
	cmd.WaitDelay = time.Second
	return cmd.Output()
}

func calculate(e evaluation) score {
	var s score
	if e.Built {
		s.Build = 1
	}
	if e.Built && !e.HasPython && e.Matched > 0 {
		s.NoPython = 1
	}
	if e.Total > 0 {
		s.Behaviour = float64(e.Matched) / float64(e.Total)
	}
	s.Reward = (s.Build + s.NoPython + 4*s.Behaviour) / 6
	return s
}

func evaluate(cases []testCase) (score, error) {
	result := evaluation{Total: len(cases)}
	if err := compile(repoPath, programPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return calculate(result), nil
	}
	result.Built = true
	var err error
	result.HasPython, err = hasPython(repoPath)
	if err != nil {
		return score{}, err
	}
	diagnostics := 0
	for index, entry := range cases {
		actual, err := runCase(programPath, entry.Args, 5*time.Second)
		if err == nil && bytes.Equal(actual, []byte(entry.Out)) {
			result.Matched++
			continue
		}
		if diagnostics < 5 {
			fmt.Fprintf(os.Stderr, "case %d args %q, error %v\n  expected %q\n  actual   %q\n",
				index+1, entry.Args, err, diagnosticPreview(entry.Out), diagnosticPreview(string(actual)))
			diagnostics++
		}
	}
	return calculate(result), nil
}

func diagnosticPreview(text string) string {
	const limit = 240
	if len(text) > limit {
		return text[:limit] + "[…]"
	}
	return text
}

func writeReward(path string, result score) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".reward-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	encodeErr := json.NewEncoder(file).Encode(result)
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

func displayResult(result score) error {
	if result.Behaviour != 1 {
		fmt.Println("No secret for you. Yet.")
		return nil
	}
	data, err := os.ReadFile("/tests/secret.b64")
	if err != nil {
		return err
	}
	message, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return err
	}
	art, err := runCase(programPath, []string{"-f", "slant", "--", string(message)}, 5*time.Second)
	if err != nil {
		return err
	}
	fmt.Println("*** SECRET UNLOCKED ***")
	_, err = os.Stdout.Write(art)
	return err
}

func grade() error {
	file, err := os.Open(casePath)
	if err != nil {
		return err
	}
	defer file.Close()
	cases, err := loadCases(file)
	if err != nil {
		return err
	}
	result, err := evaluate(cases)
	if err != nil {
		return err
	}
	if err := writeReward(rewardPath, result); err != nil {
		return err
	}
	return displayResult(result)
}

func main() {
	if err := grade(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
