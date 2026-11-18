// grade.go - the verifier for the figlet-go Harbor task. Go standard library only; no network.
//
//	build      (weight 1)  go build -o /tmp/figlet .  AND  go build ./...  both succeed in /workspace/repo
//	no_python  (weight 1)  build passed AND no *.py under /workspace/repo AND >= 1 hidden case matches
//	behaviour  (weight 4)  share of hidden cases whose stdout equals "out" byte for byte
//
// reward = (1*build + 1*no_python + 4*behaviour) / 6, written to /logs/verifier/reward.json.
// A perfect twin (behaviour == 1.0) also gets the secret drawn in giant letters in this log.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	repoDir     = "/workspace/repo"
	binPath     = "/tmp/figlet"
	hiddenPath  = "/tests/data/hidden.jsonl" // the HIDDEN cases (never visible.jsonl)
	secretPath  = "/tests/secret.b64"
	rewardPath  = "/logs/verifier/reward.json"
	caseTimeout = 5 * time.Second
	totalBudget = 10 * time.Minute // verifier timeout is 15 min; stop early and count the rest as wrong
	maxOutput   = 1 << 20
	workers     = 4
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

// capWriter keeps at most max bytes but always reports success so the child never blocks.
type capWriter struct {
	buf bytes.Buffer
	max int
}

func (w *capWriter) Write(p []byte) (int, error) {
	if room := w.max - w.buf.Len(); room > 0 {
		if len(p) > room {
			w.buf.Write(p[:room])
		} else {
			w.buf.Write(p)
		}
	}
	return len(p), nil
}

func runIn(dir string, timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.WaitDelay = time.Second
	out := &capWriter{max: maxOutput}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return out.buf.Bytes(), err
}

func goBuild(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = repoDir
	cmd.WaitDelay = 2 * time.Second
	var msg capWriter
	msg.max = 4000
	cmd.Stdout, cmd.Stderr = &msg, &msg
	if err := cmd.Run(); err != nil {
		fmt.Printf("  go %s failed: %v\n%s\n", strings.Join(args, " "), err, msg.buf.String())
		return err
	}
	return nil
}

func hasPython() []string {
	var found []string
	_ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func loadCases() ([]testCase, error) {
	f, err := os.Open(hiddenPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cases []testCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
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

func writeReward(reward, build, noPython, behaviour float64) {
	_ = os.MkdirAll(filepath.Dir(rewardPath), 0o755)
	data, _ := json.Marshal(map[string]float64{
		"reward": reward, "build": build, "no_python": noPython, "behaviour": behaviour,
	})
	if err := os.WriteFile(rewardPath, append(data, '\n'), 0o644); err != nil {
		fmt.Println("cannot write reward:", err)
		os.Exit(1)
	}
}

func main() {
	cases, err := loadCases()
	if err != nil || len(cases) == 0 {
		fmt.Println("cannot load hidden cases:", err)
		os.Exit(1)
	}

	// ---- 1. build (weight 1) -------------------------------------------------------------
	_ = os.Remove(binPath) // never trust a leftover binary
	build := 0.0
	fmt.Println("[build] go build -o /tmp/figlet .  &&  go build ./...")
	// -buildvcs=false: the verifier runs as another user than the repo owner; a stray .git must not break it.
	if goBuild("build", "-buildvcs=false", "-o", binPath, ".") == nil && goBuild("build", "-buildvcs=false", "./...") == nil {
		if st, err := os.Stat(binPath); err == nil && !st.IsDir() {
			build = 1
		}
	}
	fmt.Printf("[build] score %.0f\n", build)

	// ---- 3. behaviour (weight 4) ---------------------------------------------------------
	behaviour := 0.0
	passed := int64(0)
	var failedArgs [][]string
	if build == 1 {
		workDir, _ := os.MkdirTemp("", "figlet-cwd-") // empty directory: the program must not need the repo as cwd
		defer os.RemoveAll(workDir)
		deadline := time.Now().Add(totalBudget)
		var mu sync.Mutex
		jobs := make(chan testCase)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for c := range jobs {
					if time.Now().After(deadline) {
						continue
					}
					got, _ := runIn(workDir, caseTimeout, binPath, c.Args...)
					if bytes.Equal(got, []byte(c.Out)) {
						atomic.AddInt64(&passed, 1)
					} else {
						mu.Lock()
						if len(failedArgs) < 3 {
							failedArgs = append(failedArgs, c.Args)
						}
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
		behaviour = float64(passed) / float64(len(cases))
	}
	fmt.Printf("[behaviour] %d of %d hidden cases match -> %.4f\n", passed, len(cases), behaviour)
	for _, a := range failedArgs {
		fmt.Printf("  example failing args: %q\n", a)
	}

	// ---- 2. no_python (weight 1): tied to the build AND to at least one right answer -------
	noPython := 0.0
	if build == 1 && passed >= 1 {
		if py := hasPython(); len(py) == 0 {
			noPython = 1
		} else {
			fmt.Printf("[no_python] found %d .py file(s), e.g. %s\n", len(py), py[0])
		}
	}
	fmt.Printf("[no_python] score %.0f\n", noPython)

	// ---- 4. reward.json ------------------------------------------------------------------
	reward := (1*build + 1*noPython + 4*behaviour) / 6
	writeReward(reward, build, noPython, behaviour)
	fmt.Printf("[reward] %.4f  (build %.0f, no_python %.0f, behaviour %.4f)\n", reward, build, noPython, behaviour)

	// ---- 5. the secret: only for a perfect twin -------------------------------------------
	if behaviour == 1.0 {
		raw, err := os.ReadFile(secretPath)
		if err == nil {
			msg, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
			if err == nil {
				art, _ := runIn(os.TempDir(), caseTimeout, binPath, "-f", "slant", string(msg))
				fmt.Println("*** SECRET UNLOCKED ***")
				fmt.Print(string(art))
				return
			}
		}
		fmt.Println("*** SECRET UNLOCKED *** (but the secret file could not be read)")
		return
	}
	fmt.Println("No secret for you. Yet.")
}
