package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// mustWrite writes content to path, creating parent directories as needed.
// It calls t.Fatalf on any error.
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mustWrite: MkdirAll %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("mustWrite: WriteFile %q: %v", path, err)
	}
}

// setOfflineEnv configures the process environment so that every go tool
// invocation in the test runs fully offline and reproducibly.
func setOfflineEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("CGO_ENABLED", "0")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOWORK", "off")
}

// ---------------------------------------------------------------------------
// Scenario 1: PASS — main package importing only stdlib (fmt).
// ---------------------------------------------------------------------------

func TestCheckDepsStdlibOnly_Pass_FmtOnly(t *testing.T) {
	setOfflineEnv(t)

	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "go.mod"),
		"module example.com/fmtonly\n\ngo 1.21\n")

	mustWrite(t, filepath.Join(dir, "main.go"),
		"package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n")

	const timeout = 60 * time.Second
	ok, diag := checkDepsStdlibOnly(dir, timeout)
	if !ok {
		t.Fatalf("checkDepsStdlibOnly: expected PASS for stdlib-only module, got FAIL: %s", diag)
	}
}

// ---------------------------------------------------------------------------
// Scenario 2: PASS — main module with an in-repo sub-package.
// ---------------------------------------------------------------------------

func TestCheckDepsStdlibOnly_Pass_InRepoSubpackage(t *testing.T) {
	setOfflineEnv(t)

	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "go.mod"),
		"module example.com/withlib\n\ngo 1.21\n")

	// Internal library package — lives inside the same module.
	mustWrite(t,
		filepath.Join(dir, "internal", "greeting", "greeting.go"),
		"package greeting\n\nimport \"fmt\"\n\nfunc Hello(name string) string { return fmt.Sprintf(\"Hello, %s!\", name) }\n",
	)

	// Main package imports fmt and the in-repo internal package.
	mainSrc := fmt.Sprintf(
		"package main\n\nimport (\n\t\"fmt\"\n\t\"example.com/withlib/internal/greeting\"\n)\n\nfunc main() { fmt.Println(greeting.Hello(\"world\")) }\n",
	)
	mustWrite(t, filepath.Join(dir, "main.go"), mainSrc)

	const timeout = 60 * time.Second
	ok, diag := checkDepsStdlibOnly(dir, timeout)
	if !ok {
		t.Fatalf("checkDepsStdlibOnly: expected PASS for in-repo sub-package, got FAIL: %s", diag)
	}
}

// ---------------------------------------------------------------------------
// Scenario 3: REJECT — separate local module via require + replace.
// ---------------------------------------------------------------------------

func TestCheckDepsStdlibOnly_Reject_ExternalLocalModule(t *testing.T) {
	setOfflineEnv(t)

	// ---- Dependency module in its own temp directory ----

	depDir := t.TempDir()

	mustWrite(t, filepath.Join(depDir, "go.mod"),
		"module example.com/greeter\n\ngo 1.21\n")

	mustWrite(t, filepath.Join(depDir, "greeter.go"),
		"package greeter\n\nimport \"fmt\"\n\nfunc Greet(name string) string { return fmt.Sprintf(\"Greetings, %s!\", name) }\n")

	// ---- Consumer module that pulls in the dependency via replace ----

	mainDir := t.TempDir()

	// The replace directive uses an absolute path so that the go tool can find
	// the module on disk without any network access.
	goMod := fmt.Sprintf(
		"module example.com/consumer\n\ngo 1.21\n\nrequire example.com/greeter v0.0.0\n\nreplace example.com/greeter => %s\n",
		depDir,
	)
	mustWrite(t, filepath.Join(mainDir, "go.mod"), goMod)

	mustWrite(t, filepath.Join(mainDir, "main.go"),
		"package main\n\nimport (\n\t\"fmt\"\n\t\"example.com/greeter\"\n)\n\nfunc main() { fmt.Println(greeter.Greet(\"tester\")) }\n")

	// ---- Pre-flight: confirm the fixture builds correctly offline ----
	//
	// This step separates "fixture is broken" from "policy correctly rejected
	// a valid module".  If go build fails here the fixture must be fixed first.

	const buildTimeout = 60 * time.Second
	buildCtx, buildCancel := context.WithTimeout(context.Background(), buildTimeout)
	defer buildCancel()
	buildCmd := exec.CommandContext(buildCtx, "go", "build", "./...")
	buildCmd.Dir = mainDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf(
			"fixture pre-flight: go build ./... failed in %q — the fixture itself is broken, fix it before testing the policy.\nOutput:\n%s",
			mainDir, out,
		)
	}

	// ---- Require checkDepsStdlibOnly to reject the fixture ----

	const checkTimeout = 60 * time.Second
	ok, diag := checkDepsStdlibOnly(mainDir, checkTimeout)
	if ok {
		t.Fatalf(
			"checkDepsStdlibOnly: expected FAIL for module with an external local dependency (require+replace), but got PASS",
		)
	}
	// Report the diagnostic for transparency; exact wording is not asserted.
	t.Logf("checkDepsStdlibOnly correctly rejected the fixture. Diagnostic: %s", diag)
}
