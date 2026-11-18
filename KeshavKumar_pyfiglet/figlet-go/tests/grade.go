// grade.go: Harbor verifier for the figlet-go task. Go standard library only.
//
// Started by test.sh as root. The agent's program is always run as the
// unprivileged user "player", so it cannot read /tests (hidden answers, secret).
//
//	build      (weight 1): `go build -o /tmp/figlet .` and `go build ./...` succeed
//	no_python  (weight 1): build passed AND no .py file under the repo AND at least
//	                       one hidden case matches
//	behaviour  (weight 4): share of hidden cases whose stdout equals "out" exactly
//
// Writes /logs/verifier/reward.json.
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
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	repoDir     = "/workspace/repo"
	binPath     = "/tmp/figlet"
	goBin       = "/usr/local/go/bin/go" // absolute: /go/bin is agent-writable
	hiddenPath  = "/tests/data/hidden.jsonl"
	secretPath  = "/tests/secret.b64"
	rewardPath  = "/logs/verifier/reward.json"
	caseTimeout = 5 * time.Second
	maxOutput   = 1 << 20 // bytes kept per run; real answers are far smaller
	showFails   = 5
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

// ---------- running the agent's program as "player" ----------

var playerCred *syscall.Credential // nil when we are not root (local debugging)
var playerUID = -1

func setupPlayer() {
	if os.Geteuid() != 0 {
		fmt.Println("WARNING: not root; the agent's program runs as the current user")
		return
	}
	uid, gid := 1000, 1000
	if u, err := user.Lookup("player"); err == nil {
		uid, _ = strconv.Atoi(u.Uid)
		gid, _ = strconv.Atoi(u.Gid)
	}
	playerUID = uid
	playerCred = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{}}
}

// killPlayerProcesses removes anything the agent left running (or the program
// spawned) so nothing can tamper with the grader's files.
func killPlayerProcesses() {
	if playerUID < 0 {
		return
	}
	entries, _ := os.ReadDir("/proc")
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + e.Name() + "/status")
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if f := strings.Fields(line); len(f) >= 2 && f[0] == "Uid:" {
				if f[1] == strconv.Itoa(playerUID) {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
				break
			}
		}
	}
}

type capped struct {
	buf      bytes.Buffer
	overflow bool
}

func (c *capped) Write(p []byte) (int, error) {
	if room := maxOutput - c.buf.Len(); room > 0 {
		if len(p) > room {
			c.buf.Write(p[:room])
			c.overflow = true
		} else {
			c.buf.Write(p)
		}
	} else if len(p) > 0 {
		c.overflow = true
	}
	return len(p), nil
}

// runFiglet runs the built program with args from a neutral working directory.
func runFiglet(args []string, workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = workDir
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/home/player", "LANG=C.UTF-8"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Credential: playerCred}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	out := &capped{}
	cmd.Stdout = out
	cmd.Stderr = nil // ignored (not graded)
	err := cmd.Run()
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // reap leftovers
	}
	if ctx.Err() != nil {
		return out.buf.String(), errors.New("timeout")
	}
	if out.overflow {
		return out.buf.String(), errors.New("output too large")
	}
	_ = err // exit status is not graded
	return out.buf.String(), nil
}

// ---------- the three checks ----------

func goCmd(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func checkBuild() bool {
	_ = os.RemoveAll(binPath)
	out, err := goCmd(5*time.Minute, "build", "-o", binPath, ".")
	if err != nil {
		fmt.Printf("build: `go build -o %s .` FAILED: %v\n%s\n", binPath, err, tail(out, 2000))
		return false
	}
	out, err = goCmd(5*time.Minute, "build", "./...")
	if err != nil {
		fmt.Printf("build: `go build ./...` FAILED: %v\n%s\n", err, tail(out, 2000))
		return false
	}
	_ = os.Chmod(binPath, 0o755)
	fmt.Println("build: OK")
	return true
}

func countPyFiles() int {
	n := 0
	_ = filepath.WalkDir(repoDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".py") {
			if n < 5 {
				fmt.Println("no_python: found", p)
			}
			n++
		}
		return nil
	})
	return n
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

func checkBehaviour(cases []testCase, workDir string) int {
	type result struct {
		i   int
		got string
		err error
	}
	results := make([]result, len(cases))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				got, err := runFiglet(cases[i].Args, workDir)
				results[i] = result{i, got, err}
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	passed, shown := 0, 0
	for _, r := range results {
		if r.err == nil && r.got == cases[r.i].Out {
			passed++
			continue
		}
		if shown < showFails {
			shown++
			fmt.Printf("behaviour: case %d FAILED args=%q", r.i, cases[r.i].Args)
			if r.err != nil {
				fmt.Printf(" (%v)", r.err)
			}
			fmt.Printf("\n--- expected ---\n%s--- got ---\n%s----------------\n",
				cases[r.i].Out, tail(r.got, 3000))
		}
	}
	return passed
}

// ---------- secret ----------

func revealSecret(workDir string) {
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		fmt.Println("(no secret file)")
		return
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("(secret file is not valid base64)")
		return
	}
	out, _ := runFiglet([]string{"-f", "slant", string(text)}, workDir)
	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(out)
}

// ---------- main ----------

func tail(s string, n int) string {
	if len(s) > n {
		return "..." + s[len(s)-n:]
	}
	return s
}

func round4(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }

func main() {
	setupPlayer()
	killPlayerProcesses() // nothing from the agent phase may run during grading

	workDir, err := os.MkdirTemp("", "figlet-cwd-") // not the repo: "any working directory"
	if err == nil {
		_ = os.Chmod(workDir, 0o755)
	}

	build, noPython, behaviour := 0.0, 0.0, 0.0
	passed, total := 0, 0

	built := checkBuild()
	if built {
		build = 1
	}

	cases, err := loadCases(hiddenPath)
	if err != nil {
		fmt.Println("cannot read hidden cases:", err)
	}
	total = len(cases)
	if built && total > 0 {
		passed = checkBehaviour(cases, workDir)
		behaviour = float64(passed) / float64(total)
	}
	fmt.Printf("behaviour: %d/%d cases match exactly\n", passed, total)

	pyFiles := countPyFiles()
	fmt.Printf("no_python: %d .py file(s) under %s\n", pyFiles, repoDir)
	if built && pyFiles == 0 && passed > 0 {
		noPython = 1
	}

	if built && total > 0 && passed == total {
		revealSecret(workDir)
	} else {
		fmt.Println("No secret for you. Yet.")
	}

	killPlayerProcesses() // nothing the program spawned may touch the reward file

	reward := (1*build + 1*noPython + 4*behaviour) / 6
	res := map[string]float64{
		"reward":    round4(reward),
		"build":     build,
		"no_python": noPython,
		"behaviour": round4(behaviour),
	}
	data, _ := json.Marshal(res)
	_ = os.MkdirAll(filepath.Dir(rewardPath), 0o755)
	_ = os.Remove(rewardPath)
	if err := os.WriteFile(rewardPath, append(data, '\n'), 0o644); err != nil {
		fmt.Println("cannot write reward:", err)
		os.Exit(1)
	}
	fmt.Println("reward.json:", string(data))
}
