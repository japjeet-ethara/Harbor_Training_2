// diffcases: run a figlet binary on a .jsonl case file and show the first cases that differ.
//
//	go run ./tools/diffcases BINARY tests/data/visible.jsonl [N]
//
// Use visible.jsonl while developing; hidden.jsonl is for the grader only.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: diffcases BINARY CASES.jsonl [N]")
		os.Exit(2)
	}
	bin, path, show := os.Args[1], os.Args[2], 5
	if len(os.Args) > 3 {
		show, _ = strconv.Atoi(os.Args[3])
	}
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	dec := json.NewDecoder(bufio.NewReader(f))
	total, bad := 0, 0
	for dec.More() {
		var c struct {
			Args []string `json:"args"`
			Out  string   `json:"out"`
		}
		if err := dec.Decode(&c); err != nil {
			panic(err)
		}
		total++
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, bin, c.Args...)
		cmd.Dir = os.TempDir()
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		cmd.Run()
		timedOut := ctx.Err() != nil
		cancel()
		if !timedOut && out.String() == c.Out {
			continue
		}
		bad++
		if bad <= show {
			q, _ := json.Marshal(c.Args)
			fmt.Printf("=== case %d  args=%s%s\n", total, q, map[bool]string{true: "  (TIMEOUT)"}[timedOut])
			fmt.Printf("--- expected:\n%s", visible(c.Out))
			fmt.Printf("--- actual:\n%s", visible(out.String()))
			if errb.Len() > 0 {
				fmt.Printf("--- stderr: %s", errb.String())
			}
		}
	}
	fmt.Printf("\n%d / %d cases differ\n", bad, total)
}

// visible marks line ends with $ so trailing spaces show.
func visible(s string) string {
	return strings.ReplaceAll(s, "\n", "$\n")
}
