// grade.go - verifier for the figlet-go Harbor task. Go standard library only.
//
//	build      (weight 1): `go build -o /tmp/figlet .` in /workspace/repo succeeds.
//	no_python  (weight 1): build passed AND no .py file under /workspace/repo
//	                       AND at least one hidden case matches.
//	behaviour  (weight 4): share of /tests/data/hidden.jsonl cases whose stdout
//	                       equals "out" exactly (5 s timeout per case).
//
// Writes /logs/verifier/reward.json. Prints a secret if behaviour == 1.0.
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

const (
	repoDir    = "/workspace/repo"
	binPath    = "/tmp/figlet"
	hiddenPath = "/tests/data/hidden.jsonl"
	secretPath = "/tests/secret.b64"
	rewardDir  = "/logs/verifier"
	caseTO     = 5 * time.Second
	workers    = 8
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func build() bool {
	_ = os.Remove(binPath) // never grade a binary left behind by the agent
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("build: FAILED: %v\n%s\n", err, out)
		return false
	}
	fmt.Println("build: ok")
	return true
}

func pyFiles() []string {
	var found []string
	_ = filepath.WalkDir(repoDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".py") {
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
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, sc.Err()
}

func run(args []string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), caseTO)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = os.TempDir() // the program must work from any working directory
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run() // exit code and stderr are not graded
	if ctx.Err() != nil {
		return "", false
	}
	return stdout.String(), true
}

func behaviour(cases []testCase) (int, []int) {
	ok := make([]bool, len(cases))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				got, finished := run(cases[i].Args)
				ok[i] = finished && got == cases[i].Out
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	passed := 0
	var failed []int
	for i, good := range ok {
		if good {
			passed++
		} else {
			failed = append(failed, i)
		}
	}
	return passed, failed
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func main() {
	buildOK := build()

	py := pyFiles()
	fmt.Printf("python files under %s: %d\n", repoDir, len(py))
	for i, p := range py {
		if i == 5 {
			fmt.Println("  ...")
			break
		}
		fmt.Println("  " + p)
	}

	cases, err := loadCases(hiddenPath)
	if err != nil || len(cases) == 0 {
		fmt.Printf("cannot load hidden cases: %v\n", err)
		os.Exit(1)
	}

	passed := 0
	if buildOK {
		var failed []int
		passed, failed = behaviour(cases)
		for i, idx := range failed {
			if i == 3 {
				break
			}
			fmt.Printf("mismatch #%d: args=%q\n", i+1, cases[idx].Args)
		}
	}
	beh := float64(passed) / float64(len(cases))
	noPython := buildOK && len(py) == 0 && passed >= 1

	reward := (1*b2f(buildOK) + 1*b2f(noPython) + 4*beh) / 6
	result := map[string]float64{
		"reward":    reward,
		"build":     b2f(buildOK),
		"no_python": b2f(noPython),
		"behaviour": beh,
	}
	fmt.Printf("behaviour: %d/%d\n", passed, len(cases))

	if err := os.MkdirAll(rewardDir, 0o777); err != nil {
		fmt.Println("mkdir:", err)
	}
	data, _ := json.Marshal(result)
	if err := os.WriteFile(filepath.Join(rewardDir, "reward.json"), data, 0o666); err != nil {
		fmt.Println("write reward:", err)
		os.Exit(1)
	}
	fmt.Println("reward.json:", string(data))

	if beh == 1.0 {
		raw, err := os.ReadFile(secretPath)
		text, err2 := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil || err2 != nil {
			fmt.Println("secret unreadable:", err, err2)
			return
		}
		fmt.Println("*** SECRET UNLOCKED ***")
		out, _ := run([]string{"-f", "slant", string(text)})
		fmt.Print(out)
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
