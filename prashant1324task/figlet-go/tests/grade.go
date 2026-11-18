// grade.go: verifier for the figlet-go task. Go standard library only.
//
// Run by tests/test.sh as:  cd /tests && go run grade.go
//
// Score = (1*build + 1*no_python + 4*behaviour) / 6
//
//	build      1 if `go build -o /tmp/figlet .` works in /workspace/repo
//	no_python  1 only if build passed AND no *.py exists under /workspace/repo
//	           AND at least one hidden case matches (so deleting files alone pays 0)
//	behaviour  share of hidden cases whose stdout equals "out" exactly
//
// If behaviour is exactly 1.0 the secret is drawn by the solver's own program.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	repoDir    = "/workspace/repo"
	binPath    = "/tmp/figlet"
	hiddenPath = "/tests/data/hidden.jsonl"
	secretPath = "/tests/secret.b64"
	logDir     = "/logs/verifier"

	wBuild     = 1.0
	wNoPython  = 1.0
	wBehaviour = 4.0

	caseTimeout  = 5 * time.Second
	buildTimeout = 600 * time.Second
	workers      = 8
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func loadCases(path string) ([]testCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []testCase
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var c testCase
		if err := json.Unmarshal(line, &c); err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, nil
}

func build() bool {
	_ = os.Remove(binPath) // never trust a stale binary
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("build FAILED: %v\n%s\n", err, out)
		return false
	}
	return true
}

func hasPythonFiles() bool {
	found := false
	_ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			if !found {
				fmt.Printf("python file still present: %s\n", path)
			}
			found = true
		}
		return nil
	})
	return found
}

// runCase runs the solver's program in a neutral working directory.
func runCase(c testCase) bool {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, c.Args...)
	cmd.Dir = os.TempDir() // "must work from any working directory"
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run() // exit code is not graded, only stdout
	if ctx.Err() != nil {
		return false
	}
	return stdout.String() == c.Out
}

func score(cases []testCase) (matched int, total int) {
	total = len(cases)
	var m int64
	jobs := make(chan testCase)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range jobs {
				if runCase(c) {
					atomic.AddInt64(&m, 1)
				}
			}
		}()
	}
	for _, c := range cases {
		jobs <- c
	}
	close(jobs)
	wg.Wait()
	return int(m), total
}

func writeReward(reward, b, np, beh float64) {
	_ = os.MkdirAll(logDir, 0o755)
	data, _ := json.Marshal(map[string]float64{
		"reward":    reward,
		"build":     b,
		"no_python": np,
		"behaviour": beh,
	})
	if err := os.WriteFile(filepath.Join(logDir, "reward.json"), data, 0o644); err != nil {
		fmt.Printf("could not write reward.json: %v\n", err)
	}
}

func main() {
	cases, err := loadCases(hiddenPath)
	if err != nil || len(cases) == 0 {
		fmt.Printf("cannot read hidden cases: %v\n", err)
		writeReward(0, 0, 0, 0)
		return
	}

	var b, np, beh float64
	matched := 0

	if build() {
		b = 1
		matched, _ = score(cases)
		beh = float64(matched) / float64(len(cases))
		if !hasPythonFiles() && matched > 0 {
			np = 1
		}
	}

	reward := (wBuild*b + wNoPython*np + wBehaviour*beh) / (wBuild + wNoPython + wBehaviour)
	writeReward(reward, b, np, beh)

	fmt.Printf("build=%.0f no_python=%.0f behaviour=%.4f (%d/%d) reward=%.4f\n",
		b, np, beh, matched, len(cases), reward)

	if matched == len(cases) {
		raw, err := os.ReadFile(secretPath)
		if err != nil {
			fmt.Println("(secret file missing)")
			return
		}
		msg, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil {
			fmt.Println("(secret file is not valid base64)")
			return
		}
		out, _ := exec.Command(binPath, "-f", "slant", string(msg)).Output()
		fmt.Println("*** SECRET UNLOCKED ***")
		fmt.Print(string(out))
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
