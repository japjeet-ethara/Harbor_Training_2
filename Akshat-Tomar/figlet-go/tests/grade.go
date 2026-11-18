// tests/grade.go — Harbor verifier for figlet-go task
// Run with: cd /tests && go run grade.go
// Uses only the Go standard library.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	repoDir    = "/workspace/repo"
	figletBin  = "/tmp/figlet"
	hiddenFile = "/tests/data/hidden.jsonl"
	secretFile = "/tests/secret.b64"
	rewardFile = "/logs/verifier/reward.json"
	caseTimeout = 5 * time.Second
)

type Case struct {
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
	build := scoreBuild()
	noPy := scoreNoPython(build)
	beh := scoreBehaviour(build)
	// Weighted mean: build×1, no_python×1, behaviour×4  → total weight 6
	reward := (build*1 + noPy*1 + beh*4) / 6.0

	r := Reward{
		Reward:   reward,
		Build:    build,
		NoPython: noPy,
		Behaviour: beh,
	}

	writeReward(r)
	printSummary(r)
	handleSecret(beh)
}

// ── build ────────────────────────────────────────────────────────────────────

func scoreBuild() float64 {
	fmt.Println("=== [1/3] Build check ===")
	cmd := exec.Command("go", "build", "-o", figletBin, ".")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("FAIL: go build error:\n%s\n", out)
		return 0
	}
	fmt.Println("PASS: binary built at", figletBin)
	return 1
}

// ── no_python ────────────────────────────────────────────────────────────────

func scoreNoPython(build float64) float64 {
	fmt.Println("\n=== [2/3] No-Python check ===")
	if build == 0 {
		fmt.Println("SKIP: build failed")
		return 0
	}

	// Check for .py files
	var pyFiles []string
	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".py") {
			pyFiles = append(pyFiles, path)
		}
		return nil
	})
	if len(pyFiles) > 0 {
		fmt.Printf("FAIL: found %d .py file(s): %v\n", len(pyFiles), pyFiles)
		return 0
	}
	fmt.Println("PASS: no .py files found")

	// Also require at least one correct hidden case (prevents no_python rewarding
	// an agent that only deletes .py files without implementing anything)
	firstCase := firstHiddenCase()
	if firstCase == nil {
		fmt.Println("SKIP no_python bonus: could not read hidden cases")
		return 0
	}
	got := runFiglet(firstCase.Args, caseTimeout)
	if got == nil || *got != firstCase.Out {
		fmt.Println("FAIL: binary does not pass even the first hidden case")
		return 0
	}
	fmt.Println("PASS: binary passes at least one hidden case")
	return 1
}

func firstHiddenCase() *Case {
	f, err := os.Open(hiddenFile)
	if err != nil {
		return nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var c Case
		if json.Unmarshal([]byte(line), &c) == nil {
			return &c
		}
	}
	return nil
}

// ── behaviour ────────────────────────────────────────────────────────────────

func scoreBehaviour(build float64) float64 {
	fmt.Println("\n=== [3/3] Behaviour check ===")
	if build == 0 {
		fmt.Println("SKIP: build failed")
		return 0
	}

	f, err := os.Open(hiddenFile)
	if err != nil {
		fmt.Println("ERROR: cannot open hidden.jsonl:", err)
		return 0
	}
	defer f.Close()

	total, correct := 0, 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		total++
		got := runFiglet(c.Args, caseTimeout)
		if got != nil && *got == c.Out {
			correct++
		}
	}

	if total == 0 {
		fmt.Println("ERROR: no cases found in hidden.jsonl")
		return 0
	}
	score := float64(correct) / float64(total)
	fmt.Printf("Result: %d / %d correct (%.4f)\n", correct, total, score)
	return score
}

// ── helpers ──────────────────────────────────────────────────────────────────

func runFiglet(args []string, timeout time.Duration) *string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, figletBin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil
	}
	s := buf.String()
	return &s
}

func writeReward(r Reward) {
	_ = os.MkdirAll(filepath.Dir(rewardFile), 0755)
	data, _ := json.MarshalIndent(r, "", "  ")
	_ = os.WriteFile(rewardFile, data, 0644)
}

func printSummary(r Reward) {
	fmt.Printf("\n=== Reward Summary ===\n")
	fmt.Printf("  build:     %.4f\n", r.Build)
	fmt.Printf("  no_python: %.4f\n", r.NoPython)
	fmt.Printf("  behaviour: %.4f\n", r.Behaviour)
	fmt.Printf("  REWARD:    %.4f\n", r.Reward)
}

// ── secret ───────────────────────────────────────────────────────────────────

func handleSecret(behaviour float64) {
	fmt.Println()
	if behaviour < 1.0 {
		fmt.Println("No secret for you. Yet.")
		return
	}

	raw, err := os.ReadFile(secretFile)
	if err != nil {
		fmt.Println("*** SECRET UNLOCKED ***")
		fmt.Println("(could not read secret.b64:", err, ")")
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("*** SECRET UNLOCKED ***")
		fmt.Println("(could not decode secret:", err, ")")
		return
	}

	cmd := exec.Command(figletBin, "-f", "slant", string(decoded))
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard
	_ = cmd.Run()

	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(buf.String())
}
