// grade.go: verifier for the figlet-go Harbor task. Go standard library only.
//
//	build      (weight 1): `go build -o /tmp/figlet .` in /workspace/repo succeeds
//	no_python  (weight 1): build passed AND no *.py under /workspace/repo
//	                       AND at least one hidden case matches
//	behaviour  (weight 4): share of hidden cases whose stdout equals "out" exactly
//
// Writes /logs/verifier/reward.json. If behaviour is exactly 1.0 it draws the
// secret from /tests/secret.b64 with the agent's own binary.
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
	"time"
)

const (
	repoDir     = "/workspace/repo"
	binPath     = "/tmp/figlet"
	hiddenPath  = "/tests/data/hidden.jsonl"
	secretPath  = "/tests/secret.b64"
	rewardDir   = "/logs/verifier"
	rewardPath  = "/logs/verifier/reward.json"
	caseTimeout = 5 * time.Second

	wBuild     = 1.0
	wNoPython  = 1.0
	wBehaviour = 4.0
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

func writeReward(r reward) {
	_ = os.MkdirAll(rewardDir, 0o777)
	b, _ := json.Marshal(r)
	if err := os.WriteFile(rewardPath, append(b, '\n'), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write reward:", err)
		os.Exit(1)
	}
	fmt.Printf("reward.json: %s\n", b)
}

func build() bool {
	_ = os.Remove(binPath) // never trust a binary left behind by the agent
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("build: FAIL (%v)\n%s\n", err, out)
		return false
	}
	fmt.Println("build: ok")
	return true
}

func pythonFiles() []string {
	var found []string
	_ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(path, ".py") {
			found = append(found, path)
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

func run(args []string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = os.TempDir() // the program must work from any working directory
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run() // exit status is not graded, only stdout
	if ctx.Err() != nil {
		return nil, false
	}
	return stdout.Bytes(), true
}

func main() {
	var r reward
	// Always write a reward first, so a crash below still leaves a valid 0.
	writeReward(r)

	built := build()
	if built {
		r.Build = 1
	}

	passed, total := 0, 0
	if built {
		cases, err := loadCases(hiddenPath)
		if err != nil {
			fmt.Println("cannot load cases:", err)
		}
		total = len(cases)
		shown := 0
		for i, c := range cases {
			got, ok := run(c.Args)
			if ok && bytes.Equal(got, []byte(c.Out)) {
				passed++
			} else if shown < 3 {
				shown++
				fmt.Printf("case %d differs: args=%q\n", i, c.Args)
			}
		}
		if total > 0 {
			r.Behaviour = float64(passed) / float64(total)
		}
	}
	fmt.Printf("behaviour: %d/%d\n", passed, total)

	py := pythonFiles()
	if len(py) > 0 {
		fmt.Printf("no_python: %d .py files remain, e.g. %s\n", len(py), py[0])
	}
	if built && len(py) == 0 && passed > 0 {
		r.NoPython = 1
	}

	r.Reward = (wBuild*r.Build + wNoPython*r.NoPython + wBehaviour*r.Behaviour) /
		(wBuild + wNoPython + wBehaviour)
	writeReward(r)

	if r.Behaviour == 1.0 {
		drawSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func drawSecret() {
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
	art, _ := run([]string{"-f", "slant", string(msg)})
	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(string(art))
}
