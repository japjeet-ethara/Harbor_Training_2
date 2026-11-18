package main

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

type options struct {
	font, direction, justify        string
	width                           int
	reverse, flip, strip, normalize bool
}

func parseOptions(args []string) (options, string, error) {
	opts := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var text []string
	long := map[string]string{
		"font": "f", "direction": "D", "justify": "j", "width": "w",
		"reverse": "r", "flip": "F", "strip-surrounding-newlines": "s",
		"normalize-surrounding-newlines": "n",
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			text = append(text, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			text = append(text, arg)
			continue
		}
		sequence := arg[1:]
		value, inline := "", false
		if strings.HasPrefix(arg, "--") {
			key, val, found := strings.Cut(arg[2:], "=")
			sequence, value, inline = long[key], val, found
			if sequence == "" {
				return opts, "", fmt.Errorf("unknown option %s", arg)
			}
		}
		for index := 0; index < len(sequence); index++ {
			option := sequence[index]
			switch option {
			case 'r':
				opts.reverse = true
			case 'F':
				opts.flip = true
			case 's':
				opts.strip = true
			case 'n':
				opts.normalize = true
			case 'f', 'D', 'j', 'w':
				switch {
				case inline:
				case index+1 < len(sequence):
					value = sequence[index+1:]
				case i+1 < len(args):
					i++
					value = args[i]
				default:
					return opts, "", fmt.Errorf("missing value for -%c", option)
				}
				if err := opts.setValue(option, value); err != nil {
					return opts, "", err
				}
				index = len(sequence)
			default:
				return opts, "", fmt.Errorf("unknown option -%c", option)
			}
		}
	}
	if len(text) == 0 {
		return opts, "", fmt.Errorf("missing input text")
	}
	return opts, strings.Join(text, " "), nil
}

func (o *options) setValue(option byte, value string) error {
	switch option {
	case 'f':
		o.font = value
	case 'D':
		if !slices.Contains([]string{"auto", "left-to-right", "right-to-left"}, value) {
			return fmt.Errorf("invalid direction")
		}
		o.direction = value
	case 'j':
		if !slices.Contains([]string{"auto", "left", "center", "right"}, value) {
			return fmt.Errorf("invalid justification")
		}
		o.justify = value
	case 'w':
		width, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		o.width = width
	}
	return nil
}

func translate(text string, from, to string) string {
	return strings.Map(func(ch rune) rune {
		if index := strings.IndexRune(from, ch); index >= 0 {
			return rune(to[index])
		}
		return ch
	}, text)
}

func finish(text string, opts options) string {
	if opts.reverse {
		rows := splitLines(text)
		for i, row := range rows {
			cells := []rune(translate(row, "()<>[]{}\\/", ")(><][}{/\\"))
			slices.Reverse(cells)
			rows[i] = string(cells)
		}
		text = strings.Join(rows, "\n") + "\n"
	}
	if opts.flip {
		rows := splitLines(text)
		slices.Reverse(rows)
		for i, row := range rows {
			rows[i] = translate(row, "/\\AMPRVW^_bmvw", "\\/VWbbAMv-Pw^m")
		}
		text = strings.Join(rows, "\n") + "\n"
	}
	if opts.strip || opts.normalize {
		rows := splitLines(text)
		for len(rows) > 0 && strings.TrimFunc(rows[0], pythonSpace) == "" {
			rows = rows[1:]
		}
		text = strings.TrimRightFunc(strings.Join(rows, "\n"), pythonSpace)
		if opts.normalize && !opts.strip {
			text = "\n" + text + "\n"
		}
	}
	return text + "\n"
}

func run(args []string) (string, error) {
	opts, text, err := parseOptions(args)
	if err != nil {
		return "", err
	}
	font, err := loadFont(opts.font)
	if err != nil {
		return "", err
	}
	rendered, err := render(text, font, opts)
	if err != nil {
		return "", err
	}
	return finish(rendered, opts), nil
}

func main() {
	output, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(output)
}
