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
	"time"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	repo := "/workspace/repo"
	build := buildProgram(repo)
	behavior := 0.0
	matchedHidden := false
	if build == 1 {
		behavior, matchedHidden = gradeBehavior(repo)
	}

	noPython := 0.0
	if build == 1 && noPythonFiles(repo) && matchedHidden {
		noPython = 1
	}

	reward := (build + noPython + 4*behavior) / 6
	result := map[string]float64{
		"reward":    reward,
		"build":     build,
		"no_python": noPython,
		"behaviour": behavior,
	}
	if err := os.MkdirAll("/logs/verifier", 0755); err != nil {
		fatal(err)
	}
	file, err := os.Create("/logs/verifier/reward.json")
	if err != nil {
		fatal(err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fatal(err)
	}
	if err := file.Close(); err != nil {
		fatal(err)
	}
	fmt.Printf("reward: %.6f (build %.1f, no_python %.1f, behaviour %.6f)\n",
		reward, build, noPython, behavior)

	if behavior == 1 {
		secret, err := os.ReadFile("/tests/secret.b64")
		if err != nil {
			fatal(err)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secret)))
		if err != nil {
			fatal(err)
		}
		cmd := exec.Command("/tmp/figlet", "-f", "slant", string(decoded))
		cmd.Dir = repo
		out, err := cmd.Output()
		if err != nil {
			fatal(err)
		}
		fmt.Println("*** SECRET UNLOCKED ***")
		_, _ = os.Stdout.Write(out)
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func buildProgram(repo string) float64 {
	cmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("build failed: %v\n%s\n", err, output)
		return 0
	}
	return 1
}

func gradeBehavior(repo string) (float64, bool) {
	file, err := os.Open("/tests/data/hidden.jsonl")
	if err != nil {
		fmt.Printf("cannot read hidden cases: %v\n", err)
		return 0, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	total, passed := 0, 0
	matched := false
	for scanner.Scan() {
		var tc testCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			fmt.Printf("invalid hidden case %d: %v\n", total+1, err)
			return 0, matched
		}
		total++
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
		cmd.Dir = repo
		actual, runErr := cmd.Output()
		cancel()
		if runErr != nil {
			continue
		}
		if bytes.Equal(actual, []byte(tc.Out)) {
			passed++
			matched = true
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("cannot scan hidden cases: %v\n", err)
		return 0, matched
	}
	if total == 0 {
		fmt.Println("hidden case file is empty")
		return 0, false
	}
	return float64(passed) / float64(total), matched
}

func noPythonFiles(root string) bool {
	found := true
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".py") {
			found = false
			return filepath.SkipAll
		}
		return nil
	})
	return err == nil && found
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
