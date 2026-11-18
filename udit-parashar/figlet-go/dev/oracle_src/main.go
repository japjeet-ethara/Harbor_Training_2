package main

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var fontFS embed.FS

const usage = "Usage: figlet [options] [text..]\n"

type options struct {
	font      string
	direction string
	justify   string
	width     int
	reverse   bool
	flip      bool
	strip     bool
	normalize bool
}

func die(code int, format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(code)
}

var longNames = []string{
	"font", "direction", "justify", "width", "reverse", "normalize-surrounding-newlines",
	"strip-surrounding-newlines", "flip",
}

func longToShort(name string) (byte, bool) {
	m := map[string]byte{
		"font": 'f', "direction": 'D', "justify": 'j', "width": 'w', "reverse": 'r',
		"normalize-surrounding-newlines": 'n', "strip-surrounding-newlines": 's', "flip": 'F',
	}
	if c, ok := m[name]; ok {
		return c, true
	}
	var hit []string
	for _, n := range longNames {
		if strings.HasPrefix(n, name) {
			hit = append(hit, n)
		}
	}
	if len(hit) == 1 {
		return m[hit[0]], true
	}
	return 0, false
}

func parseArgs(args []string) (options, []string) {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var pos []string
	set := func(c byte, val string) {
		switch c {
		case 'f':
			o.font = val
		case 'D':
			if val != "auto" && val != "left-to-right" && val != "right-to-left" {
				die(2, "figlet: error: option -D: invalid choice: %q", val)
			}
			o.direction = val
		case 'j':
			if val != "auto" && val != "left" && val != "center" && val != "right" {
				die(2, "figlet: error: option -j: invalid choice: %q", val)
			}
			o.justify = val
		case 'w':
			n, err := strconv.Atoi(val)
			if err != nil {
				die(2, "figlet: error: option -w: invalid integer value: %q", val)
			}
			o.width = n
		}
	}
	takesValue := func(c byte) bool { return c == 'f' || c == 'D' || c == 'j' || c == 'w' }
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			pos = append(pos, args[i+1:]...)
			return o, pos
		case strings.HasPrefix(a, "--"):
			name, val, hasEq := strings.Cut(a[2:], "=")
			c, ok := longToShort(name)
			if !ok {
				die(2, "figlet: error: no such option: %s", a)
			}
			if takesValue(c) {
				if !hasEq {
					i++
					if i >= len(args) {
						die(2, "figlet: error: %s option requires an argument", a)
					}
					val = args[i]
				}
				set(c, val)
			} else {
				applyFlag(&o, c)
			}
		case len(a) > 1 && a[0] == '-':
			for k := 1; k < len(a); k++ {
				c := a[k]
				if takesValue(c) {
					val := a[k+1:]
					if val == "" {
						i++
						if i >= len(args) {
							die(2, "figlet: error: -%c option requires an argument", c)
						}
						val = args[i]
					}
					set(c, val)
					break
				}
				if c != 'r' && c != 'F' && c != 's' && c != 'n' {
					die(2, "figlet: error: no such option: -%c (not supported by this port)", c)
				}
				applyFlag(&o, c)
			}
		default:
			pos = append(pos, a)
		}
	}
	return o, pos
}

func applyFlag(o *options, c byte) {
	switch c {
	case 'r':
		o.reverse = true
	case 'F':
		o.flip = true
	case 's':
		o.strip = true
	case 'n':
		o.normalize = true
	}
}

func loadFont(name string) (*Font, bool) {
	for _, ext := range []string{"tlf", "flf"} {
		data, err := fontFS.ReadFile("fonts/" + name + "." + ext)
		if err == nil {
			f, perr := parseFont(name, data)
			if perr != nil {
				die(1, "pyfiglet: %v", perr)
			}
			return f, true
		}
	}
	return nil, false
}

func run() int {
	o, pos := parseArgs(os.Args[1:])
	if len(pos) == 0 {
		fmt.Print(usage)
		return 1
	}
	text := strings.Join(pos, " ")

	font, ok := loadFont(o.font)
	if !ok {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", o.font)
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
		if direction == "left-to-right" {
			justify = "left"
		} else {
			justify = "right"
		}
	}

	r := newBuilder(text, font, direction, o.width, justify).render()
	if o.reverse {
		r = reverseString(r)
	}
	if o.flip {
		r = flipString(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = normalizeSurroundingNewlines(r)
	}

	w := bufio.NewWriter(os.Stdout)
	w.WriteString(r)
	w.WriteString("\n")
	w.Flush()
	return 0
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(pyError); ok {
				fmt.Fprintln(os.Stderr, "pyfiglet:", e.msg)
			} else {
				fmt.Fprintln(os.Stderr, "pyfiglet: internal error:", r)
			}
			os.Exit(1)
		}
	}()
	os.Exit(run())
}
