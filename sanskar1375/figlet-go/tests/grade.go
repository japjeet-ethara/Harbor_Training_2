// grade.go — verifier for the figlet-go Harbor task. Go standard library only.
//
//	build      (weight 1): `go build -o /tmp/figlet .` and `go build ./...` succeed in
//	                       /workspace/repo, offline, with an empty module cache
//	                       (so any third-party module makes the build fail).
//	no_python  (weight 1): build passed AND no *.py file under /workspace/repo AND at
//	                       least one hidden case is answered correctly.
//	behaviour  (weight 4): share of cases in /tests/data/hidden.jsonl whose stdout equals
//	                       "out" byte for byte (5 s timeout each, run from an empty dir).
//
// Writes /logs/verifier/reward.json. If behaviour is exactly 1.0 the secret in
// /tests/secret.b64 is drawn by the candidate's own /tmp/figlet.
//
// Paths can be overridden with GRADE_REPO, GRADE_TESTS, GRADE_LOGS, GRADE_BIN
// (only used when the author tests the grader outside Harbor).
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

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var (
	repoDir  = env("GRADE_REPO", "/workspace/repo")
	testsDir = env("GRADE_TESTS", "/tests")
	logsDir  = env("GRADE_LOGS", "/logs/verifier")
	binPath  = env("GRADE_BIN", "/tmp/figlet")
)

const caseTimeout = 5 * time.Second

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

// goEnv is a clean environment for building the candidate: offline, no user
// go.env / GOFLAGS / go.work tricks, fresh caches.
func goEnv(scratch string) []string {
	keep := []string{}
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		if strings.HasPrefix(k, "GO") || k == "CGO_ENABLED" {
			continue
		}
		keep = append(keep, kv)
	}
	return append(keep,
		"GOENV=off",
		"GOFLAGS=-mod=mod",
		"GOPROXY=off",
		"GOWORK=off",
		"GOTOOLCHAIN=local",
		"CGO_ENABLED=0",
		"GOCACHE="+filepath.Join(scratch, "cache"),
		"GOMODCACHE="+filepath.Join(scratch, "modcache"),
		"GOPATH="+filepath.Join(scratch, "gopath"),
		"HOME="+scratch,
	)
}

func run(dir string, envv []string, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = envv
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func checkBuild(scratch string) bool {
	_ = os.Remove(binPath)
	e := goEnv(scratch)
	if out, err := run(repoDir, e, 5*time.Minute, "go", "build", "-o", binPath, "."); err != nil {
		fmt.Printf("build: FAIL (go build -o %s .): %v\n%s\n", binPath, err, tail(out))
		return false
	}
	discard := filepath.Join(scratch, "all") + string(os.PathSeparator)
	if err := os.MkdirAll(discard, 0o755); err != nil {
		fmt.Println("build: cannot make scratch dir:", err)
		return false
	}
	if out, err := run(repoDir, e, 5*time.Minute, "go", "build", "-o", discard, "./..."); err != nil {
		fmt.Printf("build: FAIL (go build ./...): %v\n%s\n", err, tail(out))
		return false
	}
	fmt.Println("build: ok")
	return true
}

func tail(s string) string {
	if len(s) > 2000 {
		return "..." + s[len(s)-2000:]
	}
	return s
}

func pyFiles() []string {
	var found []string
	_ = filepath.WalkDir(repoDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			found = append(found, p)
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
	return cases, sc.Err()
}

// runFiglet runs the candidate binary from an empty directory with a 5 s limit.
func runFiglet(workdir string, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = workdir
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8"}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return stdout.Bytes(), errors.New("timeout")
	}
	return stdout.Bytes(), err
}

func behaviour(cases []testCase) (passed int) {
	workdir, err := os.MkdirTemp("", "figlet-cwd-")
	if err != nil {
		fmt.Println("behaviour: cannot make temp dir:", err)
		return 0
	}
	defer os.RemoveAll(workdir)

	ok := make([]bool, len(cases))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < max(2, runtime.NumCPU()); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				out, _ := runFiglet(workdir, cases[i].Args) // exit code is not graded
				ok[i] = string(out) == cases[i].Out
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	shown := 0
	for i, good := range ok {
		if good {
			passed++
		} else if shown < 5 {
			shown++
			a, _ := json.Marshal(cases[i].Args)
			fmt.Printf("behaviour: mismatch on args %s\n", a)
		}
	}
	return passed
}

func writeReward(r map[string]float64) {
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		fmt.Println("cannot create", logsDir, err)
	}
	b, _ := json.Marshal(r)
	if err := os.WriteFile(filepath.Join(logsDir, "reward.json"), b, 0o644); err != nil {
		fmt.Println("cannot write reward.json:", err)
		os.Exit(1)
	}
	fmt.Println("reward.json:", string(b))
}

func secret() {
	raw, err := os.ReadFile(filepath.Join(testsDir, "secret.b64"))
	if err != nil {
		fmt.Println("(no secret file)")
		return
	}
	msg, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("(secret file is not valid base64)")
		return
	}
	out, _ := runFiglet(os.TempDir(), []string{"-f", "slant", string(msg)})
	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(string(out))
}

func main() {
	scratch, err := os.MkdirTemp("", "figlet-grade-")
	if err != nil {
		fmt.Println("cannot make scratch dir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(scratch)

	build := checkBuild(scratch)

	beh := 0.0
	passed, total := 0, 0
	if build {
		cases, err := loadCases(filepath.Join(testsDir, "data", "hidden.jsonl"))
		if err != nil || len(cases) == 0 {
			fmt.Println("behaviour: cannot load hidden cases:", err)
		} else {
			total = len(cases)
			passed = behaviour(cases)
			beh = float64(passed) / float64(total)
		}
	}
	fmt.Printf("behaviour: %d / %d\n", passed, total)

	noPy := 0.0
	py := pyFiles()
	switch {
	case !build:
		fmt.Println("no_python: 0 (build failed)")
	case len(py) > 0:
		fmt.Printf("no_python: 0 (%d .py files, e.g. %s)\n", len(py), py[0])
	case passed == 0:
		fmt.Println("no_python: 0 (no hidden case answered correctly)")
	default:
		noPy = 1
		fmt.Println("no_python: 1")
	}

	b := 0.0
	if build {
		b = 1
	}
	writeReward(map[string]float64{
		"reward":    (1*b + 1*noPy + 4*beh) / 6,
		"build":     b,
		"no_python": noPy,
		"behaviour": beh,
	})

	if total > 0 && passed == total {
		secret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
