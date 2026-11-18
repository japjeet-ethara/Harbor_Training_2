package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type tc struct {
	Args []string `json:"args"`
	Out  string   `json:"out"`
}

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	var cases []tc
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var c tc
		json.Unmarshal([]byte(sc.Text()), &c)
		cases = append(cases, c)
	}
	actual := make([]string, len(cases))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range cases {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "/tmp/figlet", cases[i].Args...)
			cmd.Dir = "/tmp"
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Run()
			actual[i] = out.String()
		}(i)
	}
	wg.Wait()
	bad := 0
	for i, c := range cases {
		if actual[i] == c.Out {
			continue
		}
		bad++
		if bad <= 5 {
			fmt.Printf("---- DIFF #%d  args=%q\n", bad, c.Args)
			fmt.Printf("EXPECTED:\n%s<END>\nACTUAL:\n%s<END>\n", c.Out, actual[i])
		}
	}
	fmt.Printf("\nRESULT: %d of %d cases differ\n", bad, len(cases))
}
