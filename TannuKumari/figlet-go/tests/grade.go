package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	repoDir     = "/workspace/repo"
	binaryPath  = "/tmp/figlet"
	hiddenCases = "/tests/data/hidden.jsonl"
	secretPath  = "/tests/secret.b64"
	rewardPath  = "/logs/verifier/reward.json"
	caseTimeout = 5 * time.Second

	weightBuild     = 1.0
	weightNoPython  = 1.0
	weightBehaviour = 4.0
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type caseResult struct {
	index int
	ok    bool
	note  string
}

func loadCases(path string) ([]testCase, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []testCase
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var c testCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, nil
}

func runBuild() (bool, string) {
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "GOFLAGS=", "CGO_ENABLED=0", "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out))
	}
	if info, statErr := os.Stat(binaryPath); statErr != nil || info.IsDir() {
		return false, "go build reported success but /tmp/figlet is missing"
	}
	return true, ""
}

func findPythonFiles() []string {
	var found []string
	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".py") {
			found = append(found, path)
		}
		return nil
	})
	sort.Strings(found)
	return found
}

func runCase(c testCase) (bool, string) {
	cmd := exec.Command(binaryPath, c.Args...)
	// graded from a different working directory on purpose: the port must not
	// depend on being started inside the repository
	cmd.Dir = "/tmp"
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return false, "start: " + err.Error()
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return false, "exit: " + err.Error()
		}
	case <-time.After(caseTimeout):
		_ = cmd.Process.Kill()
		<-done
		return false, "timeout"
	}
	if stdout.String() != c.Out {
		return false, "output differs"
	}
	return true, ""
}

func runAllCases(cases []testCase) (int, []caseResult) {
	results := make([]caseResult, len(cases))
	workers := runtime.NumCPU() * 2
	if workers < 4 {
		workers = 4
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				ok, note := runCase(cases[i])
				results[i] = caseResult{index: i, ok: ok, note: note}
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	passed := 0
	for _, r := range results {
		if r.ok {
			passed++
		}
	}
	return passed, results
}

func writeReward(rewards map[string]float64) error {
	if err := os.MkdirAll(filepath.Dir(rewardPath), 0o755); err != nil {
		return err
	}
	blob, err := json.MarshalIndent(rewards, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rewardPath, append(blob, '\n'), 0o644)
}

func printSecret() {
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		fmt.Println("secret file unreadable:", err)
		return
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		fmt.Println("secret file is not base64:", err)
		return
	}
	cmd := exec.Command(binaryPath, "-f", "slant", string(text))
	cmd.Dir = "/tmp"
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("could not draw the secret:", err)
		return
	}
	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(string(out))
}

func main() {
	cases, err := loadCases(hiddenCases)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read hidden cases:", err)
		os.Exit(1)
	}
	fmt.Printf("hidden cases: %d\n", len(cases))

	build := 0.0
	buildOK, buildLog := runBuild()
	if buildOK {
		build = 1.0
		fmt.Println("build: ok")
	} else {
		fmt.Println("build: FAILED")
		if buildLog != "" {
			fmt.Println(indent(buildLog))
		}
	}

	behaviour := 0.0
	passed := 0
	var results []caseResult
	if buildOK {
		passed, results = runAllCases(cases)
		behaviour = float64(passed) / float64(len(cases))
	}
	fmt.Printf("behaviour: %d/%d cases match exactly (%.4f)\n", passed, len(cases), behaviour)

	pyFiles := findPythonFiles()
	noPython := 0.0
	switch {
	case !buildOK:
		fmt.Println("no_python: 0 (build failed)")
	case len(pyFiles) > 0:
		fmt.Printf("no_python: 0 (%d .py file(s) still present, e.g. %s)\n",
			len(pyFiles), pyFiles[0])
	case passed == 0:
		// deleting the Python without porting it must not pay
		fmt.Println("no_python: 0 (no .py files, but not a single hidden case matches)")
	default:
		noPython = 1.0
		fmt.Println("no_python: 1")
	}

	reward := (weightBuild*build + weightNoPython*noPython + weightBehaviour*behaviour) /
		(weightBuild + weightNoPython + weightBehaviour)

	if buildOK && behaviour < 1.0 {
		printFailures(cases, results, 5)
	}

	fmt.Printf("reward: %.6f\n", reward)
	if err := writeReward(map[string]float64{
		"reward":    reward,
		"build":     build,
		"no_python": noPython,
		"behaviour": behaviour,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write reward file:", err)
		os.Exit(1)
	}

	if behaviour == 1.0 {
		printSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func printFailures(cases []testCase, results []caseResult, limit int) {
	shown := 0
	for i, r := range results {
		if r.ok || shown >= limit {
			continue
		}
		shown++
		fmt.Printf("--- failing case %d: args=%q (%s)\n", i+1, cases[i].Args, r.note)
	}
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}
