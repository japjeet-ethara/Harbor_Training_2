
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
	"strconv"
	"time"
)

type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type Result struct {
	Build    int     `json:"build"`
	NoPython int     `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
	Reward   float64 `json:"reward"`
}

func runCase(binary string, tc TestCase) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, tc.Args...)
	cmd.Dir = "/tmp"

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	return err == nil && bytes.Equal(stdout.Bytes(), []byte(tc.Out))
}

func checkPython(repo string) bool {
    found := false

    err := filepath.WalkDir(repo, func(path string, entry os.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if !entry.IsDir() && filepath.Ext(path) == ".py" {
            found = true
        }
        return nil
    })

    return err == nil && !found
}

func gradeHidden(binary, path string) (int, int) {
    file, err := os.Open(path)
    if err != nil {
        fmt.Println("Cannot open hidden tests:", err)
        return 0, 0
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)

    passed, total := 0, 0

    for scanner.Scan() {
        var tc TestCase
        if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
            total++
            continue
        }

        total++
        if runCase(binary, tc) {
            passed++
        }
    }

    if err := scanner.Err(); err != nil {
        fmt.Println("Test file read error:", err)
    }

    return passed, total
}

func unlockSecret(repo string) {
    data, err := os.ReadFile("/tests/secret.b64")
    if err != nil {
        fmt.Println("Cannot read secret:", err)
        return
    }

    secret, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(data)))
    if err != nil {
        fmt.Println("Cannot decode secret:", err)
        return
    }

    fmt.Println("*** SECRET UNLOCKED ***")
    cmd := exec.Command("/tmp/figlet", "-f", "slant", string(secret))
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    _ = cmd.Run()
}

func main() {
	repo := "/workspace/repo"

	result := Result{}

	fmt.Println("Building Go solution...")
	build := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	build.Dir = repo
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr

	if err := build.Run(); err != nil {
		fmt.Println("Build failed.")
		writeReward(repo, result)
		return
	}

	result.Build = 1
	fmt.Println("Build: PASS")

	fmt.Println("Checking hidden test cases...")
	passed, total := gradeHidden("/tmp/figlet", "/tests/data/hidden.jsonl")

	if total > 0 {
		result.Behaviour = float64(passed) / float64(total)
	}

	fmt.Printf("Behaviour: %d/%d (%.4f)\n", passed, total, result.Behaviour)

	if checkPython(repo) && passed > 0 {
		result.NoPython = 1
		fmt.Println("No Python files: PASS")
	} else {
		fmt.Println("No Python check: FAIL")
	}

	result.Reward = (float64(result.Build) +
		float64(result.NoPython) +
		4*result.Behaviour) / 6

	fmt.Printf("Final reward: %s\n", strconv.FormatFloat(result.Reward, 'f', 6, 64))

	if result.Behaviour == 1.0 && total > 0 {
		unlockSecret(repo)
	} else {
		fmt.Println("No secret for you. Yet.")
	}

	writeReward(repo, result)
}

func writeReward(repo string, result Result) {
	path := "/logs/verifier/reward.json"

	data, err := json.MarshalIndent(result, "", "  ")
        if err != nil {
                fmt.Println("Could not encode reward:", err)
                return
        }

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Println("Could not create reward directory:", err)
		return
	}

	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		fmt.Println("Could not write reward:", err)
		return
	}

	fmt.Println("Reward saved to", path)
}
