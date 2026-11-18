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

type TestCase struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

type Reward struct {
	Reward    float64 `json:"reward"`
	Build     float64 `json:"build"`
	NoPython  float64 `json:"no_python"`
	Behaviour float64 `json:"behaviour"`
}

func checkBuild() bool {
	cmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
	cmd.Dir = "/workspace/repo"
	return cmd.Run() == nil
}

func hasPythonFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".py") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func runFiglet(args []string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/tmp/figlet", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return out.String(), true
}

func main() {
	buildScore := 0.0
	noPythonScore := 0.0
	behaviourScore := 0.0

	built := checkBuild()
	if built {
		buildScore = 1.0
	}

	var cases []TestCase
	f, err := os.Open("/tests/data/hidden.jsonl")
	if err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			var tc TestCase
			if json.Unmarshal(scanner.Bytes(), &tc) == nil {
				cases = append(cases, tc)
			}
		}
		f.Close()
	}

	matched := 0
	if built && len(cases) > 0 {
		for _, tc := range cases {
			got, ok := runFiglet(tc.Args)
			if ok && got == tc.Out {
				matched++
			}
		}
		behaviourScore = float64(matched) / float64(len(cases))
	}

	if built && !hasPythonFiles("/workspace/repo") && matched > 0 {
		noPythonScore = 1.0
	}

	weighted := (1.0*buildScore + 1.0*noPythonScore + 4.0*behaviourScore) / 6.0

	_ = os.MkdirAll("/logs/verifier", 0755)
	res := Reward{
		Reward:    weighted,
		Build:     buildScore,
		NoPython:  noPythonScore,
		Behaviour: behaviourScore,
	}
	data, _ := json.MarshalIndent(res, "", "  ")
	_ = os.WriteFile("/logs/verifier/reward.json", data, 0644)

	if built && behaviourScore == 1.0 {
		b64Bytes, err := os.ReadFile("/tests/secret.b64")
		if err == nil {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b64Bytes)))
			if err == nil {
				banner, ok := runFiglet([]string{"-f", "slant", string(decoded)})
				if ok {
					fmt.Println("*** SECRET UNLOCKED ***")
					fmt.Print(banner)
				}
			}
		}
	} else {
		fmt.Println("No secret for you. Yet.")
	}
}