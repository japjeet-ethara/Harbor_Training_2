package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// record is a single JSONL line from hidden.jsonl.
type record struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

// rewardDoc is the JSON written to /logs/verifier/reward.json.
type rewardDoc struct {
	Reward    float64 `json:"reward"`
	Build     int     `json:"build"`
	NoPython  int     `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "VERIFIER ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// ------------------------------------------------------------------ //
	// Preconditions: root execution & solution-cache validation          //
	// ------------------------------------------------------------------ //
	if os.Geteuid() != 0 {
		return fmt.Errorf("grader must run as root (effective UID 0), got UID %d", os.Geteuid())
	}

	if err := validateSolutionCache(); err != nil {
		return fmt.Errorf("solution-cache validation failed: %w", err)
	}

	// ------------------------------------------------------------------ //
	// 0. Remove any stale reward file from a previous run.               //
	// ------------------------------------------------------------------ //
	if err := os.Remove("/logs/verifier/reward.json"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing stale reward.json: %w", err)
	}

	// ------------------------------------------------------------------ //
	// 1. Load and validate hidden.jsonl                                   //
	// ------------------------------------------------------------------ //
	fmt.Println("=== LOADING HIDDEN DATA ===")
	records, err := loadHidden("/tests/data/hidden.jsonl")
	if err != nil {
		return fmt.Errorf("loading hidden.jsonl: %w", err)
	}
	const requiredRecords = 2000
	if len(records) != requiredRecords {
		return fmt.Errorf("expected exactly %d records, got %d", requiredRecords, len(records))
	}
	fmt.Printf("Loaded %d records.\n", len(records))

	// ------------------------------------------------------------------ //
	// 2. Build                                                            //
	// ------------------------------------------------------------------ //
	fmt.Println("\n=== BUILD ===")
	build := 0
	buildPassed := false

	// Remove any stale binary first.
	if err2 := os.Remove("/tmp/figlet"); err2 != nil && !os.IsNotExist(err2) {
		return fmt.Errorf("removing stale /tmp/figlet: %w", err2)
	}

	const buildTimeout = 120 * time.Second

	ok1, diag1 := runCmd(buildTimeout, "/workspace/repo", "go", "build", "-o", "/tmp/figlet", ".")
	if !ok1 {
		fmt.Println("--- go build -o /tmp/figlet . FAILED ---")
		fmt.Println(diag1)
	}

	ok2, diag2 := runCmd(buildTimeout, "/workspace/repo", "go", "build", "./...")
	if !ok2 {
		fmt.Println("--- go build ./... FAILED ---")
		fmt.Println(diag2)
	}

	if ok1 && ok2 {
		// -------------------------------------------------------------- //
		// 2b. Dependency check: Go standard library only                 //
		// -------------------------------------------------------------- //
		fmt.Println("\n--- Dependency check (stdlib-only rule) ---")
		depsOK, depsDiag := checkDepsStdlibOnly("/workspace/repo", buildTimeout)
		if depsOK {
			build = 1
			buildPassed = true
			fmt.Println("Build: PASS (including stdlib-only check)")
		} else {
			fmt.Println("--- stdlib-only dependency check FAILED ---")
			fmt.Println(depsDiag)
			fmt.Println("Build: FAIL — stdlib-only rule violated, skipping behavior tests")
		}
	} else {
		fmt.Println("Build: FAIL — skipping behavior tests")
	}

	// ------------------------------------------------------------------ //
	// 3. Behavior                                                         //
	// ------------------------------------------------------------------ //
	fmt.Println("\n=== BEHAVIOR ===")
	matched := 0
	total := len(records)

	if buildPassed {
		for _, rec := range records {
			if runCase(rec) {
				matched++
			}
		}
		fmt.Printf("Cases: %d / %d matched\n", matched, total)
	} else {
		fmt.Println("Skipped (build failed).")
	}

	behaviour := 0.0
	if buildPassed && total > 0 {
		behaviour = float64(matched) / float64(total)
	}

	// ------------------------------------------------------------------ //
	// 4. no_python                                                        //
	// ------------------------------------------------------------------ //
	fmt.Println("\n=== NO_PYTHON CHECK ===")
	noPython := 0
	if buildPassed {
		hasPy, scanErr := hasPythonFiles("/workspace/repo")
		if scanErr != nil {
			return fmt.Errorf("scanning for Python files: %w", scanErr)
		}
		if hasPy {
			fmt.Println("Python files found — no_python = 0")
		} else if matched == 0 {
			fmt.Println("No Python files, but zero hidden cases matched — no_python = 0")
		} else {
			noPython = 1
			fmt.Println("No Python files, at least one case matched — no_python = 1")
		}
	} else {
		fmt.Println("Build failed — no_python = 0")
	}

	// ------------------------------------------------------------------ //
	// 5. Secret (must complete before reward is written)                 //
	// ------------------------------------------------------------------ //
	fmt.Println("\n=== SECRET ===")
	if matched != total {
		fmt.Println("No secret for you. Yet.")
	} else {
		// All hidden cases matched — attempt to unlock.
		secretB64, err4 := os.ReadFile("/tests/secret.b64")
		if err4 != nil {
			return fmt.Errorf("reading secret: invalid or missing secret file")
		}
		decoded, err5 := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secretB64)))
		if err5 != nil {
			return fmt.Errorf("decoding secret: invalid encoding")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/tmp/figlet", "-f", "slant", string(decoded))
		cmd.Dir = "/tmp"
		if err := configureSolutionCmd(cmd); err != nil {
			return fmt.Errorf("configuring secret banner command: %w", err)
		}
		var stdoutBuf strings.Builder
		cmd.Stdout = &stdoutBuf
		// Discard stderr — must not leak it in error messages.
		cmd.Stderr = nil
		if err6 := cmd.Run(); err6 != nil {
			return fmt.Errorf("rendering secret: banner command failed")
		}

		fmt.Println("*** SECRET UNLOCKED ***")
		fmt.Print(stdoutBuf.String())
	}

	// ------------------------------------------------------------------ //
	// 6. Reward (written after secret branch completes)                  //
	// ------------------------------------------------------------------ //
	fmt.Println("\n=== REWARD ===")
	reward := (float64(build) + float64(noPython) + 4.0*behaviour) / 6.0

	fmt.Printf("build     = %d\n", build)
	fmt.Printf("no_python = %d\n", noPython)
	fmt.Printf("behaviour = %.6f\n", behaviour)
	fmt.Printf("reward    = %.6f\n", reward)

	doc := rewardDoc{
		Reward:    reward,
		Build:     build,
		NoPython:  noPython,
		Behaviour: behaviour,
	}
	if err3 := writeReward(doc); err3 != nil {
		return fmt.Errorf("writing reward.json: %w", err3)
	}
	fmt.Println("Wrote /logs/verifier/reward.json")

	return nil
}

// ------------------------------------------------------------------ //
// Solution Subprocess Configuration & Validation Helpers              //
// ------------------------------------------------------------------ //

// validateSolutionCache ensures SOLUTION_GOCACHE and SOLUTION_GOPATH are
// set, exist, are directories, and are accessible/owned by player (1000:1000).
func validateSolutionCache() error {
	solGocache := os.Getenv("SOLUTION_GOCACHE")
	if strings.TrimSpace(solGocache) == "" {
		return fmt.Errorf("SOLUTION_GOCACHE environment variable is missing or empty")
	}
	solGopath := os.Getenv("SOLUTION_GOPATH")
	if strings.TrimSpace(solGopath) == "" {
		return fmt.Errorf("SOLUTION_GOPATH environment variable is missing or empty")
	}

	for _, p := range []string{solGocache, solGopath} {
		fi, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("solution cache path %q unusable: %w", p, err)
		}
		if !fi.IsDir() {
			return fmt.Errorf("solution cache path %q is not a directory", p)
		}
		if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
			if stat.Uid != 1000 || stat.Gid != 1000 {
				return fmt.Errorf("solution cache path %q must be owned by 1000:1000, got %d:%d", p, stat.Uid, stat.Gid)
			}
		}
	}
	return nil
}

// configureSolutionCmd is the common helper for configuring solution subprocesses.
// It ensures child processes run with UID 1000, GID 1000, cleared supplementary
// groups on Linux, and a minimal environment with solution cache settings.
// Existing dependency unit tests calling this helper directly as player (UID/GID 1000)
// are supported using their test environment; unexpected identities are rejected.
func configureSolutionCmd(cmd *exec.Cmd) error {
	if cmd == nil {
		return fmt.Errorf("nil exec.Cmd")
	}

	euid := os.Geteuid()
	egid := os.Getegid()

	if euid == 1000 && egid == 1000 {
		// Caller is already unprivileged player (e.g. dependency unit tests).
		// Use test environment without attempting to modify process credentials.
		if cmd.Env == nil {
			solGocache := os.Getenv("SOLUTION_GOCACHE")
			solGopath := os.Getenv("SOLUTION_GOPATH")
			if solGocache != "" || solGopath != "" {
				env := os.Environ()
				if solGocache != "" {
					env = append(env, "GOCACHE="+solGocache)
				}
				if solGopath != "" {
					env = append(env, "GOPATH="+solGopath)
				}
				cmd.Env = env
			}
		}
		return nil
	}

	if euid != 0 {
		return fmt.Errorf("unexpected process identity: UID %d, GID %d (expected root 0 or player 1000:1000)", euid, egid)
	}

	// Caller is root. Privilege drop is mandatory; never silently fall back to executing as root.
	if runtime.GOOS != "linux" {
		return fmt.Errorf("solution subprocess isolation to UID 1000 is only supported on linux, running on %s", runtime.GOOS)
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid:         1000,
		Gid:         1000,
		Groups:      []uint32{},
		NoSetGroups: false,
	}

	solGocache := os.Getenv("SOLUTION_GOCACHE")
	if strings.TrimSpace(solGocache) == "" {
		return fmt.Errorf("missing or empty SOLUTION_GOCACHE")
	}
	solGopath := os.Getenv("SOLUTION_GOPATH")
	if strings.TrimSpace(solGopath) == "" {
		return fmt.Errorf("missing or empty SOLUTION_GOPATH")
	}

	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}

	env := []string{
		"PATH=" + path,
		"HOME=/home/player",
		"CGO_ENABLED=0",
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOCACHE=" + solGocache,
		"GOPATH=" + solGopath,
	}

	// Preserve GOWORK behavior unless explicitly controlled by an existing focused test.
	goworkSet := false
	for _, kv := range cmd.Env {
		if strings.HasPrefix(kv, "GOWORK=") {
			env = append(env, kv)
			goworkSet = true
			break
		}
	}
	if !goworkSet {
		if gw, ok := os.LookupEnv("GOWORK"); ok {
			env = append(env, "GOWORK="+gw)
		}
	}

	cmd.Env = env
	return nil
}

// configureCmd is an alias for configureSolutionCmd.
func configureCmd(cmd *exec.Cmd) error {
	return configureSolutionCmd(cmd)
}

// configureSolutionSubprocess is an alias for configureSolutionCmd.
func configureSolutionSubprocess(cmd *exec.Cmd) error {
	return configureSolutionCmd(cmd)
}

// ------------------------------------------------------------------ //
// Helpers                                                            //
// ------------------------------------------------------------------ //

// loadHidden reads and validates all JSONL records from path.
func loadHidden(path string) ([]record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var records []record
	lineNum := 0
	scanner := bufio.NewScanner(f)
	// Use a large buffer to handle long JSON lines.
	const maxLine = 16 * 1024 * 1024 // 16 MiB
	buf := make([]byte, maxLine)
	scanner.Buffer(buf, maxLine)

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		// Blank physical lines are invalid.
		if len(line) == 0 {
			return nil, fmt.Errorf("line %d: unexpected blank line", lineNum)
		}

		// Decode into a raw map first to enforce exact key set and types.
		var raw map[string]json.RawMessage
		if err2 := json.Unmarshal(line, &raw); err2 != nil {
			return nil, fmt.Errorf("line %d: invalid record", lineNum)
		}

		// Record must be an object with exactly "args" and "out".
		if len(raw) != 2 {
			return nil, fmt.Errorf("line %d: record must have exactly 2 fields", lineNum)
		}
		rawArgs, hasArgs := raw["args"]
		rawOut, hasOut := raw["out"]
		if !hasArgs || !hasOut {
			return nil, fmt.Errorf("line %d: record must contain exactly \"args\" and \"out\"", lineNum)
		}

		// "args" must be a non-null JSON array.
		if string(rawArgs) == "null" {
			return nil, fmt.Errorf("line %d: \"args\" must not be null", lineNum)
		}
		var argsRaw []json.RawMessage
		if err3 := json.Unmarshal(rawArgs, &argsRaw); err3 != nil {
			return nil, fmt.Errorf("line %d: \"args\" must be an array", lineNum)
		}
		// Each element of args must be a JSON string (not null).
		args := make([]string, len(argsRaw))
		for i, elem := range argsRaw {
			if string(elem) == "null" {
				return nil, fmt.Errorf("line %d: element %d of \"args\" must not be null", lineNum, i)
			}
			var s string
			if err4 := json.Unmarshal(elem, &s); err4 != nil {
				return nil, fmt.Errorf("line %d: element %d of \"args\" must be a string", lineNum, i)
			}
			args[i] = s
		}

		// "out" must be a JSON string (not null); empty string is valid.
		if string(rawOut) == "null" {
			return nil, fmt.Errorf("line %d: \"out\" must not be null", lineNum)
		}
		var out string
		if err5 := json.Unmarshal(rawOut, &out); err5 != nil {
			return nil, fmt.Errorf("line %d: \"out\" must be a string", lineNum)
		}

		records = append(records, record{Args: args, Out: out})
	}
	if err6 := scanner.Err(); err6 != nil {
		return nil, fmt.Errorf("scanning hidden.jsonl: %w", err6)
	}
	return records, nil
}

// runCmd runs the given command with a timeout, returning (success, diagnosticOutput).
func runCmd(timeout time.Duration, dir string, name string, args ...string) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if err := configureSolutionCmd(cmd); err != nil {
		return false, fmt.Sprintf("configuring solution command failed: %v", err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, string(out)
	}
	return true, ""
}

// runCase executes /tmp/figlet with the record's args and checks the result.
func runCase(rec record) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/tmp/figlet", rec.Args...)
	cmd.Dir = "/tmp"
	if err := configureSolutionCmd(cmd); err != nil {
		return false
	}
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		return false
	}
	return stdoutBuf.String() == rec.Out
}

// hasPythonFiles recursively scans root for any .py files (case-insensitive).
func hasPythonFiles(root string) (bool, error) {
	found := false
	var scanErr error
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Surface scan errors rather than silently skipping.
			scanErr = fmt.Errorf("accessing %s: %w", path, err)
			return scanErr
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			found = true
			// No need to continue walking.
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && scanErr == nil {
		scanErr = err
	}
	return found, scanErr
}

// goListModule holds the Module field within a go list -json package object.
type goListModule struct {
	Path    string        `json:"Path"`
	Main    bool          `json:"Main"`
	Replace *goListModule `json:"Replace"`
}

// goListPackage is a single JSON object from go list -deps -json ./...
type goListPackage struct {
	ImportPath string        `json:"ImportPath"`
	Standard   bool          `json:"Standard"`
	Dir        string        `json:"Dir"`
	Module     *goListModule `json:"Module"`
}

// checkDepsStdlibOnly runs "go list -deps -json ./..." in repoDir, decodes the
// stream of package objects, and enforces the Go-standard-library-only rule
// from instruction.md §4. It returns (true, "") on success, or (false, diag)
// where diag carries human-readable diagnostics (never hidden case content).
func checkDepsStdlibOnly(repoDir string, timeout time.Duration) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-json", "./...")
	cmd.Dir = repoDir
	if err := configureSolutionCmd(cmd); err != nil {
		return false, fmt.Sprintf("configuring solution command failed: %v", err)
	}
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return false, "go list -deps -json ./... timed out after 120 s"
	}
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return false, fmt.Sprintf("go list -deps -json ./... failed: %v\n%s", err, stderr)
	}

	// Resolve repoDir once, following symlinks, for containment checks.
	resolvedRepo, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		return false, fmt.Sprintf("cannot resolve repo directory %q: %v", repoDir, err)
	}
	// Ensure the resolved path has no trailing separator; filepath.Clean
	// normalizes the path. The subsequent boundary check detects escape.
	resolvedRepo = filepath.Clean(resolvedRepo)

	// The output of go list -deps -json is a stream of JSON objects, not a
	// JSON array.  We use a streaming decoder to handle it.
	dec := json.NewDecoder(strings.NewReader(string(out)))
	pkgCount := 0
	for {
		var pkg goListPackage
		if err2 := dec.Decode(&pkg); err2 != nil {
			if err2 == io.EOF {
				break
			}
			return false, fmt.Sprintf("malformed JSON from go list: %v", err2)
		}
		pkgCount++

		// Standard library packages are always allowed.
		if pkg.Standard {
			continue
		}

		// Non-standard package: apply the allow-list.
		//
		// Rule: Module must exist, Module.Main=true, no Module.Replace,
		// and the package directory must be strictly inside repoDir.

		if pkg.Module == nil {
			return false, fmt.Sprintf(
				"dependency check FAILED: non-standard package %q has no module info (possible vendored or GOPATH dependency)",
				pkg.ImportPath,
			)
		}
		if !pkg.Module.Main {
			return false, fmt.Sprintf(
				"dependency check FAILED: package %q belongs to non-main module %q (third-party dependency not allowed)",
				pkg.ImportPath, pkg.Module.Path,
			)
		}
		if pkg.Module.Replace != nil {
			return false, fmt.Sprintf(
				"dependency check FAILED: package %q is provided via a module replacement (%q → %q) which is not allowed",
				pkg.ImportPath, pkg.Module.Path, pkg.Module.Replace.Path,
			)
		}

		// Containment check using filepath.Rel — guards against naive prefix
		// collisions such as /workspace/repo2 looking like it starts with
		// /workspace/repo.
		if pkg.Dir == "" {
			return false, fmt.Sprintf(
				"dependency check FAILED: package %q has no directory (cannot verify containment)",
				pkg.ImportPath,
			)
		}
		resolvedPkgDir, err3 := filepath.EvalSymlinks(pkg.Dir)
		if err3 != nil {
			return false, fmt.Sprintf(
				"dependency check FAILED: cannot resolve directory of package %q (%q): %v",
				pkg.ImportPath, pkg.Dir, err3,
			)
		}
		resolvedPkgDir = filepath.Clean(resolvedPkgDir)

		rel, err4 := filepath.Rel(resolvedRepo, resolvedPkgDir)
		if err4 != nil {
			return false, fmt.Sprintf(
				"dependency check FAILED: filepath.Rel error for package %q: %v",
				pkg.ImportPath, err4,
			)
		}
		// A relative path starting with ".." means the package directory is
		// outside the repo root.
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return false, fmt.Sprintf(
				"dependency check FAILED: package %q (dir %q) is outside the repo root %q",
				pkg.ImportPath, resolvedPkgDir, resolvedRepo,
			)
		}
	}

	if pkgCount == 0 {
		return false, "dependency check FAILED: go list reported zero packages"
	}

	fmt.Printf("Dependency check passed: %d package(s) examined, all stdlib or in-repo.\n", pkgCount)
	return true, ""
}

// writeReward serialises doc and writes it to /logs/verifier/reward.json.
func writeReward(doc rewardDoc) error {
	if err := os.MkdirAll("/logs/verifier", 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile("/logs/verifier/reward.json", data, 0o600)
}
