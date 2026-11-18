// Grader for the figlet-go Harbor task. Standard library only.
// Run as:  cd /tests && go run grade.go
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	rewardPath = "/logs/verifier/reward.json"

	wBuild     = 1.0
	wNoPython  = 1.0
	wBehaviour = 4.0
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("$ %s %s\n%s\n", name, strings.Join(args, " "), out)
	}
	return err
}

// stdlibOnly: go.mod must not require anything.
func stdlibOnly() bool {
	data, err := os.ReadFile(filepath.Join(repoDir, "go.mod"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "require") || strings.HasPrefix(l, "replace") {
			return false
		}
	}
	return true
}

func hasPyFile() bool {
	found := false
	filepath.Walk(repoDir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".py") {
			found = true
		}
		return nil
	})
	return found
}

func loadCases() []testCase {
	f, err := os.Open(hiddenPath)
	if err != nil {
		fmt.Println("cannot open hidden cases:", err)
		return nil
	}
	defer f.Close()
	var cases []testCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var c testCase
		if json.Unmarshal(sc.Bytes(), &c) == nil {
			cases = append(cases, c)
		}
	}
	return cases
}

func runCase(c testCase) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, c.Args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false
	}
	return stdout.String() == c.Out
}

func main() {
	os.MkdirAll(filepath.Dir(rewardPath), 0o755)

	// 1. build
	os.Remove(binPath)
	build := 0.0
	if run(repoDir, "go", "build", "-o", binPath, ".") == nil &&
		run(repoDir, "go", "build", "./...") == nil && stdlibOnly() {
		build = 1.0
	}

	// 3. behaviour (computed before 2, because no_python needs one right answer)
	cases := loadCases()
	matched := 0
	if build == 1.0 && len(cases) > 0 {
		var mu sync.Mutex
		var wg sync.WaitGroup
		jobs := make(chan testCase)
		for w := 0; w < 4; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for c := range jobs {
					if runCase(c) {
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
	}
	behaviour := 0.0
	if len(cases) > 0 {
		behaviour = float64(matched) / float64(len(cases))
	}

	// 2. no_python: build passed AND no .py anywhere AND at least one hidden case right
	noPython := 0.0
	if build == 1.0 && !hasPyFile() && matched >= 1 {
		noPython = 1.0
	}

	reward := (wBuild*build + wNoPython*noPython + wBehaviour*behaviour) / (wBuild + wNoPython + wBehaviour)
	rj := fmt.Sprintf("{\"reward\": %.6f, \"build\": %.6f, \"no_python\": %.6f, \"behaviour\": %.6f}\n",
		reward, build, noPython, behaviour)
	if err := os.WriteFile(rewardPath, []byte(rj), 0o644); err != nil {
		fmt.Println("cannot write reward:", err)
	}
	fmt.Printf("build=%.0f no_python=%.0f behaviour=%d/%d (%.4f) reward=%.4f\n",
		build, noPython, matched, len(cases), behaviour, reward)

	// 5. secret
	if len(cases) > 0 && matched == len(cases) {
		fmt.Println("*** SECRET UNLOCKED ***")
		raw, err := os.ReadFile(secretPath)
		if err == nil {
			msg, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
			if err == nil {
				cmd := exec.Command(binPath, "-f", "slant", string(msg))
				out, _ := cmd.Output()
				fmt.Print(string(out))
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
