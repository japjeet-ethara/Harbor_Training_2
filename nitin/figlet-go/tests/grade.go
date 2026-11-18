// grade.go: the verifier for the figlet-go task. Go standard library only.
//
//	build      (weight 1): `go build -o /tmp/figlet .` works in /workspace/repo
//	no_python  (weight 1): build passed AND no .py file in the repo AND >= 1 hidden case matches
//	behaviour  (weight 4): share of cases in /tests/data/hidden.jsonl whose stdout equals "out" exactly
package main

import (
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
	"sync"
	"sync/atomic"
	"time"
)

const (
	repoDir     = "/workspace/repo"
	binPath     = "/tmp/figlet"
	hiddenPath  = "/tests/data/hidden.jsonl"
	secretPath  = "/tests/secret.b64"
	rewardPath  = "/logs/verifier/reward.json"
	caseTimeout = 5 * time.Second
	totalBudget = 600 * time.Second
	workers     = 8
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	// 1. build
	build := 0.0
	_ = os.Remove(binPath)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err == nil {
		build = 1.0
	} else {
		fmt.Printf("build failed: %v\n%s\n", err, truncate(string(out), 2000))
	}
	cancel()

	// 3. behaviour (computed before no_python, which needs "at least one match")
	cases := loadCases()
	var matched int64
	if build == 1.0 {
		matched = runCases(cases)
	}
	behaviour := 0.0
	if len(cases) > 0 {
		behaviour = float64(matched) / float64(len(cases))
	}

	// 2. no_python
	noPython := 0.0
	if build == 1.0 && matched >= 1 && !hasPythonFiles(repoDir) {
		noPython = 1.0
	}

	reward := (1.0*build + 1.0*noPython + 4.0*behaviour) / 6.0

	// 4. reward file
	if err := os.MkdirAll(filepath.Dir(rewardPath), 0o755); err != nil {
		fmt.Println("cannot create reward dir:", err)
	}
	b, _ := json.Marshal(map[string]float64{
		"reward": reward, "build": build, "no_python": noPython, "behaviour": behaviour,
	})
	if err := os.WriteFile(rewardPath, b, 0o644); err != nil {
		fmt.Println("cannot write reward:", err)
	}
	fmt.Printf("build=%.2f no_python=%.2f behaviour=%.4f (%d/%d) reward=%.4f\n",
		build, noPython, behaviour, matched, len(cases), reward)

	// 5. secret
	if behaviour == 1.0 {
		raw, err := os.ReadFile(secretPath)
		if err == nil {
			if dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw))); err == nil {
				out, _ := exec.Command(binPath, "-f", "slant", string(dec)).Output()
				fmt.Println("*** SECRET UNLOCKED ***")
				fmt.Print(string(out))
				return
			}
		}
	}
	fmt.Println("No secret for you. Yet.")
}

func loadCases() []testCase {
	f, err := os.Open(filepath.Clean(hiddenPath))
	if err != nil {
		fmt.Println("cannot open hidden cases:", err)
		return nil
	}
	defer f.Close()
	var cases []testCase
	dec := json.NewDecoder(f)
	for {
		var c testCase
		if err := dec.Decode(&c); err == io.EOF {
			break
		} else if err != nil {
			fmt.Println("bad case line:", err)
			break
		}
		cases = append(cases, c)
	}
	return cases
}

func runCases(cases []testCase) int64 {
	var matched int64
	deadline := time.Now().Add(totalBudget)
	jobs := make(chan testCase)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range jobs {
				if time.Now().After(deadline) {
					continue // out of time: counts as a miss
				}
				if runOne(c) {
					atomic.AddInt64(&matched, 1)
				}
			}
		}()
	}
	for _, c := range cases {
		jobs <- c
	}
	close(jobs)
	wg.Wait()
	return matched
}

func runOne(c testCase) bool {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, c.Args...)
	cmd.Dir = "/" // the program must work from any directory
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run() // exit code and stderr are not checked, only stdout
	return ctx.Err() == nil && stdout.String() == c.Out
}

func hasPythonFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
