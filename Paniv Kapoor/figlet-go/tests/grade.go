package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type RewardData struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func main() {
	repoDir := getEnv("REPO_DIR", "/workspace/repo")
	testsDir := getEnv("TESTS_DIR", "/tests")
	figletBin := getEnv("FIGLET_BIN", "/tmp/figlet")
	logsDir := getEnv("LOGS_DIR", "/logs/verifier")

	fmt.Println("==========================================")
	fmt.Println("PyFiglet Go Task Grader")
	fmt.Printf("Repo Directory:   %s\n", repoDir)
	fmt.Printf("Tests Directory:  %s\n", testsDir)
	fmt.Printf("Figlet Binary:    %s\n", figletBin)
	fmt.Printf("Logs Directory:   %s\n", logsDir)
	fmt.Println("==========================================")

	// 1. Build check (weight 1)
	fmt.Println("\n[1/3] Running Build Check...")
	buildCmd := exec.Command("go", "build", "-o", figletBin, ".")
	buildCmd.Dir = repoDir
	buildOutput, buildErr := buildCmd.CombinedOutput()

	buildPassed := (buildErr == nil)
	buildScore := 0.0
	if buildPassed {
		buildScore = 1.0
		fmt.Println("  Build Check: PASSED (score: 1.0)")
	} else {
		fmt.Printf("  Build Check: FAILED (score: 0.0)\n  Error: %v\n  Output:\n%s\n", buildErr, string(buildOutput))
	}

	// 2. Load hidden test cases
	hiddenFile := filepath.Join(testsDir, "data", "hidden.jsonl")
	cases, err := loadTestCases(hiddenFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Grader failure: unable to load hidden test cases from %s: %v\n", hiddenFile, err)
		os.Exit(1)
	}
	fmt.Printf("\nLoaded %d hidden test cases from %s\n", len(cases), hiddenFile)

	// 3. Behaviour check (weight 4)
	fmt.Println("\n[2/3] Running Behaviour Check...")
	exactMatches := 0
	totalCases := len(cases)
	behaviourScore := 0.0

	if buildPassed && totalCases > 0 {
		numWorkers := runtime.NumCPU()
		if numWorkers < 1 {
			numWorkers = 1
		} else if numWorkers > 16 {
			numWorkers = 16
		}

		results := make([]bool, totalCases)
		taskCh := make(chan int, totalCases)
		for i := 0; i < totalCases; i++ {
			taskCh <- i
		}
		close(taskCh)

		var wg sync.WaitGroup
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for idx := range taskCh {
					tc := cases[idx]
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					cmd := exec.CommandContext(ctx, figletBin, tc.Args...)
					stdout, cmdErr := cmd.Output()
					cancel()

					if cmdErr == nil && string(stdout) == tc.Out {
						results[idx] = true
					}
				}
			}()
		}
		wg.Wait()

		for _, passed := range results {
			if passed {
				exactMatches++
			}
		}

		behaviourScore = float64(exactMatches) / float64(totalCases)
		fmt.Printf("  Behaviour Check: %d / %d exact matches (score: %.4f)\n", exactMatches, totalCases, behaviourScore)
	} else {
		fmt.Println("  Behaviour Check: SKIPPED (build failed or no test cases, score: 0.0)")
	}

	// 4. No-Python check (weight 1)
	fmt.Println("\n[3/3] Running No-Python Check...")
	hasPyFiles := checkForPythonFiles(repoDir)
	noPythonScore := 0.0
	if buildPassed && !hasPyFiles && exactMatches >= 1 {
		noPythonScore = 1.0
		fmt.Println("  No-Python Check: PASSED (no .py files found, build passed, and at least 1 match; score: 1.0)")
	} else {
		fmt.Printf("  No-Python Check: FAILED (buildPassed=%v, hasPyFiles=%v, exactMatches=%d; score: 0.0)\n",
			buildPassed, hasPyFiles, exactMatches)
	}

	// 5. Calculate reward
	reward := (buildScore*1.0 + noPythonScore*1.0 + behaviourScore*4.0) / 6.0

	// 6. Write /logs/verifier/reward.json
	rewardObj := RewardData{
		Reward:    reward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}

	rewardJSON, err := json.MarshalIndent(rewardObj, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Grader failure: failed to marshal reward JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Grader failure: failed to create logs directory %s: %v\n", logsDir, err)
		os.Exit(1)
	}

	rewardFilePath := filepath.Join(logsDir, "reward.json")
	if err := os.WriteFile(rewardFilePath, append(rewardJSON, '\n'), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Grader failure: failed to write %s: %v\n", rewardFilePath, err)
		os.Exit(1)
	}

	// 7. Print concise score summary
	fmt.Println("\n==========================================")
	fmt.Println("Score Summary")
	fmt.Println("==========================================")
	fmt.Printf("Build Check (weight 1):     %.4f\n", buildScore)
	fmt.Printf("No-Python Check (weight 1): %.4f\n", noPythonScore)
	fmt.Printf("Behaviour Check (weight 4): %.4f (%d/%d matches)\n", behaviourScore, exactMatches, totalCases)
	fmt.Printf("Weighted Reward:            %.6f\n", reward)
	fmt.Printf("Reward JSON saved to:       %s\n", rewardFilePath)
	fmt.Println("==========================================")

	// 8. Secret handling
	if behaviourScore == 1.0 {
		secretPath := filepath.Join(testsDir, "secret.b64")
		secretB64Bytes, err := os.ReadFile(secretPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Grader failure: failed to read %s: %v\n", secretPath, err)
			os.Exit(1)
		}

		decodedText, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secretB64Bytes)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Grader failure: failed to decode base64 secret: %v\n", err)
			os.Exit(1)
		}

		secretCmd := exec.Command(figletBin, "-f", "slant", string(decodedText))
		secretOutput, err := secretCmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Grader failure: failed to run secret command: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n*** SECRET UNLOCKED ***")
		fmt.Print(string(secretOutput))
	} else {
		fmt.Println("\nNo secret for you. Yet.")
	}
}

func loadTestCases(path string) ([]TestCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cases []TestCase
	scanner := bufio.NewScanner(file)
	// Set scanner buffer up to 1MB to safely handle long lines
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var tc TestCase
		if err := json.Unmarshal([]byte(line), &tc); err != nil {
			return nil, fmt.Errorf("invalid json line: %w", err)
		}
		cases = append(cases, tc)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cases, nil
}

func checkForPythonFiles(repoDir string) bool {
	hasPyFiles := false
	_ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			hasPyFiles = true
			return fs.SkipAll
		}
		return nil
	})
	return hasPyFiles
}
