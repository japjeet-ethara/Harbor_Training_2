package main

import (
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io/fs"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

type caseRow struct {
    Args []string `json:"args"`
    Out  string   `json:"out"`
}

type reward struct {
    Reward     float64 `json:"reward"`
    Build      int     `json:"build"`
    NoPython   int     `json:"no_python"`
    Behaviour  float64 `json:"behaviour"`
}

func main() {
    repo := "/workspace/repo"
    secretPath := "/tests/secret.b64"

    buildOK := 0
    cmd := exec.Command("go", "build", "-o", "/tmp/figlet", ".")
    cmd.Dir = repo
    cmd.Env = os.Environ()
    if out, err := cmd.CombinedOutput(); err != nil {
        fmt.Fprintf(os.Stderr, "build failed: %v\n%s\n", err, out)
    } else {
        buildOK = 1
    }

    noPythonOK := 0
    if buildOK == 1 {
        foundPy := false
        filepath.WalkDir(repo, func(path string, d fs.DirEntry, err error) error {
            if err != nil { return nil }
            if d.IsDir() { return nil }
            if strings.HasSuffix(strings.ToLower(d.Name()), ".py") {
                foundPy = true
            }
            return nil
        })
        if !foundPy {
            noPythonOK = 1
        }
    }

    behaviourScore := 0.0
    hiddenPath := "/tests/data/hidden.jsonl"
    if data, err := os.ReadFile(hiddenPath); err == nil {
        lines := strings.Split(strings.TrimSpace(string(data)), "\n")
        total := 0
        ok := 0
        for _, line := range lines {
            line = strings.TrimSpace(line)
            if line == "" { continue }
            total++
            var row caseRow
            if err := json.Unmarshal([]byte(line), &row); err != nil {
                continue
            }
            if len(row.Args) == 0 {
                continue
            }
            res, err := exec.Command("/tmp/figlet", row.Args...).CombinedOutput()
            if err == nil {
                if string(res) == row.Out {
                    ok++
                }
            }
        }
        if total > 0 {
            behaviourScore = float64(ok) / float64(total)
        }
    }

    weighted := (float64(buildOK) + float64(noPythonOK) + behaviourScore*4.0) / 6.0
    r := reward{
        Reward:    weighted,
        Build:     buildOK,
        NoPython:  noPythonOK,
        Behaviour: behaviourScore,
    }

    outBytes, _ := json.MarshalIndent(r, "", "  ")
    if err := os.MkdirAll("/logs/verifier", 0o755); err != nil {
        panic(err)
    }
    if err := os.WriteFile("/logs/verifier/reward.json", outBytes, 0o644); err != nil {
        panic(err)
    }

    if behaviourScore == 1.0 {
        data, err := os.ReadFile(secretPath)
        if err == nil {
            dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
            if err == nil {
                secret := string(dec)
                fmt.Println("*** SECRET UNLOCKED ***")
                cmd := exec.Command("/tmp/figlet", "-f", "slant", secret)
                if out, err := cmd.CombinedOutput(); err == nil {
                    fmt.Print(string(out))
                }
            }
        }
    } else {
        fmt.Println("No secret for you. Yet.")
    }
}

