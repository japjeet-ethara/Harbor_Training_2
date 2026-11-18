package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScoring(t *testing.T) {
	for _, tc := range []struct {
		name         string
		built, clean bool
		results      []outcome
		want         scores
	}{
		{"nop", false, false, nil, scores{}},
		{"build only", true, true, []outcome{{}}, scores{1.0 / 6, 1, 0, 0}},
		{"perfect", true, true, []outcome{{match: true}, {match: true}}, scores{1, 1, 1, 1}},
		{"partial", true, true, []outcome{{match: true}, {}}, scores{4.0 / 6, 1, 1, 0.5}},
		{"Python present", true, false, []outcome{{match: true}, {}}, scores{0.5, 1, 0, 0.5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := grade(tc.built, tc.clean, tc.results); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestJSONLAndPythonScan(t *testing.T) {
	out := strings.Repeat(" ", 100_000)
	rows, err := readCases(strings.NewReader("{\"args\":[\"a\"],\"out\":\"" + out + "\"}\n"))
	if err != nil || len(rows) != 1 || rows[0].Out != out {
		t.Fatalf("could not read a long record: %v", err)
	}
	for _, input := range []string{"", "garbage\n", "{\"out\":\"\"}\n"} {
		if _, err := readCases(strings.NewReader(input)); err == nil {
			t.Errorf("accepted invalid JSONL %q", input)
		}
	}
	dir := t.TempDir()
	if !hasNoPython(dir) {
		t.Fatal("empty directory should pass")
	}
	hidden := filepath.Join(dir, ".cache")
	if err := os.Mkdir(hidden, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "x.py"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if hasNoPython(dir) {
		t.Fatal("missed hidden Python file")
	}
}

func TestRunnerExactOutputAndTimeout(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, tc := range []struct {
		command string
		limit   time.Duration
		want    bool
	}{
		{"printf 'A \\n'", time.Second, true},
		{"printf 'A\\n'", time.Second, false},
		{"printf 'A \\n\\n'", time.Second, false},
		{"printf 'A \\n'; exit 1", time.Second, false},
		{"sleep 2; printf 'A \\n'", 30 * time.Millisecond, false},
	} {
		item := testCase{Args: []string{"-c", tc.command}, Out: "A \n"}
		if got := runCase(shell, dir, tc.limit, item).match; got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.command, got, tc.want)
		}
	}
}
