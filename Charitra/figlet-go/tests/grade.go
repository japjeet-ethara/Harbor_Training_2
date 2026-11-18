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

type Case struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func run(timeout time.Duration, dir, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}
	err := cmd.Run()
	if ctx.Err() != nil {
		return out.Bytes(), ctx.Err()
	}
	return out.Bytes(), err
}
func hasPython(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
func main() {
	const repo = "/workspace/repo"
	build := 0.0
	if _, err := run(5*time.Minute, repo, "go", "build", "-o", "/tmp/figlet", "."); err == nil {
		build = 1
	}
	behaviour := 0.0
	matched := 0
	file, err := os.Open("/tests/data/hidden.jsonl")
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
		total := 0
		for scanner.Scan() {
			var c Case
			if json.Unmarshal(scanner.Bytes(), &c) != nil {
				total++
				continue
			}
			total++
			out, e := run(5*time.Second, repo, "/tmp/figlet", c.Args...)
			if e == nil && bytes.Equal(out, []byte(c.Out)) {
				matched++
			}
		}
		if total > 0 {
			behaviour = float64(matched) / float64(total)
		}
	}
	noPython := 0.0
	if build == 1 && !hasPython(repo) && matched > 0 {
		noPython = 1
	}
	reward := (build + noPython + 4*behaviour) / 6
	payload := map[string]float64{"reward": reward, "build": build, "no_python": noPython, "behaviour": behaviour}
	b, _ := json.Marshal(payload)
	_ = os.MkdirAll("/logs/verifier", 0755)
	_ = os.WriteFile("/logs/verifier/reward.json", append(b, '\n'), 0644)
	fmt.Printf("reward=%0.6f build=%0.1f no_python=%0.1f behaviour=%0.6f (%d hidden cases matched)\n", reward, build, noPython, behaviour, matched)
	if behaviour == 1.0 {
		secret, e := os.ReadFile("/tests/secret.b64")
		if e == nil {
			decoded, de := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secret)))
			if de == nil {
				fmt.Println("*** SECRET UNLOCKED ***")
				out, _ := run(5*time.Second, repo, "/tmp/figlet", "-f", "slant", string(decoded))
				fmt.Print(string(out))
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
	if err != nil {
		fmt.Println("hidden cases unavailable:", err)
	}
}
