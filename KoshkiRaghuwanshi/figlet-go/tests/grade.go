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

func main() {
    repoDir := "/workspace/repo"
    binPath := "/tmp/figlet"

    buildScore := 0.0
    buildCmd := exec.Command("go", "build", "-o", binPath, ".")
    buildCmd.Dir = repoDir
    if out, err := buildCmd.CombinedOutput(); err == nil {
        buildScore = 1.0
    } else {
        fmt.Printf("Build failed: %v\n%s\n", err, string(out))
    }

    hasPy := false
    _ = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
        if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
            hasPy = true
        }
        return nil
    })

    behaviourScore := 0.0
    matchedAny := false
    total := 0
    passed := 0

    if buildScore == 1.0 {
        f, err := os.Open("/tests/data/hidden.jsonl")
        if err != nil {
            fmt.Printf("Failed to open hidden.jsonl: %v\n", err)
        } else {
            defer f.Close()
            scanner := bufio.NewScanner(f)
            scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
            for scanner.Scan() {
                line := scanner.Bytes()
                if len(bytes.TrimSpace(line)) == 0 {
                    continue
                }
                var tc TestCase
                if err := json.Unmarshal(line, &tc); err != nil {
                    continue
                }
                total++
                ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
                cmd := exec.CommandContext(ctx, binPath, tc.Args...)
                var stdout bytes.Buffer
                cmd.Stdout = &stdout
                err := cmd.Run()
                cancel()
                if err == nil && stdout.String() == tc.Out {
                    passed++
                    matchedAny = true
                }
            }
            if total > 0 {
                behaviourScore = float64(passed) / float64(total)
            }
        }
    }

    noPythonScore := 0.0
    if buildScore == 1.0 && !hasPy && matchedAny {
        noPythonScore = 1.0
    }

    weightedMean := (1.0*buildScore + 1.0*noPythonScore + 4.0*behaviourScore) / 6.0

    _ = os.MkdirAll("/logs/verifier", 0755)
    rew := Reward{
        Reward:    weightedMean,
        Build:     buildScore,
        NoPython:  noPythonScore,
        Behaviour: behaviourScore,
    }
    rewBytes, _ := json.MarshalIndent(rew, "", "  ")
    _ = os.WriteFile("/logs/verifier/reward.json", append(rewBytes, '\n'), 0644)

    fmt.Printf("Passed %d/%d hidden cases\n", passed, total)
    fmt.Printf("Reward: %.6f (build=%.1f, no_python=%.1f, behaviour=%.6f)\n",
        weightedMean, buildScore, noPythonScore, behaviourScore)

    if behaviourScore == 1.0 && total > 0 {
        b64Bytes, err := os.ReadFile("/tests/secret.b64")
        if err == nil {
            decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b64Bytes)))
            if err == nil {
                fmt.Println("*** SECRET UNLOCKED ***")
                cmd := exec.Command(binPath, "-f", "slant", string(decoded))
                if out, err := cmd.Output(); err == nil {
                    fmt.Print(string(out))
                }
            }
        }
    } else {
        fmt.Println("No secret for you. Yet.")
    }
}
