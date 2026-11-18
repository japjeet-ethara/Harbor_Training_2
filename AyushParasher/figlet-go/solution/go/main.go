// figlet: a Go port of pyfiglet's command-line program (pyfiglet commit 255827ef).
//
//	figlet [-f FONT] [-w COLS] [-j SIDE] [-D DIRECTION] [-r] [-F] [-s] [-n] TEXT...
//
// Output is byte-for-byte identical to `python -m pyfiglet` for these options.
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
                        auto, left-to-right or right-to-left (default: auto)
  -j SIDE, --justify=SIDE
                        auto, left, center or right (default: auto)
  -w COLS, --width=COLS
                        width for wrapping/justification (default: 80)
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

func main() {
	os.Exit(run(os.Args[1:]))
}

func fail(code int, format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "figlet: error: "+format+"\n", a...)
	return code
}

func run(argv []string) (code int) {
	opts := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var texts []string

	// optparse-style parsing: short options may be clustered (-rF) and take
	// their value attached (-w40) or as the next argument (-w 40).
	takesValue := map[byte]bool{'f': true, 'D': true, 'j': true, 'w': true, 'c': true, 'L': true}
	long := map[string]string{
		"font": "f", "direction": "D", "justify": "j", "width": "w", "reverse": "r",
		"flip": "F", "strip-surrounding-newlines": "s", "normalize-surrounding-newlines": "n",
		"color": "c", "load": "L", "list_fonts": "l", "info_font": "i",
	}
	set := func(opt byte, val string) int {
		switch opt {
		case 'f':
			opts.font = val
		case 'D':
			if val != "auto" && val != "left-to-right" && val != "right-to-left" {
				return fail(2, "option -D: invalid choice: %q", val)
			}
			opts.direction = val
		case 'j':
			if val != "auto" && val != "left" && val != "center" && val != "right" {
				return fail(2, "option -j: invalid choice: %q", val)
			}
			opts.justify = val
		case 'w':
			n, err := parseOptInt(val)
			if err != nil {
				return fail(2, "option -w: invalid integer value: %q", val)
			}
			opts.width = n
		case 'r':
			opts.reverse = true
		case 'F':
			opts.flip = true
		case 's':
			opts.strip = true
		case 'n':
			opts.normalize = true
		}
		return 0 // -c, -L, -l, -i are accepted and ignored (out of scope)
	}

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			texts = append(texts, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a[2:], "=")
			opt, ok := long[name]
			if !ok {
				return fail(2, "no such option: --%s", name)
			}
			if takesValue[opt[0]] && !hasVal {
				if i+1 >= len(argv) {
					return fail(2, "--%s option requires an argument", name)
				}
				i++
				val = argv[i]
			}
			if c := set(opt[0], val); c != 0 {
				return c
			}
		case len(a) > 1 && a[0] == '-':
			for j := 1; j < len(a); j++ {
				c := a[j]
				if !strings.ContainsRune("fDjwcLrFsnli", rune(c)) {
					return fail(2, "no such option: -%c", c)
				}
				if takesValue[c] {
					val := a[j+1:]
					if val == "" {
						if i+1 >= len(argv) {
							return fail(2, "-%c option requires an argument", c)
						}
						i++
						val = argv[i]
					}
					if code := set(c, val); code != 0 {
						return code
					}
					break
				}
				set(c, "")
			}
		default:
			texts = append(texts, a)
		}
	}

	if len(texts) == 0 {
		fmt.Print(usage)
		return 1
	}
	text := strings.Join(texts, " ")

	font, err := loadFont(opts.font)
	if err == errFontNotFound {
		fmt.Printf("pyfiglet error: requested font %s not found.\n", pyRepr(opts.font))
		return 1
	}
	if err != nil {
		return fail(1, "%v", err)
	}

	direction := opts.direction
	if direction == "auto" {
		direction = "left-to-right"
		if font.printDirection == 1 {
			direction = "right-to-left"
		}
	}
	justify := opts.justify
	if justify == "auto" {
		justify = "left"
		if direction == "right-to-left" {
			justify = "right"
		}
	}

	// Python raises (and prints nothing) on these; mirror that with exit code 1.
	defer func() {
		if r := recover(); r != nil {
			c, ok := r.(crash)
			if !ok {
				panic(r)
			}
			code = fail(1, "%s", c.msg)
		}
	}()
	rendered, err := render(text, font, direction, justify, opts.width)
	if err != nil {
		return fail(1, "%v", err)
	}

	r := []rune(rendered)
	if opts.reverse {
		r = figReverse(r)
	}
	if opts.flip {
		r = figFlip(r)
	}
	if opts.strip {
		r = figStrip(r)
	} else if opts.normalize {
		r = figNormalize(r)
	}
	os.Stdout.WriteString(string(r) + "\n")
	return 0
}

// parseOptInt follows optparse's _parse_num: 0x / 0b / leading-0 octal / decimal.
func parseOptInt(s string) (int, error) {
	t := strings.ToLower(s)
	base := 10
	switch {
	case strings.HasPrefix(t, "0x"):
		base, t = 16, t[2:]
	case strings.HasPrefix(t, "0b"):
		base, t = 2, t[2:]
	case strings.HasPrefix(t, "0") && len(t) > 1:
		base, t = 8, t[1:]
	}
	n, err := strconv.ParseInt(t, base, 64)
	return int(n), err
}

// pyRepr is Python's repr() for a simple str.
func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}
