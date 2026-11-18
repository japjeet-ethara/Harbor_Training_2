// check_visible.go - author-side debugging aid (NOT used by the grader, never shown to the agent).
// Runs a built binary over a .jsonl case file and prints the first N cases that differ.
//
//	go run dev/check_visible.go [-bin /tmp/figlet] [-cases tests/data/visible.jsonl] [-n 5]
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type tc struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	bin := flag.String("bin", "/tmp/figlet", "binary to test")
	file := flag.String("cases", "tests/data/visible.jsonl", "cases file")
	show := flag.Int("n", 5, "how many differing cases to show")
	flag.Parse()

	f, err := os.Open(*file)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	total, bad, shown := 0, 0, 0
	for sc.Scan() {
		var c tc
		if json.Unmarshal(sc.Bytes(), &c) != nil {
			continue
		}
		total++
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var out bytes.Buffer
		cmd := exec.CommandContext(ctx, *bin, c.Args...)
		cmd.Stdout = &out
		cmd.Run()
		cancel()
		if out.String() == c.Out {
			continue
		}
		bad++
		if shown < *show {
			shown++
			fmt.Printf("---- case #%d  args=%q\n", total, c.Args)
			exp, act := strings.Split(c.Out, "\n"), strings.Split(out.String(), "\n")
			fmt.Println("expected:")
			for _, l := range exp {
				fmt.Printf("  |%s|\n", l)
			}
			fmt.Println("actual:")
			for _, l := range act {
				fmt.Printf("  |%s|\n", l)
			}
			for i := 0; i < len(exp) || i < len(act); i++ {
				var e, a string
				if i < len(exp) {
					e = exp[i]
				}
				if i < len(act) {
					a = act[i]
				}
				if e != a {
					fmt.Printf("first differing line %d:\n  want %q\n  got  %q\n", i+1, e, a)
					break
				}
			}
		}
	}
	fmt.Printf("\n%d / %d cases differ\n", bad, total)
	if bad > 0 {
		os.Exit(1)
	}
}
