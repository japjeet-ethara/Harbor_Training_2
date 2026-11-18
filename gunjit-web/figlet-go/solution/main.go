package main

// Command-line front end: pyfiglet.main() ported to Go. Fonts are embedded so the
// binary works from any working directory.

import (
	"embed"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var fontFS embed.FS

type options struct {
	font      string
	width     int
	justify   string
	direction string
	reverse   bool
	flip      bool
	strip     bool
	normalize bool
	text      []string
}

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

// parseArgs accepts what optparse accepts for the supported options: options may
// appear anywhere, short options may be clustered (-rF) or carry their value
// attached (-fslant), long options may use "=".
func parseArgs(args []string) (*options, error) {
	o := &options{font: "standard", width: 80, justify: "auto", direction: "auto"}
	var widthStr string
	setValue := func(name, v string) error {
		switch name {
		case "f", "font":
			o.font = v
		case "w", "width":
			widthStr = v
		case "j", "justify":
			o.justify = v
		case "D", "direction":
			o.direction = v
		}
		return nil
	}
	setFlag := func(name string) bool {
		switch name {
		case "r", "reverse":
			o.reverse = true
		case "F", "flip":
			o.flip = true
		case "s", "strip-surrounding-newlines":
			o.strip = true
		case "n", "normalize-surrounding-newlines":
			o.normalize = true
		default:
			return false
		}
		return true
	}
	takesValue := func(name string) bool {
		switch name {
		case "f", "font", "w", "width", "j", "justify", "D", "direction":
			return true
		}
		return false
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			o.text = append(o.text, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "--") {
			name, val, hasVal := strings.Cut(a[2:], "=")
			if takesValue(name) {
				if !hasVal {
					i++
					if i >= len(args) {
						return nil, fmt.Errorf("--%s option requires 1 argument", name)
					}
					val = args[i]
				}
				setValue(name, val)
				continue
			}
			if hasVal || !setFlag(name) {
				return nil, fmt.Errorf("no such option: %s", a)
			}
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			for j := 1; j < len(a); j++ {
				name := a[j : j+1]
				if takesValue(name) {
					val := a[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							return nil, fmt.Errorf("-%s option requires 1 argument", name)
						}
						val = args[i]
					}
					setValue(name, val)
					break
				}
				if !setFlag(name) {
					return nil, fmt.Errorf("no such option: -%s", name)
				}
			}
			continue
		}
		o.text = append(o.text, a)
	}
	if widthStr != "" {
		w, err := strconv.Atoi(strings.TrimSpace(widthStr))
		if err != nil {
			return nil, fmt.Errorf("option -w: invalid integer value: %q", widthStr)
		}
		o.width = w
	}
	switch o.justify {
	case "auto", "left", "center", "right":
	default:
		return nil, fmt.Errorf("option -j: invalid choice: %q", o.justify)
	}
	switch o.direction {
	case "auto", "left-to-right", "right-to-left":
	default:
		return nil, fmt.Errorf("option -D: invalid choice: %q", o.direction)
	}
	return o, nil
}

func readFont(name string) ([]byte, bool) {
	for _, ext := range []string{"tlf", "flf"} {
		if b, err := fontFS.ReadFile("fonts/" + name + "." + ext); err == nil {
			return b, true
		}
		// shortcut: pyfiglet also looks in ./ and /usr/share/figlet; this task only uses the shipped fonts
	}
	return nil, false
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%sfiglet: error: %v\n", usage, err)
		return 2
	}
	if len(opts.text) == 0 {
		fmt.Fprint(stdout, usage)
		return 1
	}
	text := strings.Join(opts.text, " ")

	raw, ok := readFont(opts.font)
	if !ok {
		fmt.Fprintf(stdout, "pyfiglet error: requested font %s not found.\n", pyRepr(opts.font))
		return 1
	}
	font, err := loadFont(opts.font, raw)
	if err != nil {
		fmt.Fprintf(stderr, "pyfiglet.FontError: %v\n", err)
		return 1
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

	r, err := newBuilder(text, font, direction, opts.width, justify).render()
	if err != nil {
		fmt.Fprintf(stderr, "pyfiglet: %v\n", err)
		return 1
	}
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
	io.WriteString(stdout, r+"\n")
	return 0
}

// pyRepr renders a string the way Python's repr does for the font-not-found message.
func pyRepr(s string) string {
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
