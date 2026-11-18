// Grader for figlet-go. Run by tests/test.sh inside the task image after the agent is done.
//
//	build      (weight 1)  `go build -o /tmp/figlet .` in /workspace/repo succeeds
//	no_python  (weight 1)  build passed, no .py file under /workspace/repo, >= 1 hidden case with output matched
//	behaviour  (weight 4)  share of /tests/data/hidden.jsonl cases whose stdout equals "out" exactly
//
// Writes /logs/verifier/reward.json. Standard library only; no network.
package main

import (
	"bufio"
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
	repo   = "/workspace/repo"
	binary = "/tmp/figlet"
	hidden = "/tests/data/hidden.jsonl"
	secret = "/tests/secret.b64"
	reward = "/logs/verifier/reward.json"
)

type testCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	os.MkdirAll(filepath.Dir(reward), 0o755)

	build := 0.0
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err == nil {
		build = 1
	} else {
		fmt.Printf("build failed: %v\n%s\n", err, out)
	}

	matched, total, shown, realMatch := 0, 0, 0, 0
	if build == 1 {
		f, err := os.Open(hidden)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			var c testCase
			if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
				fmt.Println("bad case:", err)
				os.Exit(1)
			}
			total++
			got := run(c.Args)
			if got == c.Out {
				matched++
				if c.Out != "" { // a binary that prints nothing matches the "too wide" cases
					realMatch++
				}
			} else if shown < 5 {
				shown++
				fmt.Printf("MISMATCH %q\n--- expected\n%s--- got\n%s\n", c.Args, c.Out, got)
			}
		}
		f.Close()
	}
	behaviour := 0.0
	if total > 0 {
		behaviour = float64(matched) / float64(total)
	}

	var pyFiles []string
	filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".py") {
			pyFiles = append(pyFiles, p)
		}
		return nil
	})
	noPython := 0.0
	if build == 1 && len(pyFiles) == 0 && realMatch >= 1 {
		noPython = 1
	}

	r := map[string]float64{
		"build":     build,
		"no_python": noPython,
		"behaviour": behaviour,
		"reward":    (build + noPython + 4*behaviour) / 6,
	}
	js, _ := json.Marshal(r)
	if err := os.WriteFile(reward, js, 0o644); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("matched %d/%d hidden cases, %d .py files left\n%s\n", matched, total, len(pyFiles), js)

	if behaviour != 1 {
		fmt.Println("No secret for you. Yet.")
		return
	}
	b64, err := os.ReadFile(secret)
	if err != nil {
		fmt.Println(err)
		return
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b64)))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(run([]string{"-f", "slant", string(text)}))
}

// run executes the agent's binary from a neutral working directory with a 5 s limit and
// returns its stdout (empty on crash or timeout, which is what pyfiglet's failures print).
func run(args []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, binary, args...)
	c.Dir = "/"
	out, _ := c.Output()
	return string(out)
}
