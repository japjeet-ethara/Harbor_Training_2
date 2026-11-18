package main

import (
	"fmt"
	"strconv"
	"strings"
)

type options struct {
	font, justify, direction  string
	width                     int
	mirror, flip, strip, norm bool
}

var longOptions = map[string]byte{
	"font": 'f', "justify": 'j', "direction": 'D', "width": 'w',
	"reverse": 'r', "flip": 'F', "strip-surrounding-newlines": 's',
	"normalize-surrounding-newlines": 'n',
}

func parseArgs(args []string) (options, string, error) {
	opts := options{font: "standard", width: 80, justify: "auto", direction: "auto"}
	var words []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			words = append(words, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			words = append(words, arg)
			continue
		}
		flags := arg[1:]
		var value string
		var attached bool
		long := strings.HasPrefix(flags, "-")
		if long {
			var name string
			name, value, attached = strings.Cut(flags[1:], "=")
			key, ok := longOptions[name]
			if !ok {
				for candidate, option := range longOptions {
					if strings.HasPrefix(candidate, name) {
						if key != 0 {
							return opts, "", fmt.Errorf("ambiguous option %s", arg)
						}
						key = option
					}
				}
			}
			if key == 0 {
				return opts, "", fmt.Errorf("unknown option %s", arg)
			}
			flags = string(key)
		}
		for flags != "" {
			key := flags[0]
			flags = flags[1:]
			switch key {
			case 'f', 'w', 'j', 'D':
				if flags != "" {
					value, flags = flags, ""
				} else if !attached {
					i++
					if i == len(args) {
						return opts, "", fmt.Errorf("option -%c needs a value", key)
					}
					value = args[i]
				}
				if err := opts.setValue(key, value); err != nil {
					return opts, "", err
				}
			case 'r', 'F', 's', 'n':
				if attached {
					return opts, "", fmt.Errorf("option -%c takes no value", key)
				}
				switch key {
				case 'r':
					opts.mirror = true
				case 'F':
					opts.flip = true
				case 's':
					opts.strip = true
				case 'n':
					opts.norm = true
				}
			default:
				return opts, "", fmt.Errorf("unknown option -%c", key)
			}
		}
	}
	if len(words) == 0 {
		return opts, "", fmt.Errorf("text argument required")
	}
	return opts, strings.Join(words, " "), nil
}

func (opts *options) setValue(key byte, value string) error {
	switch key {
	case 'f':
		opts.font = value
	case 'w':
		base := 10
		number := strings.ToLower(value)
		switch {
		case strings.HasPrefix(number, "0x"):
			base, value = 16, value[2:]
		case strings.HasPrefix(number, "0b"):
			base, value = 2, value[2:]
			if value == "" {
				value = "0"
			}
		case strings.HasPrefix(number, "0"):
			base = 8
		}
		width, err := strconv.ParseInt(value, base, 0)
		if err != nil {
			return err
		}
		opts.width = int(width)
	case 'j':
		switch value {
		case "auto", "left", "center", "right":
			opts.justify = value
		default:
			return fmt.Errorf("invalid justification %q", value)
		}
	case 'D':
		switch value {
		case "auto", "left-to-right", "right-to-left":
			opts.direction = value
		default:
			return fmt.Errorf("invalid direction %q", value)
		}
	}
	return nil
}
