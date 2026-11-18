// Command grade scores a Go port of pyfiglet.
//
// Criteria (weights in brackets):
//
//	build     [1]  `go build -o /tmp/figlet .` succeeds in the repository.
//	no_python [1]  build passed, no .py files remain, and >= 1 hidden case passes.
//	behaviour [4]  fraction of hidden cases whose stdout matches byte for byte
//	               (and whose exit status is 0, as recorded by the oracle).
//
// reward = (build + no_python + 4*behaviour) / 6, in [0, 1].
//
// The reward object is printed to stdout as JSON and written to -out.
// The grader exits non-zero only when it cannot grade (for example a missing
// or malformed dataset); a broken submission simply scores low.
//
// Standard library only.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	wBuild     = 1.0
	wNoPython  = 1.0
	wBehaviour = 4.0

	buildTimeout = 5 * time.Minute
	caseTimeout  = 10 * time.Second
	// The largest expected output is a few KB; anything far beyond that is
	// wrong anyway, so cap captured stdout to keep memory bounded.
	maxStdout = 1 << 20
)

type testCase struct {
	Line int
	Args []string
	Out  []byte
}

type reward struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
	Passed    int     `json:"passed"`
	Total     int     `json:"total"`
}

func main() {
	here := "."
	if exe, err := os.Executable(); err == nil {
		here = filepath.Dir(exe)
	}
	repo := flag.String("repo", "/workspace/repo", "repository to grade")
	cases := flag.String("cases", filepath.Join(here, "data", "hidden.jsonl"), "hidden test cases (JSONL)")
	secret := flag.String("secret", filepath.Join(here, "secret.b64"), "base64 secret shown on a perfect score")
	bin := flag.String("bin", "/tmp/figlet", "path for the built binary")
	out := flag.String("out", "", "also write the reward JSON to this file")
	jobs := flag.Int("jobs", runtime.NumCPU(), "parallel test executions")
	flag.Parse()

	if err := run(*repo, *cases, *secret, *bin, *out, *jobs); err != nil {
		fmt.Fprintln(os.Stderr, "grader error:", err)
		os.Exit(1)
	}
}

func run(repo, casesPath, secretPath, bin, outPath string, jobs int) error {
	// Load everything the grader needs before touching the submission, so a
	// broken task setup fails loudly instead of producing a bogus score.
	cases, err := loadCases(casesPath)
	if err != nil {
		return err
	}
	secretText, err := loadSecret(secretPath)
	if err != nil {
		return err
	}
	if jobs < 1 {
		jobs = 1
	}

	r := reward{Total: len(cases)}

	built := build(repo, bin)
	if built {
		r.Build = 1
		r.Passed = runCases(bin, cases, jobs)
		r.Behaviour = float64(r.Passed) / float64(len(cases))

		pyFiles, unreadable := findPython(repo)
		switch {
		case len(pyFiles) > 0:
			logf("no_python: %d .py file(s) remain, e.g. %s", len(pyFiles), pyFiles[0])
		case len(unreadable) > 0:
			logf("no_python: cannot scan %d path(s), e.g. %s", len(unreadable), unreadable[0])
		case r.Passed == 0:
			logf("no_python: no hidden case passed")
		default:
			r.NoPython = 1
		}
	}
	logf("behaviour: %d/%d hidden cases passed", r.Passed, r.Total)

	r.Reward = (wBuild*r.Build + wNoPython*r.NoPython + wBehaviour*r.Behaviour) /
		(wBuild + wNoPython + wBehaviour)

	if built && r.Passed == r.Total {
		showSecret(bin, secretText)
	}

	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, append(data, '\n'), 0o644); err != nil {
			return err
		}
	}
	fmt.Println(string(data))
	return nil
}

func logf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

// loadCases parses the JSONL dataset strictly: every non-empty line must be
// an object with exactly an "args" array of strings and an "out" string.
func loadCases(path string) ([]testCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cases: %w", err)
	}
	defer f.Close()

	var cases []testCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec struct {
			Args *[]string `json:"args"`
			Out  *string   `json:"out"`
		}
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&rec); err != nil {
			return nil, fmt.Errorf("%s:%d: malformed record: %v", path, n, err)
		}
		if dec.More() {
			return nil, fmt.Errorf("%s:%d: trailing data after record", path, n)
		}
		if rec.Args == nil || rec.Out == nil {
			return nil, fmt.Errorf("%s:%d: record needs both \"args\" and \"out\"", path, n)
		}
		cases = append(cases, testCase{Line: n, Args: *rec.Args, Out: []byte(*rec.Out)})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read cases: %w", err)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("%s: no test cases", path)
	}
	return cases, nil
}

func loadSecret(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read secret: %w", err)
	}
	dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	return string(dec), nil
}

// build compiles the submission, removing any stale binary first so a
// failed build can never be graded against an old or planted executable.
func build(repo, bin string) bool {
	if err := os.RemoveAll(bin); err != nil {
		logf("build: cannot remove old %s: %v", bin, err)
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOTOOLCHAIN=local")
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if err == nil {
		if st, serr := os.Stat(bin); serr != nil || !st.Mode().IsRegular() {
			err = fmt.Errorf("no binary produced at %s", bin)
		}
	}
	if err != nil {
		logf("build: FAILED: %v\n%s", err, tail(buf.String(), 4000))
		return false
	}
	logf("build: ok")
	return true
}

// findPython walks the whole repository, hidden directories included, and
// returns every entry whose name ends in .py (any case) that is not a
// directory, plus any paths it could not read. Symlinks are matched by name
// but never followed. An unreadable path means the absence of Python cannot
// be confirmed, so the caller treats it as a failed check rather than a
// grader error.
func findPython(repo string) (found, unreadable []string) {
	filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		rel, _ := filepath.Rel(repo, p)
		if err != nil {
			unreadable = append(unreadable, rel)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".py") {
			found = append(found, rel)
		}
		return nil
	})
	return found, unreadable
}

// runCases executes every case and returns how many passed. Each process
// gets its arguments directly (no shell), empty stdin, a fresh empty working
// directory and a timeout. The result does not depend on scheduling.
func runCases(bin string, cases []testCase, jobs int) int {
	pass := make([]bool, len(cases))
	idx := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				pass[i] = runOne(bin, cases[i])
			}
		}()
	}
	for i := range cases {
		idx <- i
	}
	close(idx)
	wg.Wait()

	n, shown := 0, 0
	for i, ok := range pass {
		if ok {
			n++
		} else if shown < 5 {
			shown++
			logf("FAIL line %d: args=%q", cases[i].Line, cases[i].Args)
		}
	}
	return n
}

func runOne(bin string, c testCase) bool {
	dir, err := os.MkdirTemp("", "figlet-case-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)

	stdout, code, err := execute(bin, c.Args, dir)
	return err == nil && code == 0 && bytes.Equal(stdout, c.Out)
}

// limitedBuffer keeps at most max bytes and records overflow.
type limitedBuffer struct {
	buf      bytes.Buffer
	max      int
	overflow bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); len(p) > room {
		l.overflow = true
		if room > 0 {
			l.buf.Write(p[:room])
		}
		return len(p), nil
	}
	return l.buf.Write(p)
}

func execute(bin string, args []string, dir string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Stdin = nil // /dev/null
	out := &limitedBuffer{max: maxStdout}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, -1, ctx.Err()
	}
	if out.overflow {
		return nil, -1, errors.New("stdout too large")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out.buf.Bytes(), exitErr.ExitCode(), nil
	}
	if err != nil {
		return nil, -1, err // could not start (missing or not executable)
	}
	return out.buf.Bytes(), 0, nil
}

// showSecret renders the decoded secret with the submission itself. It is
// cosmetic and never affects the score. Printed to stderr so stdout stays a
// single JSON object.
func showSecret(bin, text string) {
	dir, err := os.MkdirTemp("", "figlet-secret-")
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)
	stdout, code, err := execute(bin, []string{"-f", "slant", text}, dir)
	if err != nil || code != 0 {
		return
	}
	os.Stderr.Write(stdout)
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
