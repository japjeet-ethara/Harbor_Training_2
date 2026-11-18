// grade.go — the figlet-go verifier. Go standard library only; needs no internet.
//
// Run by test.sh as `cd /tests && go run grade.go`. Writes /logs/verifier/reward.json:
//
//	build     (weight 1): `go build -o /tmp/figlet .` and `go build -o /dev/null ./...` pass
//	no_python (weight 1): build passed AND no .py file under the repo AND >= 1 hidden case matches
//	behaviour (weight 4): share of hidden cases whose stdout equals "out" exactly
//
// A perfect behaviour score unlocks the secret: /tests/secret.b64 drawn by the twin itself.
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

const (
	repo        = "/workspace/repo"
	binary      = "/tmp/figlet"
	hiddenPath  = "/tests/data/hidden.jsonl"
	secretPath  = "/tests/secret.b64"
	rewardPath  = "/logs/verifier/reward.json"
	caseTimeout = 5 * time.Second
	allCasesMax = 10 * time.Minute // stay inside the 900 s verifier timeout even if every case hangs
	workers     = 8
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	build := checkBuild()
	behaviour, matched := 0.0, 0
	if build == 1 {
		behaviour, matched = checkBehaviour()
	}
	noPython := 0.0
	if build == 1 && matched >= 1 && !hasPython() {
		noPython = 1
	}

	reward := (1*build + 1*noPython + 4*behaviour) / 6
	scores := map[string]float64{"reward": reward, "build": build, "no_python": noPython, "behaviour": behaviour}
	fmt.Printf("build=%v no_python=%v behaviour=%.4f reward=%.4f\n", build, noPython, behaviour, reward)
	writeReward(scores)

	if behaviour == 1.0 {
		showSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func checkBuild() float64 {
	os.Remove(binary) // never grade a stale binary left behind by the agent
	steps := [][]string{
		{"go", "build", "-o", binary, "."},
		{"go", "build", "-o", "/dev/null", "./..."},
	}
	for _, s := range steps {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		cmd := exec.CommandContext(ctx, s[0], s[1:]...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			fmt.Printf("build FAILED: %s\n%s\n", strings.Join(s, " "), tail(out, 2000))
			return 0
		}
	}
	fmt.Println("build ok")
	return 1
}

func loadHidden() []testCase {
	f, err := os.Open(hiddenPath)
	if err != nil {
		fmt.Println("cannot open hidden cases:", err)
		os.Exit(1)
	}
	defer f.Close()
	var cases []testCase
	dec := json.NewDecoder(bufio.NewReader(f))
	for dec.More() {
		var c testCase
		if err := dec.Decode(&c); err != nil {
			fmt.Println("bad hidden case:", err)
			os.Exit(1)
		}
		cases = append(cases, c)
	}
	return cases
}

func checkBehaviour() (float64, int) {
	cases := loadHidden()
	deadline := time.Now().Add(allCasesMax)
	var good atomic.Int64
	failed := make([]bool, len(cases))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if time.Now().Before(deadline) && runCase(cases[i]) {
					good.Add(1)
				} else {
					failed[i] = true
				}
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	shown := 0
	for i, f := range failed {
		if f && shown < 3 {
			fmt.Printf("  wrong: hidden case %d\n", i+1) // index only: answers stay out of logs
			shown++
		}
	}
	n := int(good.Load())
	fmt.Printf("behaviour: %d/%d hidden cases match\n", n, len(cases))
	return float64(n) / float64(len(cases)), n
}

func runCase(c testCase) bool {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, c.Args...)
	cmd.Dir = "/" // the binary must work outside the repository
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.WaitDelay = time.Second
	_ = cmd.Run() // exit code and stderr are not checked
	return ctx.Err() == nil && bytes.Equal(stdout.Bytes(), []byte(c.Out))
}

func hasPython() bool {
	found := false
	filepath.WalkDir(repo, func(path string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(d.Name(), ".py") {
			fmt.Println("python file left:", path)
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func writeReward(scores map[string]float64) {
	if err := os.MkdirAll(filepath.Dir(rewardPath), 0o755); err != nil {
		fmt.Println("cannot create log dir:", err)
	}
	data, _ := json.Marshal(scores)
	if err := os.WriteFile(rewardPath, data, 0o644); err != nil {
		fmt.Println("cannot write reward:", err)
		os.Exit(1)
	}
}

func showSecret() {
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		fmt.Println("secret missing:", err)
		return
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("secret unreadable:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-f", "slant", string(text))
	cmd.Dir = "/"
	out, _ := cmd.Output()
	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(string(out))
}

func tail(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return string(b)
}
