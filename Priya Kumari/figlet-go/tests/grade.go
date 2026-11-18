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

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// TestCase mirrors one line of the JSONL files.
type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

// Reward is the structure written to reward.json.
type Reward struct {
	RewardVal float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------

const (
	repoDir    = "/workspace/repo"
	figletBin  = "/tmp/figlet"
	hiddenPath = "/tests/data/hidden.jsonl"
	rewardDir  = "/logs/verifier"
	secretPath = "/tests/secret.b64"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadCases reads a JSONL file and returns the parsed test cases.
func loadCases(path string) ([]TestCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cases []TestCase
	scanner := bufio.NewScanner(f)
	// Default token size (64 KB) may be too small for large outputs.
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		var tc TestCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			return nil, fmt.Errorf("bad JSON line: %w", err)
		}
		cases = append(cases, tc)
	}
	return cases, scanner.Err()
}

// runCase executes /tmp/figlet with the given args, enforces a 5-second
// timeout, and returns true only if stdout matches the expected output
// byte-for-byte.
func runCase(tc TestCase) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, figletBin, tc.Args...)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return bytes.Equal(out, []byte(tc.Out))
}

// hasPythonFiles walks the directory tree and returns true if any file
// has a .py extension.
func hasPythonFiles(dir string) bool {
	found := false
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".py") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	var (
		buildScore     float64
		noPythonScore  float64
		behaviourScore float64
	)

	// ---- 1. BUILD (weight 1) ----
	fmt.Println("=== BUILD ===")
	buildCmd := exec.Command("go", "build", "-o", figletBin, ".")
	buildCmd.Dir = repoDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("BUILD: FAIL (%v)\n", err)
	} else {
		fmt.Println("BUILD: PASS")
		buildScore = 1
	}

	// ---- 3. BEHAVIOUR (weight 4) — run before NO_PYTHON so we know
	//         whether at least one case matched. ----
	fmt.Println("\n=== BEHAVIOUR ===")
	var totalCases, passedCases int

	if buildScore == 1 {
		cases, err := loadCases(hiddenPath)
		if err != nil {
			fmt.Printf("  Failed to load hidden cases: %v\n", err)
		} else {
			totalCases = len(cases)
			for i, tc := range cases {
				if runCase(tc) {
					passedCases++
				}
				if (i+1)%200 == 0 || i+1 == totalCases {
					fmt.Printf("  Progress: %d/%d (%d passed)\n",
						i+1, totalCases, passedCases)
				}
			}
			if totalCases > 0 {
				behaviourScore = float64(passedCases) / float64(totalCases)
			}
		}
	}
	fmt.Printf("BEHAVIOUR: %d/%d (%.4f)\n", passedCases, totalCases, behaviourScore)

	// ---- 2. NO_PYTHON (weight 1) ----
	fmt.Println("\n=== NO_PYTHON ===")
	if buildScore == 0 {
		fmt.Println("NO_PYTHON: FAIL (build did not pass)")
	} else if hasPythonFiles(repoDir) {
		fmt.Println("NO_PYTHON: FAIL (.py files found under repo)")
	} else if passedCases == 0 {
		fmt.Println("NO_PYTHON: FAIL (no hidden test case matched)")
	} else {
		noPythonScore = 1
		fmt.Println("NO_PYTHON: PASS")
	}

	// ---- 4. Reward ----
	const (
		wBuild     = 1.0
		wNoPython  = 1.0
		wBehaviour = 4.0
	)
	totalWeight := wBuild + wNoPython + wBehaviour
	reward := (wBuild*buildScore + wNoPython*noPythonScore + wBehaviour*behaviourScore) / totalWeight

	result := Reward{
		RewardVal: reward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}

	if err := os.MkdirAll(rewardDir, 0o755); err != nil {
		fmt.Printf("  Warning: could not create %s: %v\n", rewardDir, err)
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	rewardPath := filepath.Join(rewardDir, "reward.json")
	if err := os.WriteFile(rewardPath, data, 0o644); err != nil {
		fmt.Printf("  Warning: could not write %s: %v\n", rewardPath, err)
	}
	fmt.Printf("\n=== REWARD ===\n%s\n", string(data))

	// ---- 5. SECRET ----
	fmt.Println("\n=== SECRET ===")
	if behaviourScore == 1.0 {
		secretB64, err := os.ReadFile(secretPath)
		if err != nil {
			fmt.Printf("Could not read %s: %v\n", secretPath, err)
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(
			strings.TrimSpace(string(secretB64)))
		if err != nil {
			fmt.Printf("Could not decode secret: %v\n", err)
			return
		}
		cmd := exec.Command(figletBin, "-f", "slant", string(decoded))
		out, err := cmd.Output()
		if err != nil {
			fmt.Printf("Could not render secret: %v\n", err)
			return
		}
		fmt.Println("*** SECRET UNLOCKED ***")
		fmt.Print(string(out))
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
