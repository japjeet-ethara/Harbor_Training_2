package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	opts, input, err := parseArgs(args)
	if err != nil {
		return err
	}
	font, err := loadFont(opts.font)
	if err != nil {
		return err
	}
	output, err := render(font, opts, input)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, transform(output, opts))
	return err
}
