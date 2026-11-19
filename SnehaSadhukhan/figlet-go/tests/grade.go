// grade.go — verifier for the figlet-go task (Go standard library only).
//
// Scores, each 0..1:
//
//	build      1 if `go build -o /tmp/figlet .` succeeds in /workspace/repo
//	no_python  1 if build succeeded, no *.py file exists under /workspace/repo,
//	           and at least one hidden case matches
//	behaviour  fraction of /tests/data/hidden.jsonl cases whose stdout matches
//	           the stored "out" byte for byte (5 s timeout per case)
//
// reward = (build + no_python + 4*behaviour) / 6, written with the three scores
// to /logs/verifier/reward.json.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	caseTimeout  = 5 * time.Second
	buildTimeout = 5 * time.Minute
	// Safety net so the whole verifier finishes inside task.toml's verifier
	// timeout (900 s) even if every case hangs: hidden cases not started within
	// this time of the grader starting count as failures.
	overallBudget = 13 * time.Minute
)

var startTime = time.Now()

var (
	repoDir   = flag.String("repo", "/workspace/repo", "repository to build")
	binPath   = flag.String("bin", "/tmp/figlet", "path of the built program")
	testsDir  = flag.String("tests", "/tests", "directory containing data/hidden.jsonl and secret.b64")
	rewardOut = flag.String("reward", "/logs/verifier/reward.json", "reward file to write")
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type reward struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	flag.Parse()

	cases, loadErr := loadCases(filepath.Join(*testsDir, "data", "hidden.jsonl"))
	if loadErr != nil {
		fmt.Println("grader error:", loadErr)
	}

	// ---- build -----------------------------------------------------------
	build := 0.0
	_ = os.Remove(*binPath) // never grade a binary left over from before
	if err := runBuild(); err != nil {
		fmt.Println("build: FAIL:", err)
	} else if st, err := os.Stat(*binPath); err != nil || st.IsDir() {
		fmt.Println("build: FAIL: build succeeded but", *binPath, "was not produced")
	} else {
		build = 1
		fmt.Println("build: OK")
	}

	// ---- behaviour (hidden cases) ----------------------------------------
	matches := 0
	if build == 1 && len(cases) > 0 {
		matches = runCases(cases)
	}
	behaviour := 0.0
	if len(cases) > 0 {
		behaviour = float64(matches) / float64(len(cases))
	}
	fmt.Printf("behaviour: %d/%d hidden cases match (%.4f)\n", matches, len(cases), behaviour)

	// ---- no_python ---------------------------------------------------------
	pyFiles, walkErr := findPyFiles(*repoDir)
	if walkErr != nil {
		fmt.Println("no_python: could not scan repository:", walkErr)
	}
	noPython := 0.0
	switch {
	case build != 1:
		fmt.Println("no_python: 0 (build failed)")
	case walkErr != nil:
		fmt.Println("no_python: 0 (scan failed)")
	case len(pyFiles) > 0:
		fmt.Printf("no_python: 0 (%d .py file(s) found, e.g. %s)\n", len(pyFiles), pyFiles[0])
	case matches < 1:
		fmt.Println("no_python: 0 (no hidden case matches)")
	default:
		noPython = 1
		fmt.Println("no_python: 1")
	}

	// ---- reward ------------------------------------------------------------
	r := reward{
		Reward:    (build + noPython + 4*behaviour) / 6,
		Build:     build,
		NoPython:  noPython,
		Behaviour: behaviour,
	}
	if err := writeReward(r); err != nil {
		fmt.Println("grader error: writing reward:", err)
		os.Exit(1)
	}
	fmt.Printf("reward: %.6f  (build=%g no_python=%g behaviour=%.6f)\n", r.Reward, r.Build, r.NoPython, r.Behaviour)

	// ---- secret ------------------------------------------------------------
	if len(cases) > 0 && matches == len(cases) {
		unlockSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}

	if loadErr != nil {
		os.Exit(1)
	}
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
	for line := 1; sc.Scan(); line++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var c testCase
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			return nil, fmt.Errorf("%s:%d: %v", path, line, err)
		}
		cases = append(cases, c)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("%s: no cases", path)
	}
	return cases, nil
}

func runBuild() error {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", *binPath, ".")
	cmd.Dir = *repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	setProcessGroup(cmd)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("timed out after %s", buildTimeout)
		}
		return err
	}
	return nil
}

// runCases runs every case in parallel workers and returns the number of exact matches.
func runCases(cases []testCase) int {
	deadline := startTime.Add(overallBudget)
	workers := runtime.NumCPU()
	if workers < 4 {
		workers = 4
	}
	jobs := make(chan int)
	results := make([]bool, len(cases))
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if time.Now().After(deadline) {
					continue // budget exhausted: counts as a failure
				}
				out, err := runFiglet(cases[i].Args, len(cases[i].Out))
				results[i] = err == nil && bytes.Equal(out, []byte(cases[i].Out))
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	matches, shown := 0, 0
	for i, ok := range results {
		if ok {
			matches++
		} else if shown < 5 {
			fmt.Printf("  mismatch: hidden case #%d\n", i+1)
			shown++
		}
	}
	return matches
}

// runFiglet runs the built program with a 5 s timeout from a neutral working
// directory and returns its stdout. Output beyond expectedLen+1 bytes is
// discarded (it cannot match anyway), which bounds memory use.
func runFiglet(args []string, expectedLen int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, *binPath, args...)
	cmd.Dir = "/"
	var buf limitedBuffer
	buf.limit = expectedLen + 1
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard
	setProcessGroup(cmd)
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, errors.New("timeout")
	}
	// A non-zero exit status is not itself a failure: only stdout is compared.
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return nil, err
	}
	return buf.Bytes(), nil
}

// setProcessGroup puts the child in its own process group and makes
// cancellation kill the whole group, so no grandchild survives a timeout.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.Len(); room > 0 {
		if len(p) <= room {
			b.Buffer.Write(p)
		} else {
			b.Buffer.Write(p[:room])
		}
	}
	return len(p), nil
}

func findPyFiles(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".py") {
			found = append(found, path)
		}
		return nil
	})
	return found, err
}

func writeReward(r reward) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*rewardOut), 0o755); err != nil {
		return err
	}
	return os.WriteFile(*rewardOut, append(data, '\n'), 0o644)
}

func unlockSecret() {
	raw, err := os.ReadFile(filepath.Join(*testsDir, "secret.b64"))
	if err != nil {
		fmt.Println("grader error: reading secret:", err)
		return
	}
	plain, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("grader error: decoding secret:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, *binPath, "-f", "slant", string(plain))
	cmd.Dir = "/"
	setProcessGroup(cmd)
	out, _ := cmd.Output()
	fmt.Println("*** SECRET UNLOCKED ***")
	os.Stdout.Write(out)
}
