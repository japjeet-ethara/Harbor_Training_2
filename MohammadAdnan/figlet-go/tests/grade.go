// Grader for the figlet-go Harbor task. Go standard library only (the room has no Python).
//
//	build      (weight 1): `go build -o /tmp/figlet .` and `go build ./...` pass in /workspace/repo,
//	                       and go.mod has no `require` lines (standard library only).
//	no_python  (weight 1): build passed AND no .py file under /workspace/repo
//	                       AND at least one hidden case matches.
//	behaviour  (weight 4): share of hidden cases whose stdout equals "out" byte for byte.
//
// Writes /logs/verifier/reward.json. If behaviour is exactly 1.0 the secret is drawn.
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
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	repo       = "/workspace/repo"
	binary     = "/tmp/figlet"
	hiddenPath = "/tests/data/hidden.jsonl"
	secretPath = "/tests/secret.b64"
	rewardPath = "/logs/verifier/reward.json"
	caseTime   = 5 * time.Second
	playerUID  = 1000
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

// candidate returns a command that runs the agent's binary from "/" (any working directory must
// work). When the grader is root, the binary runs as the unprivileged player, so it cannot read
// /tests (answers, secret) or write /logs.
func candidate(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = "/"
	cmd.Env = []string{"PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin", "HOME=/tmp", "LANG=C.UTF-8"}
	if os.Geteuid() == 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: playerUID, Gid: playerUID}}
	}
	return cmd
}

func lockTests() {
	if os.Geteuid() != 0 {
		return
	}
	_ = filepath.WalkDir("/tests", func(p string, d fs.DirEntry, err error) error {
		if err == nil {
			_ = os.Chown(p, 0, 0)
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
	_ = os.Chmod("/logs", 0o700)
}

func run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	return cmd.Run()
}

func checkBuild() bool {
	_ = os.Remove(binary)
	if err := run(repo, "go", "build", "-buildvcs=false", "-o", binary, "."); err != nil {
		fmt.Println("build: go build -o /tmp/figlet . failed:", err)
		return false
	}
	if err := run(repo, "go", "build", "-buildvcs=false", "./..."); err != nil {
		fmt.Println("build: go build ./... failed:", err)
		return false
	}
	mod, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		fmt.Println("build: no go.mod")
		return false
	}
	for _, line := range strings.Split(string(mod), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "require") {
			fmt.Println("build: go.mod has a require line (standard library only)")
			return false
		}
	}
	_ = os.Chmod(binary, 0o755)
	return true
}

func hasPython() bool {
	found := false
	_ = filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".py") {
			fmt.Println("no_python: found", p)
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
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var c testCase
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, sc.Err()
}

func runCase(c testCase) bool {
	ctx, cancel := context.WithTimeout(context.Background(), caseTime)
	defer cancel()
	var out bytes.Buffer
	cmd := candidate(ctx, c.Args...)
	cmd.Stdout = &out
	_ = cmd.Run()
	return ctx.Err() == nil && out.String() == c.Out
}

func behaviour(cases []testCase) (float64, int) {
	ok := make([]bool, len(cases))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU()*2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				ok[i] = runCase(cases[i])
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	passed, shown := 0, 0
	for i, good := range ok {
		if good {
			passed++
		} else if shown < 3 {
			shown++
			fmt.Printf("behaviour: case %d differs\n", i+1)
		}
	}
	return float64(passed) / float64(len(cases)), passed
}

func showSecret() {
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		fmt.Println("secret: missing", err)
		return
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("secret: bad base64", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), caseTime)
	defer cancel()
	out, _ := candidate(ctx, "-f", "slant", string(text)).Output()
	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(string(out))
}

func main() {
	lockTests()
	_ = os.MkdirAll(filepath.Dir(rewardPath), 0o755)

	build := 0.0
	if checkBuild() {
		build = 1
	}

	beh, passed := 0.0, 0
	if build == 1 {
		cases, err := loadCases(hiddenPath)
		if err != nil || len(cases) == 0 {
			fmt.Println("cannot read hidden cases:", err)
		} else {
			beh, passed = behaviour(cases)
			fmt.Printf("behaviour: %d/%d hidden cases match\n", passed, len(cases))
		}
	}

	noPython := 0.0
	if build == 1 && !hasPython() && passed > 0 {
		noPython = 1
	}

	reward := (1*build + 1*noPython + 4*beh) / 6
	result := map[string]float64{"reward": reward, "build": build, "no_python": noPython, "behaviour": beh}
	data, _ := json.Marshal(result)
	if err := os.WriteFile(rewardPath, data, 0o644); err != nil {
		fmt.Println("cannot write reward:", err)
		os.Exit(1)
	}
	fmt.Println(string(data))

	if beh == 1.0 {
		showSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
