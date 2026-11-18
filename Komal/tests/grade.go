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
	"sync"
	"time"
)

const (
	repo       = "/workspace/repo"
	executable = "/tmp/figlet"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type outcome struct {
	match bool
	issue string
}

type scores struct {
	Reward    float64 `json:"reward"`
	Build     int     `json:"build"`
	NoPython  int     `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func readCases(input io.Reader) ([]testCase, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var cases []testCase
	for scanner.Scan() {
		var item testCase
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return nil, fmt.Errorf("case %d: %w", len(cases)+1, err)
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
		return nil, fmt.Errorf("empty case file")
	}
	return cases, nil
}

type exactOutput struct {
	pending []byte
	differs bool
}

func (o *exactOutput) Write(p []byte) (int, error) {
	if !o.differs {
		if bytes.HasPrefix(o.pending, p) {
			o.pending = o.pending[len(p):]
		} else {
			o.differs = true
		}
	}
	return len(p), nil
}

func runCase(binary, dir string, limit time.Duration, item testCase) outcome {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, item.Args...)
	cmd.Dir = dir
	cmd.WaitDelay = time.Second
	output := exactOutput{pending: []byte(item.Out)}
	cmd.Stdout = &output
	err := cmd.Run()
	switch {
	case err != nil && ctx.Err() == context.DeadlineExceeded:
		return outcome{issue: "timeout"}
	case errors.Is(err, exec.ErrWaitDelay):
		return outcome{issue: "stdout drain timeout"}
	case err != nil:
		return outcome{issue: err.Error()}
	case output.differs || len(output.pending) != 0:
		return outcome{issue: "stdout mismatch"}
	default:
		return outcome{match: true}
	}
}

func runCases(cases []testCase) []outcome {
	outcomes := make([]outcome, len(cases))
	indices := make(chan int)
	var workers sync.WaitGroup
	for i := 0; i < min(12, len(cases)); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range indices {
				outcomes[index] = runCase(executable, "/tests", 5*time.Second, cases[index])
			}
		}()
	}
	for index := range cases {
		indices <- index
	}
	close(indices)
	workers.Wait()
	return outcomes
}

func reportOutcomes(outcomes []outcome) {
	matches := 0
	reported := 0
	for i, result := range outcomes {
		if result.match {
			matches++
		} else if reported < 5 {
			fmt.Printf("Case %d: %s\n", i+1, result.issue)
			reported++
		}
	}
	fmt.Printf("Matched %d of %d hidden cases\n", matches, len(outcomes))
}

func hasNoPython(dir string) bool {
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".py") {
			return fmt.Errorf("Python file %s", path)
		}
		return nil
	})
	return err == nil
}

func grade(built, clean bool, outcomes []outcome) scores {
	var score scores
	if !built {
		return score
	}
	score.Build = 1
	matched := 0
	for _, result := range outcomes {
		if result.match {
			matched++
		}
	}
	if len(outcomes) != 0 {
		score.Behaviour = float64(matched) / float64(len(outcomes))
	}
	if matched > 0 && clean {
		score.NoPython = 1
	}
	score.Reward = (float64(score.Build+score.NoPython) + 4*score.Behaviour) / 6
	return score
}

func reveal() error {
	data, err := os.ReadFile("/tests/secret.b64")
	if err != nil {
		return err
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-f", "slant", "--", string(text))
	cmd.Dir = "/tests"
	cmd.WaitDelay = 100 * time.Millisecond
	fmt.Println("*** SECRET UNLOCKED ***")
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func verify() error {
	file, err := os.Open("/tests/data/hidden.jsonl")
	if err != nil {
		return err
	}
	defer file.Close()
	cases, err := readCases(file)
	if err != nil {
		return err
	}
	build := exec.Command("go", "build", "-o", executable, ".")
	build.Dir = repo
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	built := build.Run() == nil
	outcomes := make([]outcome, len(cases))
	if built {
		outcomes = runCases(cases)
	}
	if built {
		reportOutcomes(outcomes)
	}
	score := grade(built, built && hasNoPython(repo), outcomes)
	data, err := json.MarshalIndent(score, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("/logs/verifier", 0755); err != nil {
		return err
	}
	if err := os.WriteFile("/logs/verifier/reward.json", append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Println(string(data))
	if score.Behaviour == 1 {
		return reveal()
	}
	fmt.Println("No secret for you. Yet.")
	return nil
}

func main() {
	if err := verify(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
