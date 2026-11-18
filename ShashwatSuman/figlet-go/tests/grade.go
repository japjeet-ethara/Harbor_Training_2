// grade.go — verifier for the figlet-go task. Go standard library only.
//
//	build      (weight 1): `go build -o /tmp/figlet .` in /workspace/repo works
//	no_python  (weight 1): build passed AND no .py file under /workspace/repo
//	                       AND at least one hidden case matches
//	behaviour  (weight 4): share of hidden cases whose stdout matches exactly
//
// Writes /logs/verifier/reward.json. If behaviour == 1.0 it draws the secret.
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
	repoDir    = "/workspace/repo"
	binary     = "/tmp/figlet"
	hiddenPath = "/tests/data/hidden.jsonl"
	secretPath = "/tests/secret.b64"
	rewardDir  = "/logs/verifier"
	caseLimit  = 5 * time.Second
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func runFiglet(args []string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = "/"
	cmd.WaitDelay = time.Second
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if ctx.Err() != nil {
		return out.Bytes(), ctx.Err()
	}
	return out.Bytes(), err
}

func build() bool {
	_ = os.Remove(binary) // never grade a binary left behind by the agent
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("build FAILED: %v\n%s\n", err, out)
		return false
	}
	fmt.Println("build ok")
	return true
}

func pythonFiles() []string {
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

func behaviour(cases []testCase) (int, []int) {
	results := make([]bool, len(cases))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				got, err := runFiglet(cases[i].Args, caseLimit)
				results[i] = err == nil && bytes.Equal(got, []byte(cases[i].Out))
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
	for i, ok := range results {
		if ok {
			passed++
		} else {
			failed = append(failed, i)
		}
	}
	return passed, failed
}

func writeReward(r map[string]float64) {
	_ = os.MkdirAll(rewardDir, 0o755)
	b, _ := json.Marshal(r)
	if err := os.WriteFile(filepath.Join(rewardDir, "reward.json"), b, 0o644); err != nil {
		fmt.Println("cannot write reward:", err)
		os.Exit(1)
	}
	fmt.Println("reward.json:", string(b))
}

func showSecret() {
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		fmt.Println("secret missing:", err)
		return
	}
	msg, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("secret unreadable:", err)
		return
	}
	out, err := runFiglet([]string{"-f", "slant", string(msg)}, 10*time.Second)
	fmt.Println("*** SECRET UNLOCKED ***")
	os.Stdout.Write(out)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		fmt.Println("(figlet exited with:", err, ")")
	}
}

func main() {
	reward := map[string]float64{"reward": 0, "build": 0, "no_python": 0, "behaviour": 0}
	writeReward(reward) // always overwrite anything the agent may have left

	built := build()
	if built {
		reward["build"] = 1
	}

	cases, err := loadCases(hiddenPath)
	if err != nil || len(cases) == 0 {
		fmt.Println("cannot load hidden cases:", err)
		writeReward(reward)
		return
	}

	passed := 0
	if built {
		var failed []int
		passed, failed = behaviour(cases)
		fmt.Printf("behaviour: %d / %d cases match\n", passed, len(cases))
		for k, i := range failed {
			if k == 3 {
				break
			}
			a, _ := json.Marshal(cases[i].Args)
			fmt.Printf("  mismatch: %s\n", a)
		}
	}
	reward["behaviour"] = float64(passed) / float64(len(cases))

	py := pythonFiles()
	if len(py) > 0 {
		fmt.Printf("found %d .py files, e.g. %s\n", len(py), py[0])
	}
	if built && len(py) == 0 && passed > 0 {
		reward["no_python"] = 1
	}

	reward["reward"] = (1*reward["build"] + 1*reward["no_python"] + 4*reward["behaviour"]) / 6
	writeReward(reward)

	if passed == len(cases) {
		showSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
