package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type options struct {
	font      string
	width     int
	justify   string
	direction string
	reverse   bool
	flip      bool
	strip     bool
	normalize bool
}

func defaultOptions() options {
	return options{
		font:      "standard",
		width:     80,
		justify:   "auto",
		direction: "auto",
	}
}

func optionValue(args []string, i *int, token string, off int) (string, bool) {
	if off < len(token) {
		return token[off:], true
	}
	if *i+1 >= len(args) {
		return "", false
	}
	*i = *i + 1
	return args[*i], true
}

func applyLongOption(o *options, name string, val string, hasVal bool, args []string, i *int) {
	switch name {
	case "font":
		if !hasVal {
			if v, ok := optionValue(args, i, "", 0); ok {
				val = v
			}
		}
		o.font = val
	case "width":
		if !hasVal {
			if v, ok := optionValue(args, i, "", 0); ok {
				val = v
			}
		}
		if n, err := strconv.Atoi(val); err == nil {
			o.width = n
		}
	case "justify":
		if !hasVal {
			if v, ok := optionValue(args, i, "", 0); ok {
				val = v
			}
		}
		o.justify = val
	case "direction":
		if !hasVal {
			if v, ok := optionValue(args, i, "", 0); ok {
				val = v
			}
		}
		o.direction = val
	case "reverse":
		o.reverse = true
	case "flip":
		o.flip = true
	case "strip-surrounding-newlines":
		o.strip = true
	case "normalize-surrounding-newlines":
		o.normalize = true
	}
}

func parseArgs(args []string) (options, []string) {
	o := defaultOptions()
	var text []string
	stopOptions := false
	for i := 0; i < len(args); i++ {
		token := args[i]
		if stopOptions {
			text = append(text, token)
			continue
		}
		if token == "--" {
			stopOptions = true
			continue
		}
		if strings.HasPrefix(token, "--") && len(token) > 2 {
			body := token[2:]
			name := body
			val := ""
			hasVal := false
			if eq := strings.IndexByte(body, '='); eq >= 0 {
				name = body[:eq]
				val = body[eq+1:]
				hasVal = true
			}
			applyLongOption(&o, name, val, hasVal, args, &i)
			continue
		}
		if strings.HasPrefix(token, "-") && len(token) > 1 && token != "-" {
			rest := token[1:]
			for j := 0; j < len(rest); j++ {
				c := rest[j]
				switch c {
				case 'r':
					o.reverse = true
				case 'F':
					o.flip = true
				case 's':
					o.strip = true
				case 'n':
					o.normalize = true
				case 'f', 'w', 'j', 'D':
					val, ok := optionValue(args, &i, rest, j+1)
					if !ok {
						return o, text
					}
					switch c {
					case 'f':
						o.font = val
					case 'w':
						if n, err := strconv.Atoi(val); err == nil {
							o.width = n
						}
					case 'j':
						o.justify = val
					case 'D':
						o.direction = val
					}
					j = len(rest)
				default:
				}
			}
			continue
		}
		text = append(text, token)
	}
	return o, text
}

func execute(args []string) (string, error) {
	o, textArgs := parseArgs(args)
	text := strings.Join(textArgs, " ")

	font, err := loadFont(o.font)
	if err != nil {
		return "", err
	}

	direction := o.direction
	if direction == "auto" {
		if font.printDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}

	justify := o.justify
	if justify == "auto" {
		if direction == "right-to-left" {
			justify = "right"
		} else {
			justify = "left"
		}
	}

	out := renderText(text, font, direction, justify, o.width)
	if o.reverse {
		out = reverseFiglet(out)
	}
	if o.flip {
		out = flipFiglet(out)
	}
	if o.strip {
		out = stripSurroundingNewlines(out)
	} else if o.normalize {
		out = normalizeSurroundingNewlines(out)
	}

	return out, nil
}
func main() {
	out, err := execute(os.Args[1:])
	if err != nil {
		o, _ := parseArgs(os.Args[1:])
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", o.font)
		os.Exit(1)
	}
	os.Stdout.WriteString(out)
}
