// Grader for figlet-go. Go, not Python: the room has no Python.
//
// build      (weight 1): `go build -o /tmp/figlet .` in /workspace/repo succeeds
// no_python  (weight 1): build passed AND no .py under /workspace/repo AND at least one hidden case matches
// behaviour  (weight 4): share of /tests/data/hidden.jsonl cases whose stdout equals "out" byte for byte
// reward = (build + no_python + 4*behaviour) / 6 -> /logs/verifier/reward.json
//
// Expected: nothing done 0; build only 1/6; perfect 1.0.
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
)

const (
	repoDir    = "/workspace/repo"
	binary     = "/tmp/figlet"
	hiddenFile = "/tests/data/hidden.jsonl"
	secretFile = "/tests/secret.b64"
	rewardFile = "/logs/verifier/reward.json"
	perCase    = 5 * time.Second
	showFails  = 5
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func runCase(tc testCase) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), perCase)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, tc.Args...)
	cmd.Dir = "/" // the program must work from any working directory
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("timeout after %s", perCase)
	}
	return stdout.Bytes(), err
}

func main() {
	build := 0.0
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("build: FAIL (%v)\n%s\n", err, out)
	} else {
		build = 1
		fmt.Println("build: ok")
	}

	var pyFiles []string
	filepath.WalkDir(repoDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
			pyFiles = append(pyFiles, p)
		}
		return nil
	})

	passed, total, shown := 0, 0, 0
	if build == 1 {
		f, err := os.Open(hiddenFile)
		if err != nil {
			fmt.Printf("cannot open %s: %v\n", hiddenFile, err)
			os.Exit(1)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var tc testCase
			if err := json.Unmarshal(line, &tc); err != nil {
				fmt.Printf("bad case line: %v\n", err)
				os.Exit(1)
			}
			total++
			got, err := runCase(tc)
			if err == nil && bytes.Equal(got, []byte(tc.Out)) {
				passed++
				continue
			}
			if shown < showFails {
				shown++
				fmt.Printf("--- FAIL %v (err=%v)\n--- expected:\n%s--- actual:\n%s", tc.Args, err, tc.Out, got)
			}
		}
		f.Close()
	}
	behaviour := 0.0
	if total > 0 {
		behaviour = float64(passed) / float64(total)
	}
	noPython := 0.0
	if build == 1 && len(pyFiles) == 0 && passed > 0 {
		noPython = 1
	}
	fmt.Printf("behaviour: %d/%d cases match\n", passed, total)
	fmt.Printf("no_python: %v (%d .py files, %d right answers)\n", noPython == 1, len(pyFiles), passed)
	for i, p := range pyFiles {
		if i == 10 {
			fmt.Println("  ...")
			break
		}
		fmt.Println("  ", p)
	}

	reward := (build + noPython + 4*behaviour) / 6
	result := map[string]float64{"reward": reward, "build": build, "no_python": noPython, "behaviour": behaviour}
	os.MkdirAll(filepath.Dir(rewardFile), 0o755)
	data, _ := json.Marshal(result)
	if err := os.WriteFile(rewardFile, data, 0o644); err != nil {
		fmt.Printf("cannot write %s: %v\n", rewardFile, err)
		os.Exit(1)
	}
	fmt.Printf("reward: %s\n", data)

	if behaviour == 1.0 {
		raw, err := os.ReadFile(secretFile)
		if err != nil {
			fmt.Printf("secret file missing: %v\n", err)
			return
		}
		secret, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil {
			fmt.Printf("secret not base64: %v\n", err)
			return
		}
		banner, _ := exec.Command(binary, "-f", "slant", string(secret)).Output()
		fmt.Printf("*** SECRET UNLOCKED ***\n%s", banner)
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}
