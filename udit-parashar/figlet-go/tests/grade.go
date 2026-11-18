// grade.go - verifier for the figlet-go Harbor task. Standard library only.
//
//	build      (weight 1): go build ./... and go build -o /tmp/figlet . both succeed, go.mod has no dependency
//	no_python  (weight 1): build passed AND no .py file under the repo AND >=1 hidden case matches
//	behaviour  (weight 4): share of hidden cases whose stdout equals "out" exactly
//
// Paths can be overridden for local testing: REPO_DIR, TESTS_DIR, LOGS_DIR, FIGLET_BIN.
package main

import (
	"bufio"
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

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func run(dir string, timeout time.Duration, name string, args ...string) ([]byte, error) {
	return runQ(dir, timeout, false, name, args...)
}

// runQ is run with optional silence: stderr of the solver's program is not echoed for test cases.
func runQ(dir string, timeout time.Duration, quiet bool, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil && !quiet && stderr.Len() > 0 {
		s := stderr.String()
		if len(s) > 2000 {
			s = s[:2000] + "..."
		}
		fmt.Fprintf(os.Stderr, "  [%s %s] %s\n", name, strings.Join(args, " "), s)
	}
	return stdout.Bytes(), err
}

func checkBuild(repo, bin string) bool {
	os.Remove(bin)
	// go.mod must exist and list no dependency (standard library only).
	mod, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		fmt.Println("build: no go.mod in repo root")
		return false
	}
	for _, line := range strings.Split(string(mod), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "require") || strings.HasPrefix(t, "replace") {
			fmt.Println("build: go.mod lists a dependency:", t)
			return false
		}
	}
	if _, err := run(repo, 10*time.Minute, "go", "build", "./..."); err != nil {
		fmt.Println("build: `go build ./...` failed:", err)
		return false
	}
	if _, err := run(repo, 10*time.Minute, "go", "build", "-o", bin, "."); err != nil {
		fmt.Println("build: `go build -o", bin, ".` failed:", err)
		return false
	}
	return true
}

func hasPython(repo string) bool {
	found := false
	filepath.WalkDir(repo, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			fmt.Println("no_python: found", path)
			found = true
		}
		return nil
	})
	return found
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
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var c testCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, sc.Err()
}

func main() {
	repo := env("REPO_DIR", "/workspace/repo")
	tests := env("TESTS_DIR", "/tests")
	logs := env("LOGS_DIR", "/logs/verifier")
	bin := env("FIGLET_BIN", "/tmp/figlet")

	// 1. build
	build := 0.0
	built := checkBuild(repo, bin)
	if built {
		build = 1
	}
	fmt.Printf("build      = %.0f\n", build)

	// 3. behaviour on the HIDDEN cases (never the visible file)
	behaviour := 0.0
	matched := int64(0)
	cases, err := loadCases(filepath.Join(tests, "data", "hidden.jsonl"))
	if err != nil {
		fmt.Println("cannot load hidden cases:", err)
	}
	if built && len(cases) > 0 {
		var wg sync.WaitGroup
		jobs := make(chan testCase)
		for w := 0; w < 8; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for c := range jobs {
					out, err := runQ(os.TempDir(), 5*time.Second, true, bin, c.Args...)
					if err == nil && string(out) == c.Out {
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
		behaviour = float64(matched) / float64(len(cases))
	}
	fmt.Printf("behaviour  = %.4f (%d/%d hidden cases)\n", behaviour, matched, len(cases))

	// 2. no_python: build passed AND no .py file AND at least one right answer
	noPython := 0.0
	if built && matched > 0 && !hasPython(repo) {
		noPython = 1
	}
	fmt.Printf("no_python  = %.0f\n", noPython)

	// 4. reward.json
	reward := (1*build + 1*noPython + 4*behaviour) / 6
	fmt.Printf("reward     = %.4f\n", reward)
	if err := os.MkdirAll(logs, 0o777); err != nil {
		fmt.Println("cannot create logs dir:", err)
	}
	b, _ := json.Marshal(map[string]float64{
		"reward": reward, "build": build, "no_python": noPython, "behaviour": behaviour,
	})
	if err := os.WriteFile(filepath.Join(logs, "reward.json"), b, 0o666); err != nil {
		fmt.Println("cannot write reward.json:", err)
		os.Exit(1)
	}

	// 5. the secret, drawn by the solver's own program
	if behaviour == 1.0 {
		raw, err := os.ReadFile(filepath.Join(tests, "secret.b64"))
		if err == nil {
			dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
			if err == nil {
				out, _ := run(os.TempDir(), 5*time.Second, bin, "-f", "slant", string(dec))
				fmt.Println("*** SECRET UNLOCKED ***")
				fmt.Print(string(out))
				return
			}
		}
		fmt.Println("(secret file unreadable)")
		return
	}
	fmt.Println("No secret for you. Yet.")
}
