package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	fontName := "standard"
	width := 80
	justify := "auto"
	direction := "auto"
	reverse := false
	flip := false
	strip := false
	normalize := false

	var textArgs []string
	args := os.Args[1:]
	i := 0

	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			textArgs = append(textArgs, args[i+1:]...)
			break
		}

		if arg == "-f" && i+1 < len(args) {
			fontName = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "-f=") {
			fontName = strings.TrimPrefix(arg, "-f=")
			i++
		} else if arg == "-w" && i+1 < len(args) {
			if val, err := strconv.Atoi(args[i+1]); err == nil {
				width = val
			}
			i += 2
		} else if strings.HasPrefix(arg, "-w=") {
			if val, err := strconv.Atoi(strings.TrimPrefix(arg, "-w=")); err == nil {
				width = val
			}
			i++
		} else if arg == "-j" && i+1 < len(args) {
			justify = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "-j=") {
			justify = strings.TrimPrefix(arg, "-j=")
			i++
		} else if arg == "-D" && i+1 < len(args) {
			direction = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "-D=") {
			direction = strings.TrimPrefix(arg, "-D=")
			i++
		} else if arg == "-r" {
			reverse = true
			i++
		} else if arg == "-F" {
			flip = true
			i++
		} else if arg == "-s" {
			strip = true
			i++
		} else if arg == "-n" {
			normalize = true
			i++
		} else if strings.HasPrefix(arg, "-") && len(arg) > 2 && isCombinedFlags(arg[1:]) {
			for _, ch := range arg[1:] {
				switch ch {
				case 'r':
					reverse = true
				case 'F':
					flip = true
				case 's':
					strip = true
				case 'n':
					normalize = true
				}
			}
			i++
		} else {
			textArgs = append(textArgs, arg)
			i++
		}
	}

	if len(textArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [text..]\n", os.Args[0])
		os.Exit(1)
	}

	text := strings.Join(textArgs, " ")

	font, err := LoadFont(fontName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: requested font %q not found.\n", fontName)
		os.Exit(1)
	}

	output, err := Render(font, text, direction, justify, width)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if reverse {
		output = ReverseFiglet(output)
	}
	if flip {
		output = FlipFiglet(output)
	}

	var finalStr string
	if strip {
		finalStr = StripSurroundingNewlines(output) + "\n"
	} else if normalize {
		finalStr = NormalizeSurroundingNewlines(output) + "\n"
	} else {
		finalStr = output + "\n"
	}

	os.Stdout.WriteString(finalStr)
}

func isCombinedFlags(s string) bool {
	for _, ch := range s {
		if ch != 'r' && ch != 'F' && ch != 's' && ch != 'n' {
			return false
		}
	}
	return true
}
