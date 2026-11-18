package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWeighted(t *testing.T) {
	for _, test := range []struct {
		score score
		want  float64
	}{
		{score{}, 0},
		{score{Build: 1}, 1.0 / 6},
		{score{Build: 1, NoPython: 1, Behaviour: .5}, 4.0 / 6},
		{score{Build: 1, NoPython: 1, Behaviour: 1}, 1},
	} {
		if got := weighted(test.score); got != test.want {
			t.Fatalf("weighted(%+v) = %v, want %v", test.score, got, test.want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test-program")
	script := "#!/bin/sh\ncase \"$1\" in\nhang) exec sleep 2;;\nfail) exit 2;;\nesac\nprintf '%s\\n' \"$1\"\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	const cases = `{"args":["good"],"out":"good\n"}
{"args":["good"],"out":"good"}
{"args":["hang"],"out":""}
{"args":["fail"],"out":""}`
	total, matches, timeouts, failures, err := evaluate(strings.NewReader(cases), true, path, 150*time.Millisecond)
	if err != nil || total != 4 || matches != 1 || timeouts != 1 || failures != 1 {
		t.Fatalf("evaluate = %d, %d, %d, %d, %v", total, matches, timeouts, failures, err)
	}
	total, matches, timeouts, failures, err = evaluate(strings.NewReader(cases), false, path, time.Millisecond)
	if err != nil || total != 4 || matches != 0 || timeouts != 0 || failures != 0 {
		t.Fatalf("evaluate without build = %d, %d, %d, %d, %v", total, matches, timeouts, failures, err)
	}
}

func TestRejectsMalformedCorpus(t *testing.T) {
	for _, data := range []string{"", "{", "{\"args\":[]}"} {
		if _, _, _, _, err := evaluate(strings.NewReader(data), false, "", time.Second); err == nil {
			t.Errorf("accepted %q", data)
		}
	}
}

func TestNoPython(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "nested")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if clean, err := noPython(root); err != nil || !clean {
		t.Fatalf("clean tree = %v, %v", clean, err)
	}
	if err := os.WriteFile(filepath.Join(folder, "leftover.py"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if clean, err := noPython(root); err != nil || clean {
		t.Fatalf("tree with .py = %v, %v", clean, err)
	}
}
