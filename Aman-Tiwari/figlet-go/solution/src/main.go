// figlet: a Go port of pyfiglet (commit 255827efbc09d662875dad50fa0edc1e1263bf45).
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const usage = `Usage: figlet [options] [text..]

Options:
  -f FONT, --font=FONT  font to render with (default: standard)
  -D DIRECTION, --direction=DIRECTION
                        set direction text will be formatted in (default: auto)
  -j SIDE, --justify=SIDE
                        set justification, defaults to print direction
  -w COLS, --width=COLS
                        set terminal width for wrapping/justification (default: 80)
  -r, --reverse         shows mirror image of output text
  -n, --normalize-surrounding-newlines
                        output has one empty line before and after
  -s, --strip-surrounding-newlines
                        removes empty leading and trailing lines
  -F, --flip            flips rendered output text over
`

type options struct {
	font, direction, justify string
	width                    int
	reverse, flip            bool
	strip, normalize         bool
	text                     []string
}

type optSpec struct {
	short, long string
	takesValue  bool
}

var specs = []optSpec{
	{"f", "font", true}, {"D", "direction", true}, {"j", "justify", true},
	{"w", "width", true}, {"r", "reverse", false},
	{"n", "normalize-surrounding-newlines", false},
	{"s", "strip-surrounding-newlines", false}, {"F", "flip", false},
	{"l", "list_fonts", false}, {"i", "info_font", false},
	{"L", "load", true}, {"c", "color", true},
	{"h", "help", false}, {"", "version", false},
}

func usageError(msg string) {
	fmt.Fprint(os.Stderr, strings.SplitN(usage, "\n", 2)[0]+"\n\n")
	fmt.Fprintf(os.Stderr, "figlet: error: %s\n", msg)
	os.Exit(2)
}

// parseInt mirrors optparse's _parse_num(val, int).
func parseInt(val string) (int, error) {
	radix := 10
	v := val
	switch {
	case len(v) >= 2 && strings.ToLower(v[:2]) == "0x":
		radix, v = 16, v[2:]
	case len(v) >= 2 && strings.ToLower(v[:2]) == "0b":
		radix, v = 2, v[2:]
		if v == "" {
			v = "0"
		}
	case strings.HasPrefix(v, "0"):
		radix = 8
		if len(v) >= 2 && (v[1] == 'o' || v[1] == 'O') {
			v = v[2:]
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), radix, 64)
	return int(n), err
}

func (o *options) set(s optSpec, value string) {
	switch s.short {
	case "f":
		o.font = value
	case "D":
		if value != "auto" && value != "left-to-right" && value != "right-to-left" {
			usageError(fmt.Sprintf("option -D: invalid choice: '%s' (choose from 'auto', 'left-to-right', 'right-to-left')", value))
		}
		o.direction = value
	case "j":
		if value != "auto" && value != "left" && value != "center" && value != "right" {
			usageError(fmt.Sprintf("option -j: invalid choice: '%s' (choose from 'auto', 'left', 'center', 'right')", value))
		}
		o.justify = value
	case "w":
		n, err := parseInt(value)
		if err != nil {
			usageError(fmt.Sprintf("option -w: invalid integer value: '%s'", value))
		}
		o.width = n
	case "r":
		o.reverse = true
	case "n":
		o.normalize = true
	case "s":
		o.strip = true
	case "F":
		o.flip = true
	case "h":
		fmt.Print(usage)
		os.Exit(0)
	case "l", "i", "L", "c":
		// Out of scope for this port (font list/info/install, colours): accepted, ignored.
	default: // --version
		fmt.Println("1.0.4")
		os.Exit(0)
	}
}

// parseArgs mirrors optparse: interspersed arguments, clustered short options,
// attached values, "--" terminator and unambiguous long-option prefixes.
func parseArgs(args []string) options {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			o.text = append(o.text, args[i+1:]...)
			return o
		case strings.HasPrefix(a, "--"):
			name, value, hasValue := strings.Cut(a[2:], "=")
			var match *optSpec
			for k := range specs {
				if specs[k].long == name {
					match = &specs[k]
					break
				}
			}
			if match == nil {
				var cands []*optSpec
				for k := range specs {
					if specs[k].long != "" && strings.HasPrefix(specs[k].long, name) {
						cands = append(cands, &specs[k])
					}
				}
				if len(cands) != 1 {
					usageError("no such option: --" + name)
				}
				match = cands[0]
			}
			if match.takesValue {
				if !hasValue {
					if i+1 >= len(args) {
						usageError("--" + match.long + " option requires 1 argument")
					}
					i++
					value = args[i]
				}
				o.set(*match, value)
			} else {
				if hasValue {
					usageError("--" + match.long + " option does not take a value")
				}
				o.set(*match, "")
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			chars := []rune(a[1:])
			for j := 0; j < len(chars); j++ {
				var match *optSpec
				for k := range specs {
					if specs[k].short == string(chars[j]) {
						match = &specs[k]
					}
				}
				if match == nil {
					usageError("no such option: -" + string(chars[j]))
				}
				if match.takesValue {
					value := string(chars[j+1:])
					if value == "" {
						if i+1 >= len(args) {
							usageError("-" + match.short + " option requires 1 argument")
						}
						i++
						value = args[i]
					}
					o.set(*match, value)
					break
				}
				o.set(*match, "")
			}
		default:
			o.text = append(o.text, a)
		}
	}
	return o
}

// pyRepr is a small approximation of Python's repr() for a str.
func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "'", `\'`) + "'"
}

func main() {
	o := parseArgs(os.Args[1:])
	if len(o.text) == 0 {
		fmt.Print(usage)
		os.Exit(1)
	}
	text := strings.Join(o.text, " ")

	font, err := loadFont(o.font)
	if err == errFontNotFound {
		fmt.Printf("pyfiglet error: requested font %s not found.\n", pyRepr(o.font))
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet.FontError:", err)
		os.Exit(1)
	}

	direction := o.direction
	if direction == "auto" {
		direction = "left-to-right"
		if font.printDirection != nil && *font.printDirection == 1 {
			direction = "right-to-left"
		}
	}
	justify := o.justify
	if justify == "auto" {
		justify = "left"
		if direction == "right-to-left" {
			justify = "right"
		}
	}

	r, err := render(text, font, direction, justify, o.width)
	if err != nil {
		fmt.Fprintln(os.Stderr, "figlet:", err)
		os.Exit(1)
	}
	if o.reverse {
		r = reverseString(r)
	}
	if o.flip {
		r = flipString(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = "\n" + stripSurroundingNewlines(r) + "\n"
	}
	os.Stdout.WriteString(r + "\n")
}
