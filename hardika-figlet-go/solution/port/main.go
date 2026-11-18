package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
)

type options struct {
	font                       string
	width                      int
	direction, justify         string
	reverse, flip, strip, norm bool
}

var longOptions = map[string]byte{
	"font": 'f', "width": 'w', "direction": 'D', "justify": 'j',
	"reverse": 'r', "flip": 'F', "strip-surrounding-newlines": 's', "normalize-surrounding-newlines": 'n',
}

func takesValue(option byte) bool {
	return strings.ContainsRune("fwDj", rune(option))
}

func parseOptions(args []string) (options, string, error) {
	opts := options{font: "standard", width: 80, direction: "auto", justify: "auto"}
	var words []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			words = append(words, args[index+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			words = append(words, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			name, value, provided := strings.Cut(arg[2:], "=")
			option := longOptions[name]
			if option == 0 || provided && !takesValue(option) {
				return opts, "", fmt.Errorf("invalid option %s", arg)
			}
			if !provided && takesValue(option) {
				index++
				if index == len(args) {
					return opts, "", fmt.Errorf("%s needs a value", arg)
				}
				value = args[index]
			}
			if err := opts.set(option, value); err != nil {
				return opts, "", err
			}
			continue
		}

		short := arg[1:]
		for len(short) > 0 {
			option := short[0]
			short = short[1:]
			value := ""
			if takesValue(option) {
				value, short = short, ""
				if value == "" {
					index++
					if index == len(args) {
						return opts, "", fmt.Errorf("-%c needs a value", option)
					}
					value = args[index]
				}
			}
			if err := opts.set(option, value); err != nil {
				return opts, "", err
			}
		}
	}
	if len(words) == 0 {
		return opts, "", fmt.Errorf("provide text to render")
	}
	return opts, strings.Join(words, " "), nil
}

func (o *options) set(option byte, value string) error {
	switch option {
	case 'f':
		o.font = value
	case 'w':
		lower := strings.ToLower(value)
		radix := 10
		if strings.HasPrefix(lower, "0") {
			radix = 0
		}
		if lower == "0b" {
			value = "0"
		}
		width, err := strconv.ParseInt(strings.TrimFunc(value, isPythonSpace), radix, strconv.IntSize)
		if err != nil {
			return err
		}
		o.width = int(width)
	case 'j':
		if !slices.Contains([]string{"auto", "left", "center", "right"}, value) {
			return fmt.Errorf("invalid justification %q", value)
		}
		o.justify = value
	case 'D':
		if !slices.Contains([]string{"auto", "left-to-right", "right-to-left"}, value) {
			return fmt.Errorf("invalid direction %q", value)
		}
		o.direction = value
	case 'r':
		o.reverse = true
	case 'F':
		o.flip = true
	case 's':
		o.strip = true
	case 'n':
		o.norm = true
	default:
		return fmt.Errorf("unknown option -%c", option)
	}
	return nil
}

var mirrorChars = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<',
	'[': ']', ']': '[', '{': '}', '}': '{',
}

var flipChars = map[rune]rune{
	'/': '\\', '\\': '/', 'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M',
	'P': 'b', 'R': 'b', 'b': 'P', '^': 'v', 'v': '^', '_': '-', 'm': 'w', 'w': 'm',
}

func transform(text string, opts options) string {
	if opts.reverse {
		rows := splitLines(text)
		for index, row := range rows {
			chars := []rune(row)
			for i, ch := range chars {
				if replacement, ok := mirrorChars[ch]; ok {
					chars[i] = replacement
				}
			}
			slices.Reverse(chars)
			rows[index] = string(chars)
		}
		text = strings.Join(rows, "\n") + "\n"
	}
	if opts.flip {
		rows := splitLines(text)
		slices.Reverse(rows)
		for index, row := range rows {
			rows[index] = strings.Map(func(ch rune) rune {
				if replacement, ok := flipChars[ch]; ok {
					return replacement
				}
				return ch
			}, row)
		}
		text = strings.Join(rows, "\n") + "\n"
	}
	if opts.strip || opts.norm {
		rows := splitLines(text)
		index := 0
		for index < len(rows) && strings.TrimFunc(rows[index], isPythonSpace) == "" {
			index++
		}
		text = strings.TrimRightFunc(strings.Join(rows[index:], "\n"), isPythonSpace)
		if !opts.strip {
			text = "\n" + text + "\n"
		}
	}
	return text + "\n"
}

func run(args []string, out io.Writer) error {
	opts, text, err := parseOptions(args)
	if err != nil {
		return err
	}
	font, err := loadFont(opts.font)
	if err != nil {
		return err
	}
	plan, err := planRendering(font, text, opts)
	if err != nil {
		return err
	}
	rendered, err := plan.serialize()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, transform(rendered, opts))
	return err
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
