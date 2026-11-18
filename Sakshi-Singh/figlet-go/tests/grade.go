// grade.go - verifier for the figlet-go Harbor task. Go standard library only.
//
//	build      (weight 1): `go build -o /tmp/figlet .` and `go build ./...` succeed in /workspace/repo
//	no_python  (weight 1): build passed AND no .py file under /workspace/repo AND >= 1 hidden case matches
//	behaviour  (weight 4): share of /tests/data/hidden.jsonl cases whose stdout equals "out" exactly
//
// Writes /logs/verifier/reward.json. A perfect behaviour score unlocks the secret.
package main

import (
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
	"time"
)

const (
	repoDir    = "/workspace/repo"
	binPath    = "/tmp/figlet"
	hiddenPath = "/tests/data/hidden.jsonl"
	secretPath = "/tests/secret.b64"
	rewardDir  = "/logs/verifier"
	caseTime   = 5 * time.Second
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func writeReward(build, noPython, behaviour float64) {
	reward := (1*build + 1*noPython + 4*behaviour) / 6
	data, _ := json.Marshal(map[string]float64{
		"reward":    reward,
		"build":     build,
		"no_python": noPython,
		"behaviour": behaviour,
	})
	_ = os.MkdirAll(rewardDir, 0o755)
	if err := os.WriteFile(filepath.Join(rewardDir, "reward.json"), data, 0o644); err != nil {
		fmt.Println("cannot write reward.json:", err)
		os.Exit(1)
	}
	fmt.Printf("reward.json: %s\n", data)
}

func goCmd(args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = repoDir
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if err != nil {
		out := buf.String()
		if len(out) > 2000 {
			out = out[:2000] + "..."
		}
		fmt.Printf("$ go %s\n%s\n", strings.Join(args, " "), out)
	}
	return err
}

func runFiglet(args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), caseTime)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = os.TempDir() // the program must work from any working directory
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.Bytes(), err
}

func hasPython() bool {
	found := false
	_ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			fmt.Println("python file found:", path)
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func loadCases() []testCase {
	raw, err := os.ReadFile(hiddenPath)
	if err != nil {
		fmt.Println("cannot read hidden cases:", err)
		writeReward(0, 0, 0)
		os.Exit(1)
	}
	var cases []testCase
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var c testCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			fmt.Println("bad case line:", err)
			writeReward(0, 0, 0)
			os.Exit(1)
		}
		cases = append(cases, c)
	}
	return cases
}

func main() {
	cases := loadCases()

	// 1. build (a stale binary from the agent never counts)
	_ = os.Remove(binPath)
	build := 0.0
	if goCmd("build", "-o", binPath, ".") == nil && goCmd("build", "./...") == nil {
		build = 1
	}
	fmt.Printf("build: %v\n", build)

	// 3. behaviour (only a binary we just built is run)
	passed := 0
	if build == 1 {
		ok := make([]bool, len(cases))
		jobs := make(chan int)
		var wg sync.WaitGroup
		for w := 0; w < runtime.NumCPU(); w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					got, _ := runFiglet(cases[i].Args)
					ok[i] = bytes.Equal(got, []byte(cases[i].Out))
				}
			}()
		}
		for i := range cases {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		shown := 0
		for i, good := range ok {
			if good {
				passed++
			} else if shown < 5 {
				shown++
				fmt.Printf("mismatch: %q\n", cases[i].Args)
			}
		}
	}
	behaviour := 0.0
	if len(cases) > 0 {
		behaviour = float64(passed) / float64(len(cases))
	}
	fmt.Printf("behaviour: %d/%d\n", passed, len(cases))

	// 2. no_python: tied to the build AND to at least one right answer
	noPython := 0.0
	if build == 1 && passed >= 1 && !hasPython() {
		noPython = 1
	}
	fmt.Printf("no_python: %v\n", noPython)

	writeReward(build, noPython, behaviour)

	// 5. the secret, drawn by the twin itself
	if behaviour == 1.0 {
		enc, err := os.ReadFile(secretPath)
		if err == nil {
			var text []byte
			text, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(enc)))
			if err == nil {
				var art []byte
				art, err = runFiglet([]string{"-f", "slant", string(text)})
				if err == nil {
					fmt.Println("*** SECRET UNLOCKED ***")
					fmt.Print(string(art))
				}
			}
		}
		if err != nil {
			fmt.Println("secret could not be drawn:", err)
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
