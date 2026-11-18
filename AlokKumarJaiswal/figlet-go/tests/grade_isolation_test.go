package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestGradeIsolation(t *testing.T) {
	// 1. Require Linux and root; fail clearly otherwise.
	if runtime.GOOS != "linux" {
		t.Fatalf("test requires linux, running on %s", runtime.GOOS)
	}
	if os.Geteuid() != 0 {
		t.Fatalf("grader must run as root (effective UID 0), running as UID %d", os.Geteuid())
	}

	// 2. Prepare temporary solution gocache/gopath directories owned 1000:1000.
	// Ensure their parent directories allow player traversal.
	// Use t.Setenv for SOLUTION_GOCACHE/SOLUTION_GOPATH.
	// Arrange cleanup.
	baseDir, err := os.MkdirTemp("", "sol-isolation-*")
	if err != nil {
		t.Fatalf("failed to create temporary base directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(baseDir)
	})

	if err := os.Chmod(baseDir, 0755); err != nil {
		t.Fatalf("failed to chmod temporary base directory: %v", err)
	}

	solGocache := filepath.Join(baseDir, "gocache")
	solGopath := filepath.Join(baseDir, "gopath")

	if err := os.Mkdir(solGocache, 0755); err != nil {
		t.Fatalf("failed to create solution gocache directory: %v", err)
	}
	if err := os.Mkdir(solGopath, 0755); err != nil {
		t.Fatalf("failed to create solution gopath directory: %v", err)
	}

	if err := os.Chown(solGocache, 1000, 1000); err != nil {
		t.Fatalf("failed to chown solution gocache to 1000:1000: %v", err)
	}
	if err := os.Chown(solGopath, 1000, 1000); err != nil {
		t.Fatalf("failed to chown solution gopath to 1000:1000: %v", err)
	}

	t.Setenv("SOLUTION_GOCACHE", solGocache)
	t.Setenv("SOLUTION_GOPATH", solGopath)

	// 3. Call validateSolutionCache and require success.
	if err := validateSolutionCache(); err != nil {
		t.Fatalf("validateSolutionCache failed: %v", err)
	}

	// 4. Launch id -u, id -g and id -G through configureSolutionCmd,
	// each with a bounded timeout.
	// Require UID 1000, GID 1000 and group list exactly 1000.
	runID := func(flag string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "id", flag)
		if err := configureSolutionCmd(cmd); err != nil {
			t.Fatalf("configureSolutionCmd for id %s failed: %v", flag, err)
		}

		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("executing id %s failed: %v", flag, err)
		}
		return strings.TrimSpace(string(out))
	}

	uid := runID("-u")
	if uid != "1000" {
		t.Fatalf("expected UID 1000, got %q", uid)
	}

	gid := runID("-g")
	if gid != "1000" {
		t.Fatalf("expected GID 1000, got %q", gid)
	}

	groups := runID("-G")
	if groups != "1000" {
		t.Fatalf("expected group list exactly 1000, got %q", groups)
	}

	// 5. As root, confirm the hidden file can be opened and immediately closed.
	// Do not read any bytes.
	hiddenPath := "/tests/data/hidden.jsonl"
	hiddenFile, err := os.Open(hiddenPath)
	if err != nil {
		t.Fatalf("as root, failed to open %s: %v", hiddenPath, err)
	}
	if err := hiddenFile.Close(); err != nil {
		t.Fatalf("failed to close %s: %v", hiddenPath, err)
	}

	// 6. Build /tools/probe_hidden_access.go into a temporary executable
	// accessible to player, using a go build command configured through
	// configureSolutionCmd and a 120-second timeout.
	probeDir, err := os.MkdirTemp("", "probe-access-*")
	if err != nil {
		t.Fatalf("failed to create probe temp directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(probeDir)
	})

	if err := os.Chmod(probeDir, 0777); err != nil {
		t.Fatalf("failed to chmod probe directory: %v", err)
	}
	if err := os.Chown(probeDir, 1000, 1000); err != nil {
		t.Fatalf("failed to chown probe directory to 1000:1000: %v", err)
	}

	probeBin := filepath.Join(probeDir, "probe_hidden_access")

	buildCtx, buildCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer buildCancel()

	buildCmd := exec.CommandContext(buildCtx, "go", "build", "-o", probeBin, "/tools/probe_hidden_access.go")
	buildCmd.Dir = "/tmp"
	if err := configureSolutionCmd(buildCmd); err != nil {
		t.Fatalf("configureSolutionCmd for go build failed: %v", err)
	}

	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build /tools/probe_hidden_access.go failed: %v\nOutput: %s", err, string(buildOut))
	}

	if fi, err := os.Stat(probeBin); err != nil {
		t.Fatalf("probe executable %s not found: %v", probeBin, err)
	} else if fi.Mode()&0111 == 0 {
		t.Fatalf("probe binary %s is not executable (mode: %v)", probeBin, fi.Mode())
	}

	// 7. Execute that probe through configureSolutionCmd, cwd=/tmp,
	// with a 5-second timeout.
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer probeCancel()

	probeCmd := exec.CommandContext(probeCtx, probeBin)
	probeCmd.Dir = "/tmp"
	if err := configureSolutionCmd(probeCmd); err != nil {
		t.Fatalf("configureSolutionCmd for probe execution failed: %v", err)
	}

	probeOut, probeErr := probeCmd.CombinedOutput()

	// 8. Require exit status 1 and output containing:
	//    Effective UID: 1000
	//    HIDDEN_FILE_OPEN_FAILED
	//    permission denied
	//    Require HIDDEN_FILE_OPEN_SUCCEEDED to be absent.
	if probeErr == nil {
		t.Fatalf("expected probe execution to fail with exit status 1, but it succeeded. Output:\n%s", string(probeOut))
	}

	var exitErr *exec.ExitError
	if !errors.As(probeErr, &exitErr) {
		t.Fatalf("expected exec.ExitError from probe execution, got: %v", probeErr)
	}

	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected probe exit status 1, got %d. Output:\n%s", exitErr.ExitCode(), string(probeOut))
	}

	probeOutStr := string(probeOut)

	if !strings.Contains(probeOutStr, "Effective UID: 1000") {
		t.Fatalf("probe output missing required substring 'Effective UID: 1000'. Output:\n%s", probeOutStr)
	}

	if !strings.Contains(probeOutStr, "HIDDEN_FILE_OPEN_FAILED") {
		t.Fatalf("probe output missing required substring 'HIDDEN_FILE_OPEN_FAILED'. Output:\n%s", probeOutStr)
	}

	if !strings.Contains(probeOutStr, "permission denied") && !strings.Contains(strings.ToLower(probeOutStr), "permission denied") {
		t.Fatalf("probe output missing required substring 'permission denied'. Output:\n%s", probeOutStr)
	}

	if strings.Contains(probeOutStr, "HIDDEN_FILE_OPEN_SUCCEEDED") {
		t.Fatalf("probe output unexpectedly contains forbidden substring 'HIDDEN_FILE_OPEN_SUCCEEDED'. Output:\n%s", probeOutStr)
	}

	// 9. Print only identity results and the access-denied probe output.
	fmt.Printf("uid: %s\ngid: %s\ngroups: %s\n", uid, gid, groups)
	fmt.Print(probeOutStr)
	if !strings.HasSuffix(probeOutStr, "\n") {
		fmt.Println()
	}
}
