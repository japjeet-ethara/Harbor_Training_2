// Author tool (not part of grading): run /tmp/figlet on a .jsonl case file and show the first
// cases whose output differs, with args, expected and actual output.
//
//	go run tools/diffcases.go tests/data/visible.jsonl 5
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

func main() {
	path, limit := "tests/data/visible.jsonl", 5
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if len(os.Args) > 2 {
		limit, _ = strconv.Atoi(os.Args[2])
	}
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	total, bad := 0, 0
	for sc.Scan() {
		var c struct {
			Args []string `json:"args"`
			Out  string   `json:"out"`
		}
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			panic(err)
		}
		total++
		var out bytes.Buffer
		cmd := exec.Command("/tmp/figlet", c.Args...)
		cmd.Stdout = &out
		_ = cmd.Run()
		if out.String() == c.Out {
			continue
		}
		bad++
		if bad <= limit {
			a, _ := json.Marshal(c.Args)
			fmt.Printf("=== case %d args %s\n--- expected\n%q\n--- actual\n%q\n", total, a, c.Out, out.String())
		}
	}
	fmt.Printf("%d of %d cases differ\n", bad, total)
}
