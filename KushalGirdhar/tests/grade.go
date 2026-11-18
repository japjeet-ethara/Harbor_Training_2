package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	repo      = "/workspace/repo"
	binary    = "/tmp/figlet"
	hidden    = "/tests/data/hidden.jsonl"
	secret    = "/tests/secret.b64"
	rewardDir = "/logs/verifier"
)

type testCase struct {
	Args []string `json:"args"`
	Out  *string  `json:"out"`
}

type score struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	log.SetFlags(0)
	if err := grade(); err != nil {
		log.Fatal(err)
	}
}

func grade() error {
	data, err := os.Open(hidden)
	if err != nil {
		return err
	}
	defer data.Close()

	var result score
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = repo
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	buildErr := build.Run()
	if buildErr == nil {
		result.Build = 1
	} else {
		log.Printf("build failed: %v", buildErr)
	}
	total, matches, timeouts, failures, err := evaluate(data, result.Build == 1, binary, 5*time.Second)
	if err != nil {
		return err
	}
	result.Behaviour = float64(matches) / float64(total)
	if result.Build == 1 && matches > 0 {
		clean, err := noPython(repo)
		if err != nil {
			return err
		}
		if clean {
			result.NoPython = 1
		}
	}
	result.Reward = weighted(result)
	if err := saveReward(result); err != nil {
		return err
	}
	fmt.Printf("build=%.0f no_python=%.0f exact=%d/%d timeouts=%d process_errors=%d reward=%.8g\n",
		result.Build, result.NoPython, matches, total, timeouts, failures, result.Reward)
	if result.Behaviour == 1 {
		return unlock()
	}
	fmt.Println("No secret for you. Yet.")
	return nil
}

func evaluate(data io.Reader, built bool, program string, timeout time.Duration) (total, matches, timeouts, failures int, err error) {
	decoder := json.NewDecoder(data)
	for {
		var test testCase
		err = decoder.Decode(&test)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return total, matches, timeouts, failures, fmt.Errorf("decode case %d: %w", total+1, err)
		}
		if test.Args == nil || test.Out == nil {
			return total, matches, timeouts, failures, fmt.Errorf("case %d lacks args or out", total+1)
		}
		total++
		if !built {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		command := exec.CommandContext(ctx, program, test.Args...)
		command.WaitDelay = time.Second
		stdout, runErr := command.Output()
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		cancel()
		switch {
		case timedOut:
			timeouts++
		case runErr != nil:
			failures++
		case bytes.Equal(stdout, []byte(*test.Out)):
			matches++
		}
	}
	if total == 0 {
		return total, matches, timeouts, failures, errors.New("empty hidden corpus")
	}
	return total, matches, timeouts, failures, nil
}

func noPython(root string) (bool, error) {
	found := false
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".py") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return !found && err == nil, err
}

func weighted(result score) float64 {
	return (result.Build + result.NoPython + 4*result.Behaviour) / 6
}

func saveReward(result score) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(rewardDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rewardDir, "reward.json"), append(data, '\n'), 0644)
}

func unlock() error {
	encoded, err := os.ReadFile(secret)
	if err != nil {
		return err
	}
	message, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "-f", "slant", string(message))
	command.WaitDelay = time.Second
	art, err := command.Output()
	if err != nil {
		return err
	}
	fmt.Println("*** SECRET UNLOCKED ***")
	_, err = os.Stdout.Write(art)
	return err
}
