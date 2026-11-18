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

type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type Reward struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	build := runBuild()
	noPython := 0.0
	behaviour := 0.0

	if build == 1.0 {
		var passedCase bool
		behaviour, passedCase = runBehaviour()
		if !containsPythonFile("/workspace/repo") && passedCase {
			noPython = 1.0
		}
	}

	reward := (build*1.0 + noPython*1.0 + behaviour*4.0) / 6.0

	result := Reward{
		Reward:    reward,
		Build:     build,
		NoPython:  noPython,
		Behaviour: behaviour,
	}

	if err := os.MkdirAll("/logs/verifier", 0755); err != nil {
		fmt.Println("Could not create reward directory:", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Println("Could not encode reward:", err)
		os.Exit(1)
	}

	if err := os.WriteFile("/logs/verifier/reward.json", data, 0644); err != nil {
		fmt.Println("Could not write reward:", err)
		os.Exit(1)
	}

	fmt.Printf("\nReward: %.6f\n", reward)
	fmt.Printf("Build: %.2f | No Python: %.2f | Behaviour: %.6f\n",
		build, noPython, behaviour)

	if behaviour == 1.0 {
		unlockSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func runBuild() float64 {
	fmt.Println("Checking Go build...")

	cmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	cmd.Dir = "/workspace/repo"
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Println("Build failed:")
		fmt.Println(string(output))
		return 0.0
	}

	fmt.Println("Build passed.")
	return 1.0
}

func containsPythonFile(root string) bool {
	found := false

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".py") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})

	return found
}

func runBehaviour() (float64, bool) {
	file, err := os.Open("/tests/data/hidden.jsonl")
	if err != nil {
		fmt.Println("Could not open hidden test data:", err)
		return 0.0, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	total := 0
	passed := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var test TestCase
		if err := json.Unmarshal(line, &test); err != nil {
			fmt.Println("Invalid test case:", err)
			total++
			continue
		}

		total++

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "/tmp/figlet", test.Args...)
		output, runErr := cmd.Output()
		cancel()

		if runErr == nil && bytes.Equal(output, []byte(test.Out)) {
			passed++
		}

		if total%100 == 0 {
			fmt.Printf("Checked %d tests; passed %d\n", total, passed)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading hidden tests:", err)
		return 0.0, passed > 0
	}

	if total == 0 {
		fmt.Println("No hidden test cases found.")
		return 0.0, false
	}

	score := float64(passed) / float64(total)
	fmt.Printf("Behaviour: %d/%d passed (%.6f)\n", passed, total, score)
	return score, passed > 0
}

func unlockSecret() {
	encoded, err := os.ReadFile("/tests/secret.b64")
	if err != nil {
		fmt.Println("Could not read secret:", err)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		fmt.Println("Could not decode secret:", err)
		return
	}

	cmd := exec.Command("/tmp/figlet", "-f", "slant", string(decoded))
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Could not render secret:", err)
		return
	}

	fmt.Println("\n*** SECRET UNLOCKED ***")
	fmt.Print(string(output))
}
