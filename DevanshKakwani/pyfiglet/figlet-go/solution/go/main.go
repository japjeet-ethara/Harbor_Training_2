// Command figlet is a Go port of the pyfiglet command-line tool
// (https://github.com/pwaller/pyfiglet, commit 255827e). It renders text as
// FIGlet ASCII art and reproduces the original's output byte for byte.
//
// Layout:
//
//	main.go       command-line options and the CLI flow (pyfiglet.main)
//	font.go       embedded fonts and the FIGfont parser (FigletFont)
//	render.go     layout, kerning, smushing, wrapping (FigletBuilder)
//	figstring.go  -r / -F / -s / -n post-processing (FigletString)
//	pystr.go      Python-exact string helpers
package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opts, words, err := parseArgs(args)
	if err != nil {
		os.Stderr.WriteString(usageLine() + "\npyfiglet: error: " + err.Error() + "\n")
		return 2
	}
	if opts.Help {
		os.Stdout.WriteString(helpText())
		return 0
	}
	if opts.Version {
		os.Stdout.WriteString(version + "\n")
		return 0
	}
	if len(words) == 0 {
		os.Stdout.WriteString(helpText())
		return 1
	}
	text := strings.Join(words, " ")

	font, err := LoadFont(opts.Font)
	if errors.Is(err, errFontNotFound) {
		fmt.Printf("pyfiglet error: requested font %s not found.\n", pyRepr(opts.Font))
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet.FontError:", err)
		return 1
	}

	// Figlet.direction / Figlet.justify: "auto" follows the font.
	direction := opts.Direction
	if direction == "auto" {
		direction = "left-to-right"
		if font.HasPrintDir && font.PrintDirection == 1 {
			direction = "right-to-left"
		}
	}
	justify := opts.Justify
	if justify == "auto" {
		justify = "left"
		if direction == "right-to-left" {
			justify = "right"
		}
	}

	rendered, err := Render(font, text, direction, justify, opts.Width)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet:", err)
		return 1
	}

	r := []rune(rendered)
	if opts.Reverse {
		r = Reverse(r)
	}
	if opts.Flip {
		r = Flip(r)
	}
	if opts.Strip {
		r = StripSurroundingNewlines(r)
	} else if opts.Normalize {
		r = NormalizeSurroundingNewlines(r)
	}
	os.Stdout.WriteString(string(r) + "\n")
	return 0
}

// Command-line parsing compatible with the subset of Python's optparse that
// pyfiglet uses: short options (clustered "-rF", attached "-fslant" or
// separate "-f slant"), long options ("--font=slant", "--font slant",
// unambiguous prefixes such as "--fo"), "--" to end options, and options
// interspersed with the text words.

const version = "1.0.4"

type options struct {
	Font      string
	Direction string
	Justify   string
	Width     int
	Reverse   bool
	Flip      bool
	Strip     bool
	Normalize bool
	Help      bool
	Version   bool
}

type optSpec struct {
	short, long string
	takesValue  bool
	metavar     string
	help        string
	apply       func(o *options, value string) error
}

// usageError is an optparse error: the caller prints usage and exits 2.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func choice(name, value string, choices ...string) error {
	for _, c := range choices {
		if value == c {
			return nil
		}
	}
	quoted := make([]string, len(choices))
	for i, c := range choices {
		quoted[i] = pyRepr(c)
	}
	return usageError{fmt.Sprintf("option %s: invalid choice: %s (choose from %s)",
		name, pyRepr(value), strings.Join(quoted, ", "))}
}

var specs = []optSpec{
	{"-h", "--help", false, "", "show this help message and exit",
		func(o *options, _ string) error { o.Help = true; return nil }},
	{"", "--version", false, "", "show program's version number and exit",
		func(o *options, _ string) error { o.Version = true; return nil }},
	{"-f", "--font", true, "FONT", "font to render with (default: standard)",
		func(o *options, v string) error { o.Font = v; return nil }},
	{"-D", "--direction", true, "DIRECTION", "set direction text will be formatted in (default: auto)",
		func(o *options, v string) error {
			o.Direction = v
			return choice("-D/--direction", v, "auto", "left-to-right", "right-to-left")
		}},
	{"-j", "--justify", true, "SIDE", "set justification, defaults to print direction",
		func(o *options, v string) error {
			o.Justify = v
			return choice("-j/--justify", v, "auto", "left", "center", "right")
		}},
	{"-w", "--width", true, "COLS", "set terminal width for wrapping/justification (default: 80)",
		func(o *options, v string) error {
			n, err := optparseInt(v)
			if err != nil {
				return usageError{fmt.Sprintf("option -w: invalid integer value: %s", pyRepr(v))}
			}
			o.Width = n
			return nil
		}},
	{"-r", "--reverse", false, "", "shows mirror image of output text",
		func(o *options, _ string) error { o.Reverse = true; return nil }},
	{"-n", "--normalize-surrounding-newlines", false, "", "output has one empty line before and after",
		func(o *options, _ string) error { o.Normalize = true; return nil }},
	{"-s", "--strip-surrounding-newlines", false, "", "removes empty leading and trailing lines",
		func(o *options, _ string) error { o.Strip = true; return nil }},
	{"-F", "--flip", false, "", "flips rendered output text over",
		func(o *options, _ string) error { o.Flip = true; return nil }},
}

// optparseInt is optparse's "int" type: 0x.. hex, 0b.. binary, a leading 0
// means octal, otherwise decimal.
func optparseInt(v string) (int, error) {
	r := []rune(v)
	base := 10
	low := strings.ToLower(v)
	switch {
	case strings.HasPrefix(low, "0x"):
		base, r = 16, r[2:]
	case strings.HasPrefix(low, "0b"):
		base, r = 2, r[2:]
	case strings.HasPrefix(v, "0") && len(v) > 1:
		base, r = 8, r[1:]
	}
	var n int64
	var err error
	if base == 16 || base == 10 {
		n, err = pyInt(r, base)
	} else {
		n, err = pyIntBase(r, base)
	}
	return int(n), err
}

func pyIntBase(r []rune, base int) (int64, error) {
	t := pyStrip(r)
	if len(t) == 0 {
		return 0, fmt.Errorf("invalid")
	}
	var n int64
	for _, c := range t {
		d := int(c - '0')
		if d < 0 || d >= base {
			return 0, fmt.Errorf("invalid")
		}
		n = n*int64(base) + int64(d)
	}
	return n, nil
}

func findShort(s string) *optSpec {
	for i := range specs {
		if specs[i].short == s {
			return &specs[i]
		}
	}
	return nil
}

func findLong(s string) (*optSpec, error) {
	var matches []*optSpec
	for i := range specs {
		if specs[i].long == s {
			return &specs[i], nil
		}
		if strings.HasPrefix(specs[i].long, s) {
			matches = append(matches, &specs[i])
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return nil, usageError{"no such option: " + s}
	}
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.long
	}
	sort.Strings(names)
	return nil, usageError{fmt.Sprintf("ambiguous option: %s (%s?)", s, strings.Join(names, ", "))}
}

// parseArgs returns the options and the positional (text) arguments.
// It stops early when --help or --version is seen, as optparse does.
func parseArgs(args []string) (*options, []string, error) {
	o := &options{Font: "standard", Direction: "auto", Justify: "auto", Width: 80}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return o, append(positional, args[i+1:]...), nil

		case strings.HasPrefix(arg, "--"):
			name, value, hasValue := strings.Cut(arg, "=")
			spec, err := findLong(name)
			if err != nil {
				return nil, nil, err
			}
			if spec.takesValue {
				if !hasValue {
					if i+1 >= len(args) {
						return nil, nil, usageError{spec.long + " option requires 1 argument"}
					}
					i++
					value = args[i]
				}
			} else if hasValue {
				return nil, nil, usageError{spec.long + " option does not take a value"}
			}
			if err := spec.apply(o, value); err != nil {
				return nil, nil, err
			}

		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			rest := []rune(arg[1:])
			for j := 0; j < len(rest); j++ {
				name := "-" + string(rest[j])
				spec := findShort(name)
				if spec == nil {
					return nil, nil, usageError{"no such option: " + name}
				}
				value := ""
				if spec.takesValue {
					if j+1 < len(rest) {
						value = string(rest[j+1:])
					} else if i+1 < len(args) {
						i++
						value = args[i]
					} else {
						return nil, nil, usageError{name + " option requires 1 argument"}
					}
					j = len(rest)
				}
				if err := spec.apply(o, value); err != nil {
					return nil, nil, err
				}
			}

		default:
			positional = append(positional, arg)
		}
		if o.Help || o.Version {
			return o, positional, nil
		}
	}
	return o, positional, nil
}

func usageLine() string { return "Usage: pyfiglet [options] [text..]\n" }

func helpText() string {
	var b strings.Builder
	b.WriteString(usageLine())
	b.WriteString("\nOptions:\n")
	for _, s := range specs {
		var names []string
		if s.short != "" {
			n := s.short
			if s.takesValue {
				n += " " + s.metavar
			}
			names = append(names, n)
		}
		n := s.long
		if s.takesValue {
			n += "=" + s.metavar
		}
		names = append(names, n)
		fmt.Fprintf(&b, "  %-36s %s\n", strings.Join(names, ", "), s.help)
	}
	return b.String()
}
