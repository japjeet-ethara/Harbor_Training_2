// grade.go grades the figlet-go Harbor task.
//
// It builds the agent's Go program, checks the repository for Python files,
// compares the program's stdout against every hidden case, and writes a
// weighted reward to /logs/verifier/reward.json.
//
// Only the Go standard library is used.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	repoDir   = "/workspace/repo"
	binary    = "/tmp/figlet"
	testsDir  = "/tests"
	hidden    = testsDir + "/data/hidden.jsonl"
	secret    = testsDir + "/secret.b64"
	rewardOut = "/logs/verifier/reward.json"

	buildTimeout  = 180 * time.Second
	caseTimeout   = 5 * time.Second
	secretTimeout = 10 * time.Second

	buildWeight     = 1.0
	noPythonWeight  = 1.0
	behaviourWeight = 4.0
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type reward struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	buildOK := doBuild()

	cases, err := loadCases(hidden)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot load hidden cases: %v\n", err)
	}

	matched := 0
	total := 0
	if buildOK && err == nil {
		matched, total = runBehaviour(cases)
	}

	behaviour := 0.0
	if total > 0 {
		behaviour = float64(matched) / float64(total)
	}

	noPython := 0.0
	if buildOK && !hasPythonFiles(repoDir) && matched > 0 {
		noPython = 1.0
	}

	build := 0.0
	if buildOK {
		build = 1.0
	}

	weighted := (build*buildWeight + noPython*noPythonWeight + behaviour*behaviourWeight) /
		(buildWeight + noPythonWeight + behaviourWeight)

	writeReward(reward{
		Reward:    weighted,
		Build:     build,
		NoPython:  noPython,
		Behaviour: behaviour,
	})

	fmt.Printf("build=%v no_python=%v behaviour=%.6f (%d/%d cases) reward=%.6f\n",
		build, noPython, behaviour, matched, total, weighted)

	if behaviour == 1.0 {
		printSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

// doBuild runs `go build -o /tmp/figlet .` in the repository.
func doBuild() bool {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, goBinary(), "build", "-o", binary, ".")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, out)
		return false
	}
	fmt.Println("build: ok")
	return true
}

// goBinary returns the go executable, falling back to common install paths
// when PATH does not contain it.
func goBinary() string {
	if path, err := exec.LookPath("go"); err == nil {
		return path
	}
	for _, candidate := range []string{
		"/usr/local/go/bin/go",
		"/usr/lib/go/bin/go",
		"/usr/bin/go",
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "go"
}

// loadCases reads a JSONL file of hidden test cases.
func loadCases(path string) ([]testCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cases []testCase
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var c testCase
		if err := json.Unmarshal(line, &c); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, lineNo, err)
		}
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cases, nil
}

// runBehaviour runs every case and returns the number that matched and the
// total number of cases.
func runBehaviour(cases []testCase) (matched, total int) {
	for i, c := range cases {
		out, ok := runCase(c.Args)
		if ok && bytes.Equal(out, []byte(c.Out)) {
			matched++
		}
		if (i+1)%250 == 0 {
			fmt.Printf("behaviour: %d/%d cases checked, %d matched\n", i+1, len(cases), matched)
		}
	}
	return matched, len(cases)
}

// runCase runs /tmp/figlet with args and returns its stdout. ok is false only
// when the process timed out.
func runCase(args []string) (stdout []byte, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	_ = cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, false
	}
	return buf.Bytes(), true
}

// hasPythonFiles reports whether any .py file exists under root.
func hasPythonFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".py") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

// writeReward writes the reward file, creating its parent directory if needed.
func writeReward(r reward) {
	data, err := json.Marshal(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot encode reward: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(rewardOut), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create reward directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(rewardOut, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write reward: %v\n", err)
		os.Exit(1)
	}
}

// printSecret decodes /tests/secret.b64 and renders it with the slant font.
func printSecret() {
	raw, err := os.ReadFile(secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "secret unavailable: %v\n", err)
		return
	}

	// Be tolerant of newlines/whitespace in the base64 file.
	encoded := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, string(raw))

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "secret decode failed: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), secretTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "-f", "slant", string(data))
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintln(os.Stderr, "secret render timed out")
		} else {
			fmt.Fprintf(os.Stderr, "secret render failed: %v\n", err)
		}
		return
	}

	fmt.Println("*** SECRET UNLOCKED ***")
	os.Stdout.Write(buf.Bytes())
	if b := buf.Bytes(); len(b) == 0 || b[len(b)-1] != '\n' {
		fmt.Println()
	}
}
