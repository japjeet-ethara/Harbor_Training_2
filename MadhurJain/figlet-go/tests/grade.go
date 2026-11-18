package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// types
// ---------------------------------------------------------------------------

type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type Result struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

// ---------------------------------------------------------------------------
// 1. build (weight 1)
// ---------------------------------------------------------------------------

func checkBuild() (bool, string) {
	// go.mod must have no require lines
	data, err := os.ReadFile("/workspace/repo/go.mod")
	if err != nil {
		return false, fmt.Sprintf("cannot read go.mod: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "require") {
			return false, "go.mod contains require lines (third-party dependencies are not allowed)"
		}
	}

	// go build -o /tmp/figlet .
	cmd1 := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	cmd1.Dir = "/workspace/repo"
	if out, err := cmd1.CombinedOutput(); err != nil {
		return false, fmt.Sprintf("go build -o /tmp/figlet . failed:\n%s", out)
	}

	// go build -o /dev/null ./...
	cmd2 := exec.Command("go", "build", "-o", "/dev/null", "./...")
	cmd2.Dir = "/workspace/repo"
	if out, err := cmd2.CombinedOutput(); err != nil {
		return false, fmt.Sprintf("go build -o /dev/null ./... failed:\n%s", out)
	}

	return true, ""
}

// ---------------------------------------------------------------------------
// 2. behaviour (weight 4)
// ---------------------------------------------------------------------------

func runBehaviour() (passed, total int) {
	f, err := os.Open("/tests/data/hidden.jsonl")
	if err != nil {
		fmt.Printf("Cannot open hidden.jsonl: %v\n", err)
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 64*1024*1024) // 64 MB
	scanner.Buffer(buf, 64*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var tc TestCase
		if err := json.Unmarshal(line, &tc); err != nil {
			fmt.Printf("JSON parse error: %v\n", err)
			continue
		}

		total++

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "/tmp/figlet", tc.Args...)
		cmd.Dir = "/tmp"
		out, _ := cmd.Output()
		timedOut := ctx.Err() != nil
		cancel()

		if !timedOut && bytes.Equal(out, []byte(tc.Out)) {
			passed++
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Scanner error: %v\n", err)
	}

	return passed, total
}

// ---------------------------------------------------------------------------
// 3. no_python (weight 1)
// ---------------------------------------------------------------------------

func hasPyFiles() bool {
	found := false
	filepath.Walk("/workspace/repo", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".py") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// ---------------------------------------------------------------------------
// 4-6. main
// ---------------------------------------------------------------------------

func main() {
	// 1. build
	buildOK, buildErr := checkBuild()
	buildScore := 0.0
	if buildOK {
		buildScore = 1.0
	} else {
		fmt.Printf("Build error: %s\n", buildErr)
	}

	// 2. behaviour
	passed, total := 0, 0
	behaviourScore := 0.0
	if buildOK {
		passed, total = runBehaviour()
		if total > 0 {
			behaviourScore = float64(passed) / float64(total)
		}
	}

	// 3. no_python
	noPythonScore := 0.0
	if buildOK && !hasPyFiles() && passed > 0 {
		noPythonScore = 1.0
	}

	// 4. write reward.json
	reward := (buildScore + noPythonScore + 4.0*behaviourScore) / 6.0

	os.MkdirAll("/logs/verifier", 0o755)
	result := Result{
		Reward:    reward,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}
	data, _ := json.Marshal(result)
	os.WriteFile("/logs/verifier/reward.json", data, 0o644)

	// 5. summary
	fmt.Printf("build=%.0f  no_python=%.0f  behaviour=%d/%d  reward=%.4f\n",
		buildScore, noPythonScore, passed, total, reward)

	// 6. secret
	if total > 0 && passed == total {
		revealSecret()
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}

func revealSecret() {
	raw, err := os.ReadFile("/tests/secret.b64")
	if err != nil {
		fmt.Printf("Cannot read secret: %v\n", err)
		return
	}

	// strip all whitespace
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, string(raw))

	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		fmt.Printf("Cannot decode secret: %v\n", err)
		return
	}

	cmd := exec.Command("/tmp/figlet", "-f", "slant", string(decoded))
	cmd.Dir = "/tmp"
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Cannot render secret: %v\n", err)
		return
	}

	fmt.Println("*** SECRET UNLOCKED ***")
	fmt.Print(string(out))
}
