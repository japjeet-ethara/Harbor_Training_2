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

// TestCase represents one line of the JSONL test dataset.
type TestCase struct {
	Args   []string `json:"args"`
	Stdout string   `json:"stdout"`
}

// RewardReport represents the structured JSON output written for Harbor.
type RewardReport struct {
	Reward         float64 `json:"reward"`
	BuildScore     float64 `json:"build_score"`
	NoPythonScore  float64 `json:"no_python_score"`
	BehaviourScore float64 `json:"behaviour_score"`
	MatchedCases   int     `json:"matched_cases"`
	TotalCases     int     `json:"total_cases"`
}

// resolvePath checks multiple candidate locations and returns the first one that exists.
func resolvePath(candidates ...string) string {
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

// runBuildCheck attempts to compile the candidate Go program.
// It executes 'go build -o <binaryPath> .' inside repoDir.
// Returns 1.0 if the build succeeds, or 0.0 on failure.
func runBuildCheck(repoDir, binaryPath string) float64 {
	fmt.Printf("[Check 1/3] Building candidate binary from %s ...\n", repoDir)

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = repoDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("  [-] Build failed: %v\n", err)
		if stderr.Len() > 0 {
			fmt.Printf("      Error details: %s\n", strings.TrimSpace(stderr.String()))
		}
		return 0.0
	}

	fmt.Printf("  [+] Build succeeded -> binary at %s\n", binaryPath)
	return 1.0
}

// checkForPythonFiles inspects the candidate repository directory to verify that no .py files exist.
// It specifically ignores the Harbor task's test infrastructure files (gen.py, test_outputs.py, etc.).
// Returns true if no candidate Python files are found, false otherwise.
func checkForPythonFiles(repoDir string) bool {
	foundPython := false
	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip test infrastructure or version control directories if within repoDir
		if info.IsDir() {
			if info.Name() == "tests" || info.Name() == ".git" {
				if _, err := os.Stat(filepath.Join(path, "grade.go")); err == nil {
					return filepath.SkipDir
				}
				if _, err := os.Stat(filepath.Join(path, "gen.py")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		// Ignore Harbor task's own test infrastructure files
		if info.Name() == "gen.py" || info.Name() == "test_outputs.py" {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".py") {
			fmt.Printf("  [-] Found forbidden Python file in candidate repo: %s\n", path)
			foundPython = true
		}
		return nil
	})
	return !foundPython
}

// runBehaviourCheck reads hidden.jsonl and executes the candidate binary against every test case.
// Returns (matchedCases, totalCases, behaviourScore).
func runBehaviourCheck(binaryPath, jsonlPath string) (int, int, float64) {
	fmt.Printf("[Check 3/3] Running behavioural evaluation against %s ...\n", jsonlPath)

	file, err := os.Open(jsonlPath)
	if err != nil {
		fmt.Printf("  [-] Error opening dataset %s: %v\n", jsonlPath, err)
		return 0, 0, 0.0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Allocate a larger buffer in case a test case output is big
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	totalCases := 0
	matchedCases := 0
	caseIndex := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var tc TestCase
		if err := json.Unmarshal([]byte(line), &tc); err != nil {
			fmt.Printf("  [-] Warning: could not parse JSONL line %d: %v\n", caseIndex+1, err)
			continue
		}

		totalCases++
		caseIndex++

		// Execute candidate binary with 5s timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, binaryPath, tc.Args...)

		var stdoutBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf

		execErr := cmd.Run()
		cancel()

		if execErr == nil {
			// Compare byte-for-byte against expected stdout
			if stdoutBuf.String() == tc.Stdout {
				matchedCases++
			}
		}

		if caseIndex%500 == 0 {
			fmt.Printf("  ... evaluated %d cases (%d matching so far)\n", caseIndex, matchedCases)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("  [-] Error reading dataset lines: %v\n", err)
	}

	score := 0.0
	if totalCases > 0 {
		score = float64(matchedCases) / float64(totalCases)
	}

	fmt.Printf("  [+] Behaviour check complete: %d / %d cases matched (score: %.4f)\n",
		matchedCases, totalCases, score)
	return matchedCases, totalCases, score
}

// unlockSecret reads tests/secret.b64, decodes it, and renders it using the candidate binary with the slant font.
func unlockSecret(binaryPath, secretPath string) {
	data, err := os.ReadFile(secretPath)
	if err != nil {
		fmt.Printf("  [-] Error reading secret file: %v\n", err)
		return
	}

	trimmed := strings.TrimSpace(string(data))
	decodedBytes, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		fmt.Printf("  [-] Error decoding secret: %v\n", err)
		return
	}
	secretMessage := string(decodedBytes)

	// Render with slant font
	cmd := exec.Command(binaryPath, "-f", "slant", secretMessage)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  [-] Error rendering secret with candidate binary: %v\n", err)
		return
	}

	fmt.Println("\n*** SECRET UNLOCKED ***")
	fmt.Print(string(out))
}

// saveRewards writes reward.json and reward.txt to the specified log directory.
func saveRewards(logDir string, report RewardReport) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("  [-] Notice: could not create log directory %s: %v\n", logDir, err)
		fallback := "./verifier_logs"
		if fallbackErr := os.MkdirAll(fallback, 0755); fallbackErr == nil {
			logDir = fallback
			fmt.Printf("  [+] Falling back to log directory: %s\n", logDir)
		} else {
			return
		}
	}

	// Write reward.json
	jsonPath := filepath.Join(logDir, "reward.json")
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		if writeErr := os.WriteFile(jsonPath, jsonData, 0644); writeErr == nil {
			fmt.Printf("Saved structured report -> %s\n", jsonPath)
		}
	}

	// Write reward.txt
	txtPath := filepath.Join(logDir, "reward.txt")
	txtData := fmt.Sprintf("%.6f\n", report.Reward)
	if writeErr := os.WriteFile(txtPath, []byte(txtData), 0644); writeErr == nil {
		fmt.Printf("Saved reward value -> %s\n", txtPath)
	}
}

func main() {
	fmt.Println("==================================================")
	fmt.Println("           FIGLET-GO HARBOR GRADER               ")
	fmt.Println("==================================================")

	// 1. Resolve environment paths
	repoDir := resolvePath("/workspace/repo", ".", "..")
	binaryPath := resolvePath("/tmp/figlet", filepath.Join(os.TempDir(), "figlet"))
	hiddenJSONL := resolvePath("/tests/hidden.jsonl", "tests/hidden.jsonl", "hidden.jsonl")
	secretPath := resolvePath("/tests/secret.b64", "tests/secret.b64", "secret.b64")
	logDir := resolvePath("/logs/verifier", "./verifier_logs")

	// Allow environment variable overrides
	if val := os.Getenv("REPO_DIR"); val != "" {
		repoDir = val
	}
	if val := os.Getenv("BINARY_PATH"); val != "" {
		binaryPath = val
	}
	if val := os.Getenv("HIDDEN_JSONL"); val != "" {
		hiddenJSONL = val
	}
	if val := os.Getenv("SECRET_PATH"); val != "" {
		secretPath = val
	}
	if val := os.Getenv("LOG_DIR"); val != "" {
		logDir = val
	}

	// 2. Build Check
	buildScore := runBuildCheck(repoDir, binaryPath)

	// 3. Behaviour Check (only if build succeeded)
	matchedCases := 0
	totalCases := 0
	behaviourScore := 0.0

	if buildScore > 0.0 {
		matchedCases, totalCases, behaviourScore = runBehaviourCheck(binaryPath, hiddenJSONL)
	} else {
		fmt.Println("[Check 3/3] Skipping behavioural evaluation because build failed.")
	}

	// 4. No-Python Check:
	// "Check that no .py files exist anywhere in the candidate repository.
	// Also require at least one hidden test case to match exactly.
	// Return 1 only if both conditions pass; otherwise return 0."
	fmt.Printf("[Check 2/3] Checking for forbidden Python (.py) files in %s ...\n", repoDir)
	cleanRepo := checkForPythonFiles(repoDir)
	noPythonScore := 0.0
	if cleanRepo && matchedCases >= 1 {
		noPythonScore = 1.0
		fmt.Println("  [+] No Python files found and matched >= 1 case -> Pass (1.0)")
	} else {
		if !cleanRepo {
			fmt.Println("  [-] Repository contains .py files -> Fail (0.0)")
		} else {
			fmt.Println("  [-] Did not match at least 1 test case -> Fail (0.0)")
		}
	}

	// 5. Scoring
	// Weights: Build (1), No-Python (1), Behaviour (4)
	// Weighted mean: (build*1 + no_python*1 + behaviour*4) / 6
	finalReward := (buildScore*1.0 + noPythonScore*1.0 + behaviourScore*4.0) / 6.0

	fmt.Println("\n==================================================")
	fmt.Println("                 GRADING SUMMARY                  ")
	fmt.Println("==================================================")
	fmt.Printf("Build Score:     %.2f / 1.00  (weight: 1)\n", buildScore)
	fmt.Printf("No-Python Score: %.2f / 1.00  (weight: 1)\n", noPythonScore)
	fmt.Printf("Behaviour Score: %.4f / 1.00  (weight: 4) [%d/%d matched]\n",
		behaviourScore, matchedCases, totalCases)
	fmt.Println("--------------------------------------------------")
	fmt.Printf("FINAL REWARD:    %.6f / 1.000000\n", finalReward)
	fmt.Println("==================================================")

	// 6. Secret Unlocking
	fmt.Println("\n--- Secret Status ---")
	if behaviourScore == 1.0 {
		unlockSecret(binaryPath, secretPath)
	} else {
		fmt.Println("No secret for you. Yet.")
	}

	// 7. Save outputs for Harbor
	report := RewardReport{
		Reward:         finalReward,
		BuildScore:     buildScore,
		NoPythonScore:  noPythonScore,
		BehaviourScore: behaviourScore,
		MatchedCases:   matchedCases,
		TotalCases:     totalCases,
	}
	saveRewards(logDir, report)
}
