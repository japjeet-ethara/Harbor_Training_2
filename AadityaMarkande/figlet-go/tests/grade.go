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

const (
	repoDir    = "/workspace/repo"
	binPath    = "/tmp/figlet"
	hiddenPath = "/tests/data/hidden.jsonl"
	secretPath = "/tests/secret.b64"
	rewardDir  = "/logs/verifier"
)

func loadCases(path string) ([]testCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cases []testCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
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

func build() float64 {
	os.Remove(binPath)
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = repoDir
	if os.Getenv("GOCACHE") == "" {
		cmd.Env = append(os.Environ(), "GOCACHE=/tmp/gocache-grader")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("build failed:", err)
		fmt.Println(string(out))
		return 0
	}
	if _, err := os.Stat(binPath); err != nil {
		return 0
	}
	return 1
}

func hasPython() bool {
	found := false
	filepath.Walk(repoDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".py") {
			found = true
		}
		return nil
	})
	return found
}

func runBinary(args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = "/tmp"
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	return stdout.Bytes(), err
}

func matches(c testCase) bool {
	out, _ := runBinary(c.Args)
	return string(out) == c.Out
}

func main() {
	cases, err := loadCases(hiddenPath)
	if err != nil || len(cases) == 0 {
		fmt.Println("cannot load hidden cases:", err)
		os.Exit(1)
	}

	r := reward{}
	r.Build = build()

	if r.Build == 1 {
		results := make([]bool, len(cases))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for i := range cases {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				results[i] = matches(cases[i])
			}(i)
		}
		wg.Wait()
		pass := 0
		for _, ok := range results {
			if ok {
				pass++
			}
		}
		r.Behaviour = float64(pass) / float64(len(cases))
		if !hasPython() && pass >= 1 {
			r.NoPython = 1
		}
	}

	r.Reward = (1*r.Build + 1*r.NoPython + 4*r.Behaviour) / 6

	os.MkdirAll(rewardDir, 0o755)
	data, _ := json.Marshal(r)
	os.WriteFile(filepath.Join(rewardDir, "reward.json"), data, 0o644)

	fmt.Printf("build=%.4f no_python=%.4f behaviour=%.4f reward=%.4f\n",
		r.Build, r.NoPython, r.Behaviour, r.Reward)

	if r.Behaviour == 1.0 {
		raw, err := os.ReadFile(secretPath)
		if err == nil {
			dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
			if err == nil {
				out, _ := runBinary([]string{"-f", "slant", string(dec)})
				fmt.Println("*** SECRET UNLOCKED ***")
				fmt.Print(string(out))
				return
			}
		}
		fmt.Println("secret file could not be read")
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
