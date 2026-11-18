// Grader (stdlib only). Run as: cd /tests && go run grade.go
//   build (w=1)     : go.mod has no `require`, `go build ./...` and `go build -o /tmp/figlet .` work
//   no_python (w=1) : build ok AND no *.py under /workspace/repo AND >=1 hidden case matches
//   behaviour (w=4) : share of hidden cases whose stdout equals "out" exactly
// Secret is shown only if behaviour == 1.0.
package main

import (
	"bufio"
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
	"time"
)

const (
	repo   = "/workspace/repo"
	bin    = "/tmp/figlet"
	logDir = "/logs/verifier"
	hidden = "/tests/data/hidden.jsonl"
	secret = "/tests/secret.b64"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func goEnv() []string {
	return append(os.Environ(), "GOPROXY=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOFLAGS=")
}

func run(dir string, timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = goEnv()
	return cmd.Output()
}

func checkBuild() bool {
	os.Remove(bin)
	mod, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(mod), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "require") {
			return false
		}
	}
	if _, err := run(repo, 300*time.Second, "go", "build", "./..."); err != nil {
		return false
	}
	if _, err := run(repo, 300*time.Second, "go", "build", "-o", bin, "."); err != nil {
		return false
	}
	return true
}

func hasPython() bool {
	found := false
	filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".py") {
			found = true
		}
		return nil
	})
	return found
}

func loadCases() []testCase {
	f, err := os.Open(hidden)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open hidden cases:", err)
		return nil
	}
	defer f.Close()
	var cases []testCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var c testCase
		if json.Unmarshal(sc.Bytes(), &c) == nil {
			cases = append(cases, c)
		}
	}
	return cases
}

func behaviour(cases []testCase) (matched int) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	jobs := make(chan testCase)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range jobs {
				out, _ := run("/", 5*time.Second, bin, c.Args...) // from "/" : fonts must not depend on cwd
				if string(out) == c.Out {
					mu.Lock()
					matched++
					mu.Unlock()
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

func main() {
	os.MkdirAll(logDir, 0o755)
	os.Remove(filepath.Join(logDir, "reward.txt"))
	os.Remove(filepath.Join(logDir, "reward.json"))

	build := 0.0
	noPy, beh := 0.0, 0.0
	matched, total := 0, 0
	if checkBuild() {
		build = 1
		cases := loadCases()
		total = len(cases)
		if total > 0 {
			matched = behaviour(cases)
			beh = float64(matched) / float64(total)
		}
		if !hasPython() && matched > 0 {
			noPy = 1
		}
	}
	reward := (build*1 + noPy*1 + beh*4) / 6
	res := map[string]float64{"reward": reward, "build": build, "no_python": noPy, "behaviour": beh}
	b, _ := json.Marshal(res)
	if err := os.WriteFile(filepath.Join(logDir, "reward.json"), b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write reward:", err)
		os.Exit(1)
	}
	fmt.Printf("build=%.0f no_python=%.0f behaviour=%.4f (%d/%d) reward=%.4f\n", build, noPy, beh, matched, total, reward)

	if total > 0 && matched == total {
		raw, err := os.ReadFile(secret)
		if err == nil {
			msg, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
			if err == nil {
				out, _ := run("/", 10*time.Second, bin, "-f", "slant", string(msg))
				fmt.Println("*** SECRET UNLOCKED ***")
				fmt.Print(string(out))
				return
			}
		}
	}
	fmt.Println("No secret for you. Yet.")
}
