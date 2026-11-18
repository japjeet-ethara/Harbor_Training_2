// grade.go - verifier for the pyfiglet -> Go task. Go standard library only.
//
// Scores (weights in brackets):
//
//	build      [1]  `go build -o /tmp/figlet .` in /workspace/repo succeeds
//	no_python  [1]  build passed AND no .py file under /workspace/repo
//	                AND at least one hidden case matches
//	behaviour  [4]  share of /tests/data/hidden.jsonl cases whose stdout
//	                equals "out" exactly (5 s timeout per case)
//
// reward = (build*1 + no_python*1 + behaviour*4) / 6, written to
// /logs/verifier/reward.json.
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
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	repoDir     = "/workspace/repo"
	binPath     = "/tmp/figlet"
	casesPath   = "/tests/data/hidden.jsonl"
	secretPath  = "/tests/secret.b64"
	rewardDir   = "/logs/verifier"
	rewardPath  = "/logs/verifier/reward.json"
	caseTimeout = 5 * time.Second
	buildTime   = 5 * time.Minute
	// Stop starting new cases after this long, so a slow or hanging binary
	// cannot push the verifier past its 900 s limit. Unstarted cases fail.
	behaviourBudget = 600 * time.Second

	wBuild, wNoPython, wBehaviour = 1.0, 1.0, 4.0
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
	var r reward

	// 1. build
	r.Build = build()
	fmt.Printf("build: %.0f\n", r.Build)

	// 3. behaviour (needed by no_python, so it runs first)
	matched, total := 0, 0
	if r.Build == 1 {
		cases, err := loadCases(casesPath)
		if err != nil {
			fmt.Println("cannot read cases:", err)
		} else {
			total = len(cases)
			matched = runCases(cases)
		}
	}
	if total > 0 {
		r.Behaviour = float64(matched) / float64(total)
	}
	fmt.Printf("behaviour: %d/%d = %.4f\n", matched, total, r.Behaviour)

	// 2. no_python
	pyFiles := findPy(repoDir)
	if r.Build == 1 && len(pyFiles) == 0 && matched >= 1 {
		r.NoPython = 1
	}
	fmt.Printf("no_python: %.0f (build=%.0f, .py files=%d, matched=%d)\n",
		r.NoPython, r.Build, len(pyFiles), matched)
	for i, p := range pyFiles {
		if i == 5 {
			fmt.Println("  ...")
			break
		}
		fmt.Println("  found:", p)
	}

	// 4. reward
	r.Reward = (wBuild*r.Build + wNoPython*r.NoPython + wBehaviour*r.Behaviour) /
		(wBuild + wNoPython + wBehaviour)
	writeReward(r)
	fmt.Printf("reward: %.4f\n", r.Reward)

	// 5. secret
	fmt.Println()
	if r.Behaviour == 1.0 {
		fmt.Println("*** SECRET UNLOCKED ***")
		fmt.Print(secret())
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func build() float64 {
	_ = os.Remove(binPath) // never grade a stale binary
	ctx, cancel := context.WithTimeout(context.Background(), buildTime)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("go build failed: %v\n%s\n", err, tail(out, 2000))
		return 0
	}
	if _, err := os.Stat(binPath); err != nil {
		fmt.Println("go build produced no binary")
		return 0
	}
	return 1
}

func loadCases(path string) ([]testCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cases []testCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var c testCase
		if err := json.Unmarshal(line, &c); err != nil {
			return nil, fmt.Errorf("bad case line: %w", err)
		}
		cases = append(cases, c)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, errors.New("no cases")
	}
	return cases, nil
}

// runCases runs every case in parallel from an empty working directory
// (the program must work from any directory) and returns how many match.
func runCases(cases []testCase) int {
	workDir, err := os.MkdirTemp("", "figlet-cwd-")
	if err != nil {
		workDir = "/"
	}
	defer os.RemoveAll(workDir)

	deadline := time.Now().Add(behaviourBudget)
	ok := make([]bool, len(cases))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	if workers < 4 {
		workers = 4
	}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if time.Now().After(deadline) {
					continue // out of time: case fails
				}
				ok[i] = runOne(cases[i], workDir)
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	matched, shown := 0, 0
	for i, good := range ok {
		if good {
			matched++
		} else if shown < 10 {
			fmt.Printf("  mismatch: %q\n", cases[i].Args)
			shown++
		}
	}
	return matched
}

func runOne(c testCase, workDir string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, c.Args...)
	cmd.Dir = workDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	cmd.WaitDelay = time.Second
	_ = cmd.Run() // exit code and stderr are not graded
	if ctx.Err() != nil {
		return false // timed out
	}
	return bytes.Equal(stdout.Bytes(), []byte(c.Out))
}

func findPy(root string) []string {
	var found []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".py") {
			found = append(found, p)
		}
		return nil
	})
	return found
}

func writeReward(r reward) {
	if err := os.MkdirAll(rewardDir, 0o755); err != nil {
		fmt.Println("cannot create", rewardDir, err)
	}
	data, _ := json.Marshal(r)
	if err := os.WriteFile(rewardPath, data, 0o644); err != nil {
		fmt.Println("cannot write reward:", err)
	}
}

func secret() string {
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		return "(secret file missing)\n"
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return "(secret file unreadable)\n"
	}
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	out, _ := exec.CommandContext(ctx, binPath, "-f", "slant", string(text)).Output()
	return string(out)
}

func tail(b []byte, n int) []byte {
	if len(b) > n {
		return b[len(b)-n:]
	}
	return b
}
