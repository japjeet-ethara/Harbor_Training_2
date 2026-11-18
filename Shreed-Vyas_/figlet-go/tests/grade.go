// Grader for the figlet-go Harbor task.
// reward = (build + noPython + 4*behaviour) / 6
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type testCase struct {
	ID       string   `json:"id"`
	Args     []string `json:"args"`
	Text     string   `json:"text"`
	Expected string   `json:"expected"`
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func goFiles(dir string) []string {
	var out []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".go") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// stdlibOnly: every import must be a standard-library path (first element has no dot).
func stdlibOnly(dir string) (bool, string) {
	fset := token.NewFileSet()
	for _, f := range goFiles(dir) {
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			return false, "parse error in " + f
		}
		for _, imp := range af.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if strings.Contains(strings.Split(p, "/")[0], ".") {
				return false, "non-stdlib import " + p
			}
		}
	}
	return true, ""
}

func checkBuild(dir, bin string) (bool, string) {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return false, "go.mod missing in " + dir
	}
	if ok, why := stdlibOnly(dir); !ok {
		return false, why
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if len(msg) > 800 {
			msg = msg[:800]
		}
		return false, "go build failed: " + msg
	}
	return true, ""
}

var pyLiteral = regexp.MustCompile("(?is)(\"[^\"\\n]*python[^\"\\n]*\")|(`[^`]*python[^`]*`)")

func checkNoPython(dir string) (bool, string) {
	for _, name := range []string{"python3", "python"} {
		if _, err := exec.LookPath(name); err == nil {
			return false, name + " is on PATH"
		}
	}
	bad := ""
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || bad != "" {
			return nil
		}
		low := strings.ToLower(p)
		if strings.HasSuffix(low, ".py") || strings.HasSuffix(low, ".pyc") || strings.HasSuffix(low, ".pyw") {
			bad = "python file: " + p
			return nil
		}
		if strings.HasSuffix(low, ".go") {
			data, _ := os.ReadFile(p)
			if pyLiteral.Match(data) {
				bad = "string literal referring to python in " + p
			}
		}
		return nil
	})
	return bad == "", bad
}

func runCase(bin, dir string, tc testCase) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := append(append([]string{}, tc.Args...), "--", tc.Text)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false, "(run error: " + err.Error() + ")"
	}
	got := out.String()
	return got == tc.Expected, got
}

func loadCases(path string) ([]testCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	var cases []testCase
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var tc testCase
		if err := json.Unmarshal([]byte(line), &tc); err != nil {
			return nil, err
		}
		cases = append(cases, tc)
	}
	return cases, sc.Err()
}

func main() {
	repo := env("REPO_DIR", "/workspace/repo")
	tests := env("TESTS_DIR", "/tests")
	logDir := env("LOG_DIR", "/logs/verifier")
	dataset := env("GRADE_DATA", "hidden") // "visible" for debugging
	proj := filepath.Join(repo, "figlet")
	bin := filepath.Join(os.TempDir(), "figlet-under-test")
	os.MkdirAll(logDir, 0o777)

	build, noPy := 0.0, 0.0
	buildOK, why := checkBuild(proj, bin)
	if buildOK {
		build = 1
	}
	fmt.Printf("[build]     %v %s\n", buildOK, why)
	pyOK, pyWhy := checkNoPython(proj)
	fmt.Printf("[no-python] %v %s\n", pyOK, pyWhy)

	passed, total := 0, 0
	if buildOK {
		cases, err := loadCases(filepath.Join(tests, "data", dataset+".jsonl"))
		if err != nil {
			fmt.Println("cannot load cases:", err)
		}
		total = len(cases)
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		shown := 0
		for _, tc := range cases {
			wg.Add(1)
			sem <- struct{}{}
			go func(tc testCase) {
				defer wg.Done()
				defer func() { <-sem }()
				ok, got := runCase(bin, repo, tc)
				mu.Lock()
				defer mu.Unlock()
				if ok {
					passed++
				} else if dataset == "visible" && shown < 3 {
					shown++
					fmt.Printf("\n--- MISMATCH %s args=%q text=%q\n--- expected:\n%s--- got:\n%s\n", tc.ID, tc.Args, tc.Text, tc.Expected, got)
				}
			}(tc)
		}
		wg.Wait()
	}
	frac := 0.0
	if total > 0 {
		frac = float64(passed) / float64(total)
	}
	// "No Python" credit requires a program that actually runs (>=1 case right),
	// so an empty stub cannot collect free points.
	if buildOK && pyOK && passed > 0 {
		noPy = 1
	}
	reward := (build + noPy + 4*frac) / 6
	fmt.Printf("[behaviour] %d/%d (%s) = %.4f\n", passed, total, dataset, frac)
	fmt.Printf("[reward]    %.4f\n", reward)
	os.WriteFile(filepath.Join(logDir, "reward.txt"), []byte(fmt.Sprintf("%.4f\n", reward)), 0o644)

	if dataset == "hidden" && total > 0 && passed == total {
		if raw, err := os.ReadFile(filepath.Join(tests, "secret.b64")); err == nil {
			if dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw))); err == nil {
				fmt.Println("[secret]    " + string(dec))
				os.WriteFile(filepath.Join(logDir, "secret.txt"), dec, 0o644)
			}
		}
	}
}
