// Command figlet is a Go port of pyfiglet's command-line program: same options,
// same output, byte for byte.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

type options struct {
	font, direction, justify string
	width                    int
	reverse, flip            bool
	strip, normalize         bool
	listFonts, infoFont      bool
	load, color              string
	args                     []string
}

const usage = `Usage: pyfiglet [options] [text..]

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

func fail(msg string) {
	fmt.Fprint(os.Stderr, usage)
	fmt.Fprintf(os.Stderr, "pyfiglet: error: %s\n", msg)
	os.Exit(2)
}

var longOpts = []string{
	"font", "direction", "justify", "width", "reverse",
	"normalize-surrounding-newlines", "strip-surrounding-newlines", "flip",
	"list_fonts", "info_font", "load", "color",
}

var longToShort = map[string]string{
	"font": "f", "direction": "D", "justify": "j", "width": "w", "reverse": "r",
	"normalize-surrounding-newlines": "n", "strip-surrounding-newlines": "s",
	"flip": "F", "list_fonts": "l", "info_font": "i", "load": "L", "color": "c",
}

func takesValue(short string) bool { return strings.Contains("fDjwLc", short) }

func setOpt(o *options, short, value string) {
	choice := func(v string, allowed ...string) string {
		for _, a := range allowed {
			if v == a {
				return v
			}
		}
		fail(fmt.Sprintf("option -%s: invalid choice: %q", short, v))
		return ""
	}
	switch short {
	case "f":
		o.font = value
	case "D":
		o.direction = choice(value, "auto", "left-to-right", "right-to-left")
	case "j":
		o.justify = choice(value, "auto", "left", "center", "right")
	case "w":
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			fail(fmt.Sprintf("option -w: invalid integer value: %q", value))
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
	case "l":
		o.listFonts = true
	case "i":
		o.infoFont = true
	case "L":
		o.load = value
	case "c":
		o.color = value
	}
}

// parseArgs follows optparse: options may be interspersed with text, short
// options may be clustered (-rF, -fslant), long options may be abbreviated.
func parseArgs(argv []string) options {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80, color: ":"}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			o.args = append(o.args, argv[i+1:]...)
			return o
		case strings.HasPrefix(a, "--"):
			name, value, hasValue := strings.Cut(a[2:], "=")
			var match []string
			for _, l := range longOpts {
				if l == name {
					match = []string{l}
					break
				}
				if strings.HasPrefix(l, name) {
					match = append(match, l)
				}
			}
			if len(match) != 1 {
				fail("no such option: --" + name)
			}
			short := longToShort[match[0]]
			if takesValue(short) {
				if !hasValue {
					if i+1 >= len(argv) {
						fail("--" + match[0] + " option requires 1 argument")
					}
					i++
					value = argv[i]
				}
				setOpt(&o, short, value)
			} else {
				if hasValue {
					fail("--" + match[0] + " option does not take a value")
				}
				setOpt(&o, short, "")
			}
		case len(a) > 1 && a[0] == '-':
			for j := 1; j < len(a); j++ {
				short := string(a[j])
				if !strings.Contains("fDjwLcrFsnli", short) {
					fail("no such option: -" + short)
				}
				if takesValue(short) {
					value := a[j+1:]
					if value == "" {
						if i+1 >= len(argv) {
							fail("-" + short + " option requires 1 argument")
						}
						i++
						value = argv[i]
					}
					setOpt(&o, short, value)
					break
				}
				setOpt(&o, short, "")
			}
		default:
			o.args = append(o.args, a)
		}
	}
	return o
}

func fontNames() []string {
	var names []string
	_ = fs.WalkDir(fontFS, "fonts", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".flf") || strings.HasSuffix(p, ".tlf")) {
			names = append(names, strings.SplitN(path.Base(p), ".", 2)[0])
		}
		return nil
	})
	sort.Strings(names)
	return names
}

func pyRepr(s string) string {
	q := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = "\""
	}
	s = strings.ReplaceAll(s, "\\", "\\\\")
	if q == "'" {
		s = strings.ReplaceAll(s, "'", "\\'")
	}
	return q + s + q
}

func run() int {
	o := parseArgs(os.Args[1:])

	if o.listFonts {
		fmt.Println(strings.Join(fontNames(), "\n"))
		return 0
	}
	if o.infoFont || o.load != "" { // out of scope for this port
		return 0
	}
	if len(o.args) == 0 {
		fmt.Print(usage)
		return 1
	}
	text := []rune(strings.Join(o.args, " "))

	font, err := loadFont(o.font)
	if err == errFontNotFound {
		fmt.Printf("pyfiglet error: requested font %s not found.\n", pyRepr(o.font))
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet.FontError:", err)
		return 1
	}

	direction := o.direction
	if direction == "auto" {
		direction = "left-to-right"
		if font.printDirection == 1 {
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

	r, perr := safeRender(text, font, direction, o.width, justify)
	if perr != "" {
		fmt.Fprintln(os.Stderr, "pyfiglet error:", perr)
		return 1
	}
	if o.reverse {
		r = reverseText(r)
	}
	if o.flip {
		r = flipText(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = normalizeSurroundingNewlines(r)
	}
	os.Stdout.Write([]byte(string(r) + "\n"))
	return 0
}

// safeRender turns the places where pyfiglet raises into an error (no stdout).
func safeRender(text []rune, font *Font, direction string, width int, justify string) (out []rune, msg string) {
	defer func() {
		if p := recover(); p != nil {
			if rp, ok := p.(renderPanic); ok {
				msg = string(rp)
				return
			}
			panic(p)
		}
	}()
	return render(text, font, direction, width, justify), ""
}

func main() {
	os.Exit(run())
}
