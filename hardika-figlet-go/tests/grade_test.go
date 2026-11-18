package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadCases(t *testing.T) {
	for _, input := range []string{"", "{}", "{\"args\":[\"hello\"]}", "bad JSON", "{\"args\":[\"a\"],\"out\":\"\"}\n{\"args\":[\"a\"],\"out\":\"b\"}"} {
		if _, err := loadCases(strings.NewReader(input)); err == nil {
			t.Errorf("accepted invalid records %q", input)
		}
	}
	cases, err := loadCases(strings.NewReader("{\"args\":[\"a\"],\"out\":\" \\n\\n\"}\n"))
	if err != nil || len(cases) != 1 || cases[0].Out != " \n\n" {
		t.Fatalf("parsed %#v, %v", cases, err)
	}
}

func TestScore(t *testing.T) {
	cases := []struct {
		name       string
		evaluation evaluation
		want       score
	}{
		{"nothing", evaluation{Total: 2}, score{}},
		{"build only", evaluation{Built: true, Total: 2}, score{Build: 1, Reward: 1.0 / 6}},
		{"perfect", evaluation{Built: true, Total: 2, Matched: 2}, score{Build: 1, Behaviour: 1, NoPython: 1, Reward: 1}},
		{"python present", evaluation{Built: true, HasPython: true, Total: 2, Matched: 1}, score{Build: 1, Behaviour: 0.5, Reward: 0.5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := calculate(tc.evaluation); got != tc.want {
				t.Fatalf("score = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestPythonInspection(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "hidden")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if found, err := hasPython(dir); err != nil || found {
		t.Fatalf("empty repository returned %t, %v", found, err)
	}
	if err := os.WriteFile(filepath.Join(nested, "reference.py"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if found, err := hasPython(dir); err != nil || !found {
		t.Fatalf("nested Python file returned %t, %v", found, err)
	}
}

func TestRunCase(t *testing.T) {
	got, err := runCase("/bin/sh", []string{"-c", "printf 'A  \\n\\n'"}, time.Second)
	if err != nil || string(got) != "A  \n\n" {
		t.Fatalf("stdout = %q, %v", got, err)
	}
	if _, err := runCase("/bin/sh", []string{"-c", "exec sleep 30"}, 30*time.Millisecond); err == nil {
		t.Fatal("timed command returned success")
	}
}

func TestCompileAndReadReward(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "candidate")
	if err := compile(dir, target); err == nil {
		t.Fatal("empty directory built successfully")
	}
	for name, text := range map[string]string{"go.mod": "module candidate\n\ngo 1.23\n", "main.go": "package main\n\nfunc main() {}\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := compile(dir, target); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(dir, "logs", "reward.json")
	if err := writeReward(resultPath, score{Build: 1}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(resultPath)
	if err != nil || string(got) != "{\"reward\":0,\"build\":1,\"no_python\":0,\"behaviour\":0}\n" {
		t.Fatalf("reward = %q, %v", got, err)
	}
}
