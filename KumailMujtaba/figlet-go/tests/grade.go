// grade.go: verifier for the pyfiglet-to-Go task. Go standard library only.
//
//	build      (weight 1): `go build -o /tmp/figlet .` in /workspace/repo succeeds.
//	no_python  (weight 1): build passed, no .py file under /workspace/repo,
//	                       and at least one hidden case matches.
//	behaviour  (weight 4): share of hidden cases whose stdout equals "out" exactly.
//
// Writes /logs/verifier/reward.json. Always exits 0 once the reward is written.
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
	"sync/atomic"
	"time"
)

const (
	repoDir     = "/workspace/repo"
	binPath     = "/tmp/figlet"
	casesPath   = "/tests/data/hidden.jsonl"
	secretPath  = "/tests/secret.b64"
	rewardDir   = "/logs/verifier"
	caseTimeout = 5 * time.Second
	buildLimit  = 5 * time.Minute
	// The verifier itself has a 900 s budget (task.toml). If the program is slow
	// or hangs on many cases, stop starting new cases after this long; cases not
	// run count as failures, and reward.json is still written.
	behaviourBudget = 600 * time.Second
	maxOutput       = 1 << 20 // cap captured stdout per case
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

// cappedBuffer keeps at most max bytes and records whether more were written.
type cappedBuffer struct {
	buf      bytes.Buffer
	max      int
	overflow bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := c.max - c.buf.Len(); room < len(p) {
		c.overflow = true
		if room > 0 {
			c.buf.Write(p[:room])
		}
		return len(p), nil
	}
	return c.buf.Write(p)
}

func gradeBuild() float64 {
	_ = os.Remove(binPath) // never grade a stale binary
	ctx, cancel := context.WithTimeout(context.Background(), buildLimit)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOTOOLCHAIN=local", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("build: FAILED: %v\n%s\n", err, tail(out, 40))
		return 0
	}
	if st, err := os.Stat(binPath); err != nil || st.IsDir() {
		fmt.Println("build: FAILED: no binary at", binPath)
		return 0
	}
	fmt.Println("build: ok")
	return 1
}

func findPython() []string {
	var found []string
	_ = filepath.WalkDir(repoDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".py") {
			found = append(found, p)
		}
		return nil
	})
	return found
}

func loadCases() ([]testCase, error) {
	f, err := os.Open(casesPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cases []testCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
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

// runFiglet runs the built program from workDir (outside the repo, so fonts
// must be found without relying on the working directory).
func runFiglet(ctx context.Context, workDir string, args []string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = workDir
	cmd.WaitDelay = time.Second // don't hang on children that keep stdout open
	out := &cappedBuffer{max: maxOutput}
	cmd.Stdout = out
	cmd.Stderr = nil
	_ = cmd.Run() // exit code is not graded, only stdout
	if ctx.Err() != nil || out.overflow {
		return nil, false
	}
	return out.buf.Bytes(), true
}

func gradeBehaviour(cases []testCase) (float64, int) {
	workDir, err := os.MkdirTemp("", "figlet-cwd-")
	if err != nil {
		workDir = "/"
	}
	defer os.RemoveAll(workDir)

	ctx, cancel := context.WithTimeout(context.Background(), behaviourBudget)
	defer cancel()

	var matched, timedOut, skipped atomic.Int64
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	if workers > 16 {
		workers = 16
	}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					skipped.Add(1)
					continue
				}
				got, ok := runFiglet(ctx, workDir, cases[i].Args)
				if !ok {
					timedOut.Add(1)
					continue
				}
				if string(got) == cases[i].Out {
					matched.Add(1)
				}
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	m := int(matched.Load())
	fmt.Printf("behaviour: %d/%d cases match (timeouts/oversized: %d, not run: %d)\n",
		m, len(cases), timedOut.Load(), skipped.Load())
	if len(cases) == 0 {
		return 0, 0
	}
	return float64(m) / float64(len(cases)), m
}

func secret(behaviour float64) {
	if behaviour != 1.0 {
		fmt.Println("No secret for you. Yet.")
		return
	}
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		fmt.Println("secret: cannot read", secretPath+":", err)
		return
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("secret: bad base64:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "-f", "slant", string(text))
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	fmt.Println("*** SECRET UNLOCKED ***")
	if err != nil {
		fmt.Println("(figlet error:", err, ")")
	}
	os.Stdout.Write(out)
}

func writeReward(build, noPython, behaviour float64) {
	reward := (1*build + 1*noPython + 4*behaviour) / 6
	data, _ := json.Marshal(map[string]float64{
		"reward":    reward,
		"build":     build,
		"no_python": noPython,
		"behaviour": behaviour,
	})
	if err := os.MkdirAll(rewardDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "cannot create", rewardDir+":", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(rewardDir, "reward.json"), append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write reward.json:", err)
		os.Exit(1)
	}
	fmt.Println("reward.json:", string(data))
}

func tail(b []byte, n int) string {
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func main() {
	build := gradeBuild()

	behaviour, matched := 0.0, 0
	if build == 1 {
		cases, err := loadCases()
		if err != nil {
			fmt.Println("behaviour: cannot load cases:", err)
		} else if len(cases) == 0 {
			fmt.Println("behaviour: no cases in", casesPath)
		} else {
			behaviour, matched = gradeBehaviour(cases)
		}
	} else {
		fmt.Println("behaviour: skipped (build failed)")
	}

	noPython := 0.0
	py := findPython()
	switch {
	case build != 1:
		fmt.Println("no_python: 0 (build failed)")
	case len(py) > 0:
		fmt.Printf("no_python: 0 (%d .py files, e.g. %s)\n", len(py), py[0])
	case matched == 0:
		fmt.Println("no_python: 0 (no hidden case matches)")
	default:
		noPython = 1
		fmt.Println("no_python: 1")
	}

	writeReward(build, noPython, behaviour)
	secret(behaviour)
}
