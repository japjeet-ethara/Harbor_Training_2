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
	repoDir    = "/workspace/repo"
	binPath    = "/tmp/figlet"
	hiddenPath = "/tests/data/hidden.jsonl"
	secretPath = "/tests/secret.b64"
	rewardPath = "/logs/verifier/reward.json"
	caseLimit  = 5 * time.Second
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

func runFiglet(args []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), caseLimit)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = "/tmp" // must work from any working directory
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()
	return out.String()
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
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		var c testCase
		if json.Unmarshal(sc.Bytes(), &c) == nil {
			cases = append(cases, c)
		}
	}
	return cases
}

func hasPython() bool {
	found := false
	_ = filepath.WalkDir(repoDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".py") {
			found = true
		}
		return nil
	})
	return found
}

func main() {
	_ = os.Remove(binPath)

	// 1. build
	build := 0.0
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println("build failed:", err)
		fmt.Println(string(out))
	} else {
		build = 1
	}

	// 3. behaviour (hidden cases only)
	cases := loadCases()
	matched := 0
	if build == 1 {
		for _, c := range cases {
			if runFiglet(c.Args) == c.Out {
				matched++
			}
		}
	}
	behaviour := 0.0
	if len(cases) > 0 {
		behaviour = float64(matched) / float64(len(cases))
	}

	// 2. no_python: needs a passing build, no .py files, and one right answer
	noPython := 0.0
	if build == 1 && !hasPython() && matched > 0 {
		noPython = 1
	}

	total := (build*1 + noPython*1 + behaviour*4) / 6
	r := reward{Reward: total, Build: build, NoPython: noPython, Behaviour: behaviour}

	_ = os.MkdirAll(filepath.Dir(rewardPath), 0o755)
	data, _ := json.Marshal(r)
	if err := os.WriteFile(rewardPath, data, 0o644); err != nil {
		fmt.Println("cannot write reward:", err)
		os.Exit(1)
	}
	fmt.Printf("build=%.2f no_python=%.2f behaviour=%.4f (%d/%d) reward=%.4f\n",
		build, noPython, behaviour, matched, len(cases), total)

	// 5. secret
	if behaviour == 1.0 {
		raw, err := os.ReadFile(secretPath)
		if err == nil {
			text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
			if err == nil {
				fmt.Println("*** SECRET UNLOCKED ***")
				fmt.Print(runFiglet([]string{"-f", "slant", string(text)}))
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}