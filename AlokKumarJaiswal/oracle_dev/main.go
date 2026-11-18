package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CLIOptions holds the parsed CLI flags and positional arguments.
type CLIOptions struct {
	Font      string
	Width     int
	Justify   string
	Direction string
	Reverse   bool
	Flip      bool
	Strip     bool
	Normalize bool
	Args      []string
}

func printHelp() {
	fmt.Println("Usage: figlet [options] [text..]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -f FONT, --font=FONT  font to render with (default: standard)")
	fmt.Println("  -D DIRECTION, --direction=DIRECTION")
	fmt.Println("                        set direction text will be formatted in (default: auto)")
	fmt.Println("  -j SIDE, --justify=SIDE")
	fmt.Println("                        set justification, defaults to print direction")
	fmt.Println("  -w COLS, --width=COLS")
	fmt.Println("                        set terminal width for wrapping/justification (default: 80)")
	fmt.Println("  -r, --reverse         shows mirror image of output text")
	fmt.Println("  -n, --normalize-surrounding-newlines")
	fmt.Println("                        output has one empty line before and after")
	fmt.Println("  -s, --strip-surrounding-newlines")
	fmt.Println("                        removes empty leading and trailing lines")
	fmt.Println("  -F, --flip            flips rendered output text over")
}

// parseArgs parses CLI arguments matching optparse conventions.
func parseArgs(rawArgs []string) (*CLIOptions, error) {
	opts := &CLIOptions{
		Font:      "standard",
		Width:     80,
		Justify:   "auto",
		Direction: "auto",
	}

	i := 0
	for i < len(rawArgs) {
		arg := rawArgs[i]
		if arg == "--" {
			i++
			opts.Args = append(opts.Args, rawArgs[i:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			name := arg[2:]
			val := ""
			hasVal := false
			if eqIdx := strings.IndexByte(name, '='); eqIdx != -1 {
				val = name[eqIdx+1:]
				name = name[:eqIdx]
				hasVal = true
			}

			switch name {
			case "font":
				if !hasVal {
					i++
					if i >= len(rawArgs) {
						return nil, fmt.Errorf("--font requires an argument")
					}
					val = rawArgs[i]
				}
				opts.Font = val
			case "width":
				if !hasVal {
					i++
					if i >= len(rawArgs) {
						return nil, fmt.Errorf("--width requires an argument")
					}
					val = rawArgs[i]
				}
				w, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid width: %s", val)
				}
				opts.Width = w
			case "justify":
				if !hasVal {
					i++
					if i >= len(rawArgs) {
						return nil, fmt.Errorf("--justify requires an argument")
					}
					val = rawArgs[i]
				}
				opts.Justify = val
			case "direction":
				if !hasVal {
					i++
					if i >= len(rawArgs) {
						return nil, fmt.Errorf("--direction requires an argument")
					}
					val = rawArgs[i]
				}
				opts.Direction = val
			case "reverse":
				opts.Reverse = true
			case "flip":
				opts.Flip = true
			case "strip-surrounding-newlines":
				opts.Strip = true
			case "normalize-surrounding-newlines":
				opts.Normalize = true
			default:
				return nil, fmt.Errorf("unknown option: %s", arg)
			}
			i++
			continue
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			j := 1
			for j < len(arg) {
				opt := arg[j]
				switch opt {
				case 'f':
					if j+1 < len(arg) {
						opts.Font = arg[j+1:]
						j = len(arg)
					} else {
						i++
						if i >= len(rawArgs) {
							return nil, fmt.Errorf("-f requires an argument")
						}
						opts.Font = rawArgs[i]
					}
				case 'w':
					var val string
					if j+1 < len(arg) {
						val = arg[j+1:]
						j = len(arg)
					} else {
						i++
						if i >= len(rawArgs) {
							return nil, fmt.Errorf("-w requires an argument")
						}
						val = rawArgs[i]
					}
					w, err := strconv.Atoi(val)
					if err != nil {
						return nil, fmt.Errorf("invalid width: %s", val)
					}
					opts.Width = w
				case 'j':
					if j+1 < len(arg) {
						opts.Justify = arg[j+1:]
						j = len(arg)
					} else {
						i++
						if i >= len(rawArgs) {
							return nil, fmt.Errorf("-j requires an argument")
						}
						opts.Justify = rawArgs[i]
					}
				case 'D':
					if j+1 < len(arg) {
						opts.Direction = arg[j+1:]
						j = len(arg)
					} else {
						i++
						if i >= len(rawArgs) {
							return nil, fmt.Errorf("-D requires an argument")
						}
						opts.Direction = rawArgs[i]
					}
				case 'r':
					opts.Reverse = true
				case 'F':
					opts.Flip = true
				case 's':
					opts.Strip = true
				case 'n':
					opts.Normalize = true
				default:
					return nil, fmt.Errorf("unknown option: -%c", opt)
				}
				j++
			}
			i++
			continue
		}

		// Positional argument
		opts.Args = append(opts.Args, arg)
		i++
	}

	return opts, nil
}

// renderText renders input text to FIGlet string.
// Replaces Figlet.renderText and FigletRenderingEngine.render.
func renderText(text string, font *FigletFont, direction string, width int, justify string) (string, error) {
	builder := NewFigletBuilder(text, font, direction, width, justify)
	for builder.IsNotFinished() {
		if err := builder.AddCharToProduct(); err != nil {
			return "", err
		}
		builder.GoToNextChar()
	}
	return builder.ReturnProduct(), nil
}

// main entrypoint.
// Replaces pyfiglet.main.
func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		printHelp()
		os.Exit(2)
	}

	if len(opts.Args) == 0 {
		printHelp()
		os.Exit(1)
	}

	text := strings.Join(opts.Args, " ")

	font, err := loadFontByName(opts.Font)
	if err != nil {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", opts.Font)
		os.Exit(1)
	}

	// Resolve direction
	direction := opts.Direction
	if direction == "auto" {
		if font.PrintDirection == nil || *font.PrintDirection == 0 {
			direction = "left-to-right"
		} else if *font.PrintDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}

	// Resolve justification
	justify := opts.Justify
	if justify == "auto" {
		if direction == "left-to-right" {
			justify = "left"
		} else if direction == "right-to-left" {
			justify = "right"
		}
	}

	r, err := renderText(text, font, direction, opts.Width, justify)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if opts.Reverse {
		r = reverseString(r)
	}
	if opts.Flip {
		r = flipString(r)
	}
	if opts.Strip {
		r = stripSurroundingNewlines(r)
	} else if opts.Normalize {
		r = normalizeSurroundingNewlines(r)
	}

	os.Stdout.WriteString(r)
	os.Stdout.WriteString("\n")
}
