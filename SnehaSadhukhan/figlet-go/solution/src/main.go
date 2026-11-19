// Command figlet is a Go port of pyfiglet's command-line tool, producing
// byte-for-byte identical output.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const version = "1.0.4"
const defaultFont = "standard"

const helpText = `Usage: figlet [options] [text..]

Options:
  --version             show program's version number and exit
  -h, --help            show this help message and exit
  -f FONT, --font=FONT  font to render with (default: standard)
  -D DIRECTION, --direction=DIRECTION
                        set direction text will be formatted in (default:
                        auto)
  -j SIDE, --justify=SIDE
                        set justification, defaults to print direction
  -w COLS, --width=COLS
                        set terminal width for wrapping/justification
                        (default: 80)
  -r, --reverse         shows mirror image of output text
  -n, --normalize-surrounding-newlines
                        output has one empty line before and after
  -s, --strip-surrounding-newlines
                        removes empty leading and trailing lines
  -F, --flip            flips rendered output text over
  -l, --list_fonts      show installed fonts list
  -i, --info_font       show font's information, use with -f FONT
  -L LOAD, --load=LOAD  load and install the specified font definition
  -c COLOR, --color=COLOR
                        prints text with passed foreground color,
                        --color=foreground:background
                        --color=:background                  # only background
                        --color=foreground | foreground:     # only foreground
                        --color=list                         # list all colors
                        COLOR = list[COLOR] | [0-255];[0-255];[0-255] (RGB)
`

// ---- option parsing (mirrors optparse) ------------------------------------

type option struct {
	short      string // "-f" or ""
	long       string // "--font"
	dest       string
	takesValue bool
	choices    []string
	isInt      bool
	action     string // "store", "store_true", "help", "version"
}

var options = []option{
	{long: "--version", action: "version"},
	{short: "-h", long: "--help", action: "help"},
	{short: "-f", long: "--font", dest: "font", takesValue: true, action: "store"},
	{short: "-D", long: "--direction", dest: "direction", takesValue: true, action: "store",
		choices: []string{"auto", "left-to-right", "right-to-left"}},
	{short: "-j", long: "--justify", dest: "justify", takesValue: true, action: "store",
		choices: []string{"auto", "left", "center", "right"}},
	{short: "-w", long: "--width", dest: "width", takesValue: true, isInt: true, action: "store"},
	{short: "-r", long: "--reverse", dest: "reverse", action: "store_true"},
	{short: "-n", long: "--normalize-surrounding-newlines", dest: "normalize", action: "store_true"},
	{short: "-s", long: "--strip-surrounding-newlines", dest: "strip", action: "store_true"},
	{short: "-F", long: "--flip", dest: "flip", action: "store_true"},
	{short: "-l", long: "--list_fonts", dest: "list_fonts", action: "store_true"},
	{short: "-i", long: "--info_font", dest: "info_font", action: "store_true"},
	{short: "-L", long: "--load", dest: "load", takesValue: true, action: "store"},
	{short: "-c", long: "--color", dest: "color", takesValue: true, action: "store"},
}

type opts struct {
	font, direction, justify, color, load string
	hasLoad                               bool
	width                                 int
	reverse, normalize, strip, flip       bool
	listFonts, infoFont                   bool
}

type usageError struct{ msg string }

func parserError(format string, a ...any) {
	panic(usageError{fmt.Sprintf(format, a...)})
}

func matchLong(s string) *option {
	for i := range options {
		if options[i].long == s {
			return &options[i]
		}
	}
	var matches []*option
	for i := range options {
		if strings.HasPrefix(options[i].long, s) {
			matches = append(matches, &options[i])
		}
	}
	switch len(matches) {
	case 1:
		return matches[0]
	case 0:
		parserError("no such option: %s", s)
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.long
		}
		sort.Strings(names)
		parserError("ambiguous option: %s (%s?)", s, strings.Join(names, ", "))
	}
	return nil
}

func matchShort(s string) *option {
	for i := range options {
		if options[i].short == s {
			return &options[i]
		}
	}
	parserError("no such option: %s", s)
	return nil
}

func (o *opts) apply(opt *option, name, value string) {
	switch opt.action {
	case "help":
		fmt.Print(helpText)
		os.Exit(0)
	case "version":
		fmt.Println(version)
		os.Exit(0)
	case "store_true":
		switch opt.dest {
		case "reverse":
			o.reverse = true
		case "normalize":
			o.normalize = true
		case "strip":
			o.strip = true
		case "flip":
			o.flip = true
		case "list_fonts":
			o.listFonts = true
		case "info_font":
			o.infoFont = true
		}
		return
	}
	if opt.choices != nil {
		ok := false
		for _, c := range opt.choices {
			if c == value {
				ok = true
			}
		}
		if !ok {
			parserError("option %s: invalid choice: %s", name, pyRepr(value))
		}
	}
	switch opt.dest {
	case "font":
		o.font = value
	case "direction":
		o.direction = value
	case "justify":
		o.justify = value
	case "width":
		w, err := parseOptparseInt(value)
		if err != nil {
			parserError("option %s: invalid integer value: %s", name, pyRepr(value))
		}
		o.width = w
	case "load":
		o.load, o.hasLoad = value, true
	case "color":
		o.color = value
	}
}

// parseOptparseInt mirrors optparse's _parse_num(val, int).
func parseOptparseInt(val string) (int, error) {
	low := strings.ToLower(val)
	switch {
	case strings.HasPrefix(low, "0x"):
		return pyParseInt(val, 16)
	case strings.HasPrefix(low, "0b"):
		v := val[2:]
		if v == "" {
			v = "0"
		}
		return pyParseInt(v, 2)
	case strings.HasPrefix(val, "0"):
		return pyParseInt(val, 8)
	}
	return pyParseInt(val, 10)
}

// parseArgs mirrors OptionParser.parse_args with interspersed arguments.
func parseArgs(argv []string) (o opts, positional []string) {
	o = opts{font: defaultFont, direction: "auto", justify: "auto", width: 80, color: ":"}
	rargs := append([]string(nil), argv...)
	for len(rargs) > 0 {
		arg := rargs[0]
		switch {
		case arg == "--":
			positional = append(positional, rargs[1:]...)
			return
		case strings.HasPrefix(arg, "--"):
			rargs = rargs[1:]
			name, value, explicit := arg, "", false
			if i := strings.Index(arg, "="); i >= 0 {
				name, value, explicit = arg[:i], arg[i+1:], true
			}
			opt := matchLong(name)
			name = opt.long
			if opt.takesValue {
				if !explicit {
					if len(rargs) < 1 {
						parserError("%s option requires 1 argument", name)
					}
					value, rargs = rargs[0], rargs[1:]
				}
			} else if explicit {
				parserError("%s option does not take a value", name)
			}
			o.apply(opt, name, value)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			rargs = rargs[1:]
			chars := []rune(arg[1:])
			for i := 0; i < len(chars); i++ {
				name := "-" + string(chars[i])
				opt := matchShort(name)
				value := ""
				if opt.takesValue {
					if i+1 < len(chars) {
						value = string(chars[i+1:])
					} else {
						if len(rargs) < 1 {
							parserError("%s option requires 1 argument", name)
						}
						value, rargs = rargs[0], rargs[1:]
					}
					o.apply(opt, name, value)
					break
				}
				o.apply(opt, name, value)
			}
		default:
			positional = append(positional, arg)
			rargs = rargs[1:]
		}
	}
	return
}

// pyRepr approximates Python's repr() of a str (used in messages only).
func pyRepr(s string) string {
	q := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		q = `"`
	}
	r := strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`, q, `\`+q)
	return q + r.Replace(s) + q
}

// ---- colours (mirrors parse_color / color_to_ansi) -------------------------

var colorCodes = map[string]int{
	"BLACK": 30, "RED": 31, "GREEN": 32, "YELLOW": 33, "BLUE": 34, "MAGENTA": 35, "CYAN": 36,
	"LIGHT_GRAY": 37, "DEFAULT": 39, "DARK_GRAY": 90, "LIGHT_RED": 91, "LIGHT_GREEN": 92,
	"LIGHT_YELLOW": 93, "LIGHT_BLUE": 94, "LIGHT_MAGENTA": 95, "LIGHT_CYAN": 96, "WHITE": 97,
	"RESET": 0,
}

func colorToANSI(color string, background bool) (string, error) {
	if color == "" {
		return "", nil
	}
	color = strings.ToUpper(color)
	n := strings.Count(color, ";")
	if n > 0 && n != 2 {
		return "", fmt.Errorf("Specified color '{}' not a valid color in R;G;B format")
	}
	code, known := colorCodes[color]
	if n == 0 && !known {
		return "", fmt.Errorf("Specified color '%s' not found in ANSI COLOR_CODES list", color)
	}
	if known {
		if background {
			code += 10
		}
		return fmt.Sprintf("\033[%dm", code), nil
	}
	base := 38
	if background {
		base = 48
	}
	return fmt.Sprintf("\033[%d;2;%sm", base, color), nil
}

func parseColor(color string) (string, error) {
	fg, bg, _ := strings.Cut(color, ":")
	f, err := colorToANSI(fg, false)
	if err != nil {
		return "", err
	}
	b, err := colorToANSI(bg, true)
	if err != nil {
		return "", err
	}
	return f + b, nil
}

// ---- main (mirrors pyfiglet.main) -----------------------------------------

func fail(err error) {
	fmt.Fprintln(os.Stderr, "figlet error:", err)
	os.Exit(1)
}

func main() {
	var o opts
	var args []string
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ue, ok := r.(usageError); ok {
					fmt.Fprintf(os.Stderr, "Usage: figlet [options] [text..]\n\nfiglet: error: %s\n", ue.msg)
					os.Exit(2)
				}
				panic(r)
			}
		}()
		o, args = parseArgs(os.Args[1:])
	}()

	if o.listFonts {
		fmt.Println(strings.Join(listFonts(), "\n"))
		os.Exit(0)
	}
	if o.color == "list" {
		keys := make([]string, 0, len(colorCodes))
		for k := range colorCodes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Println("[0-255];[0-255];[0-255] # RGB\n" + strings.Join(keys, "\n"))
		os.Exit(0)
	}
	if o.infoFont {
		info, err := infoFont(o.font)
		if err != nil {
			fail(err)
		}
		fmt.Println(info)
		os.Exit(0)
	}
	if o.hasLoad {
		fail(fmt.Errorf("installing fonts (-L) is not supported"))
	}
	if len(args) == 0 {
		fmt.Print(helpText)
		os.Exit(1)
	}
	text := strings.Join(args, " ")

	f, err := newFiglet(o.font, o.direction, o.justify, o.width)
	if err != nil {
		if _, ok := err.(fontNotFound); ok {
			fmt.Printf("pyfiglet error: requested font %s not found.\n", pyRepr(o.font))
			os.Exit(1)
		}
		fail(err)
	}
	r, err := f.renderText(text)
	if err != nil {
		fail(err)
	}
	if o.reverse {
		r = figletReverse(r)
	}
	if o.flip {
		r = figletFlip(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = normalizeSurroundingNewlines(r)
	}

	ansi, err := parseColor(o.color)
	if err != nil {
		fail(err)
	}
	out := ansi + r + "\n"
	if ansi != "" {
		out += "\033[0m"
	}
	os.Stdout.WriteString(out)
}
