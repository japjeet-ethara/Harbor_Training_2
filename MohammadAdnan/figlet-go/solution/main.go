package main

import (
	"fmt"
	"os"
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
}

var longNames = map[string]string{
	"font": "f", "direction": "D", "justify": "j", "width": "w", "reverse": "r",
	"normalize-surrounding-newlines": "n", "strip-surrounding-newlines": "s", "flip": "F",
	"list_fonts": "l", "info_font": "i", "load": "L", "color": "c", "help": "h",
}

func takesValue(o string) bool { return strings.Contains("fDjwLc", o) }

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "Usage: figlet [options] [text..]\n\nfiglet: error: %s\n", msg)
	os.Exit(2)
}

func setOpt(o *options, name, val string) {
	switch name {
	case "f":
		o.font = val
	case "D":
		if val != "auto" && val != "left-to-right" && val != "right-to-left" {
			fail("option -D: invalid choice: '" + val + "'")
		}
		o.direction = val
	case "j":
		if val != "auto" && val != "left" && val != "center" && val != "right" {
			fail("option -j: invalid choice: '" + val + "'")
		}
		o.justify = val
	case "w":
		n, err := pyInt(val, 10)
		if err != nil {
			fail("option -w: invalid integer value: '" + val + "'")
		}
		o.width = n
	case "r":
		o.reverse = true
	case "F":
		o.flip = true
	case "s":
		o.strip = true
	case "n":
		o.normalize = true
	case "h":
		fmt.Print(usage)
		os.Exit(0)
	case "l", "i", "L", "c":
		// Out of scope for this port; accepted and ignored.
	}
}

// parseArgs follows optparse: interspersed options, clustered short flags, "--" ends options.
func parseArgs(argv []string) (options, []string) {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var rest []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			rest = append(rest, argv[i+1:]...)
			return o, rest
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a[2:], "=")
			short, ok := longNames[name]
			if !ok {
				var matches []string
				for long := range longNames {
					if strings.HasPrefix(long, name) {
						matches = append(matches, long)
					}
				}
				if len(matches) != 1 {
					fail("no such option: --" + name)
				}
				short = longNames[matches[0]]
			}
			if takesValue(short) {
				if !hasVal {
					if i+1 >= len(argv) {
						fail("--" + name + " option requires an argument")
					}
					i++
					val = argv[i]
				}
				setOpt(&o, short, val)
			} else {
				setOpt(&o, short, "")
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			flags := a[1:]
			for j := 0; j < len(flags); j++ {
				c := string(flags[j])
				if !strings.Contains("fDjwrnsFliLch", c) {
					fail("no such option: -" + c)
				}
				if takesValue(c) {
					val := flags[j+1:]
					if val == "" {
						if i+1 >= len(argv) {
							fail("-" + c + " option requires an argument")
						}
						i++
						val = argv[i]
					}
					setOpt(&o, c, val)
					break
				}
				setOpt(&o, c, "")
			}
		default:
			rest = append(rest, a)
		}
	}
	return o, rest
}

func main() {
	o, args := parseArgs(os.Args[1:])
	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(1)
	}
	text := []rune(strings.Join(args, " "))

	fg, err := newFiglet(o.font, o.direction, o.justify, o.width)
	if err == errFontNotFound {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", o.font)
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "figlet: problem parsing font", o.font)
		os.Exit(1)
	}
	out, err := fg.render(text)
	if err != nil {
		fmt.Fprintln(os.Stderr, "figlet: width is not enough to print this text")
		os.Exit(1)
	}
	r := []rune(out)
	if o.reverse {
		r = reverseFS(r)
	}
	if o.flip {
		r = flipFS(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = normalizeSurroundingNewlines(r)
	}
	os.Stdout.WriteString(string(r) + "\n")
}
