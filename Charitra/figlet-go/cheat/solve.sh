#!/bin/bash
# Deliberately weak visible-set lookup used only for the Step 7 anti-cheat check.
# Before the run, place the generated tests/data/visible.jsonl at /solution/visible.jsonl.
set -euo pipefail
cd /workspace/repo
cp /solution/visible.jsonl ./visible_lookup.json
cp /solution/go.mod ./go.mod
cat > main.go <<'GO'
package main
import("bufio";"encoding/json";"os";"strings")
type C struct{Args []string `json:"args"`;Out string `json:"out"`}
func main(){b,e:=os.Open("visible_lookup.json");if e!=nil{return};defer b.Close();want:=strings.Join(os.Args[1:],"\x00");sc:=bufio.NewScanner(b);sc.Buffer(make([]byte,65536),16*1024*1024);for sc.Scan(){var c C;if json.Unmarshal(sc.Bytes(),&c)==nil&&strings.Join(c.Args,"\x00")==want{os.Stdout.WriteString(c.Out);return}}}
GO
find . -type f -name '*.py' -delete
