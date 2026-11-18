// grade.go - grader for the figlet-go Harbor task. Go standard library only.
//
// Rewards (weights 1 / 1 / 4):
//
//	build      1 if `go build -o /tmp/figlet .` works in /workspace/repo
//	no_python  1 only if build passed AND no .py file is left under the repo
//	           AND at least one hidden case matched (so "just delete the files" earns nothing)
//	behaviour  share of hidden cases whose stdout equals "out" byte for byte
//
// Writes /logs/verifier/reward.json. If behaviour is exactly 1.0 it also draws the secret.
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
	"time"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var (
	repoDir   = env("GRADE_REPO", "/workspace/repo")
	testsDir  = env("GRADE_TESTS", "/tests")
	logDir    = env("GRADE_LOGS", "/logs/verifier")
	binPath   = env("GRADE_BIN", "/tmp/figlet")
	caseLimit = 5 * time.Second
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type result struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func writeReward(r result) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		fmt.Println("cannot create log dir:", err)
	}
	b, _ := json.Marshal(r)
	if err := os.WriteFile(filepath.Join(logDir, "reward.json"), append(b, '\n'), 0o644); err != nil {
		fmt.Println("cannot write reward.json:", err)
	}
	fmt.Printf("reward.json: %s\n", b)
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

func build() bool {
	os.Remove(binPath) // never trust a binary the agent left behind
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", binPath, ".")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("go build failed: %v\n%s\n", err, out)
		return false
	}
	if st, err := os.Stat(binPath); err != nil || st.IsDir() {
		fmt.Println("go build produced no binary at", binPath)
		return false
	}
	return true
}

func runCase(c testCase) bool {
	ctx, cancel := context.WithTimeout(context.Background(), caseLimit)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, c.Args...)
	cmd.Dir = "/" // the program must work from any working directory
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run() // exit status and stderr are not graded
	if ctx.Err() != nil {
		return false
	}
	return stdout.String() == c.Out
}

func behaviour(cases []testCase) (passed int) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	jobs := make(chan testCase)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range jobs {
				if runCase(c) {
					mu.Lock()
					passed++
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
	return passed
}

func pythonFiles() []string {
	var found []string
	_ = filepath.WalkDir(repoDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			found = append(found, p)
		}
		return nil
	})
	return found
}

func secret() {
	raw, err := os.ReadFile(filepath.Join(testsDir, "secret.b64"))
	if err != nil {
		fmt.Println("(no secret file)")
		return
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("(secret file is not valid base64)")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), caseLimit)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "-f", "slant", string(text))
	cmd.Dir = "/"
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()
	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(out.String())
}

func main() {
	res := result{}
	defer func() { writeReward(res) }()

	cases, err := loadCases(filepath.Join(testsDir, "data", "hidden.jsonl"))
	if err != nil || len(cases) == 0 {
		fmt.Println("cannot load hidden cases:", err)
		return
	}

	built := build()
	passed := 0
	if built {
		res.Build = 1
		passed = behaviour(cases)
		res.Behaviour = float64(passed) / float64(len(cases))
	}

	py := pythonFiles()
	if built && len(py) == 0 && passed > 0 {
		res.NoPython = 1
	}
	res.Reward = (1*res.Build + 1*res.NoPython + 4*res.Behaviour) / 6

	fmt.Printf("build      : %.0f\n", res.Build)
	fmt.Printf("no_python  : %.0f  (%d .py files left)\n", res.NoPython, len(py))
	for i, p := range py {
		if i >= 5 {
			fmt.Println("             ...")
			break
		}
		fmt.Println("             ", p)
	}
	fmt.Printf("behaviour  : %.4f  (%d of %d cases)\n", res.Behaviour, passed, len(cases))
	fmt.Printf("reward     : %.4f\n", res.Reward)

	if built && passed == len(cases) {
		secret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
