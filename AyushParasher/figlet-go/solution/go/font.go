package main

// Port of pyfiglet.FigletFont (preloadFont + loadFont).

import (
	"embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var fontFS embed.FS

var errFontNotFound = errors.New("font not found")

type Font struct {
	name           string
	height         int
	hardBlank      []rune
	printDirection int // -1 when the header has none (Python: None)
	smushMode      int
	chars          map[int64][][]rune
	width          map[int64]int
}

func loadFont(name string) (*Font, error) {
	var data []byte
	for _, ext := range []string{"tlf", "flf"} {
		if b, err := fontFS.ReadFile("fonts/" + name + "." + ext); err == nil {
			data = b
			break
		}
	}
	if len(data) == 0 {
		return nil, errFontNotFound
	}
	f := &Font{name: name, chars: map[int64][][]rune{}, width: map[int64]int{}}
	if err := f.parse(pyDecodeReplace(data)); err != nil {
		return nil, fmt.Errorf("problem parsing %s font: %w", name, err)
	}
	return f, nil
}

func (f *Font) parse(text []rune) error {
	// Unicode line splitting characters other than CRLF become spaces.
	for i, r := range text {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			text[i] = ' '
		}
	}
	data := pySplitlines(text)
	pop := func() ([]rune, error) {
		if len(data) == 0 {
			return nil, errors.New("pop from empty list")
		}
		l := data[0]
		data = data[1:]
		return l, nil
	}

	header, err := pop()
	if err != nil {
		return err
	}
	// reMagicNumber = ^[tf]lf2.
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') ||
		string(header[1:4]) != "lf2" || header[4] == '\n' {
		return fmt.Errorf("%s is not a valid figlet font", f.name)
	}
	fields := pyFields(header[5:])
	if len(fields) < 6 {
		return fmt.Errorf("malformed header for %s", f.name)
	}
	f.hardBlank = fields[0]
	nums := make([]int, 5)
	for k := 0; k < 5; k++ {
		if nums[k], err = pyInt(fields[k+1], 10); err != nil {
			return err
		}
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	f.printDirection = -1
	fullLayout, haveFull := 0, false
	if len(fields) > 6 {
		if f.printDirection, err = pyInt(fields[6], 10); err != nil {
			return err
		}
	}
	if len(fields) > 7 {
		if fullLayout, err = pyInt(fields[7], 10); err != nil {
			return err
		}
		haveFull = true
	}
	if !haveFull {
		switch {
		case oldLayout == 0:
			fullLayout = 64
		case oldLayout < 0:
			fullLayout = 0
		default:
			fullLayout = (oldLayout & 31) | 128
		}
	}
	f.height = height
	f.smushMode = fullLayout

	for i := 0; i < commentLines; i++ {
		if _, err := pop(); err != nil {
			return err
		}
	}

	loadChar := func() (int, [][]rune, error) {
		var end rune
		haveEnd := false
		width := 0
		var rows [][]rune
		for k := 0; k < height; k++ {
			line, err := pop()
			if err != nil {
				return 0, nil, err
			}
			if !haveEnd {
				// reEndMarker = (.)\s*$ : leftmost char followed only by whitespace
				found := false
				for i := 0; i < len(line); i++ {
					if line[i] != '\n' && allSpace(line[i+1:]) {
						end, found = line[i], true
						break
					}
				}
				if !found {
					return 0, nil, errors.New("no end marker")
				}
				haveEnd = true
			}
			line = cutEndMarker(line, end)
			if len(line) > width {
				width = len(line)
			}
			rows = append(rows, line)
		}
		return width, rows, nil
	}
	empty := func(rows [][]rune) bool {
		for _, r := range rows {
			if len(r) > 0 {
				return false
			}
		}
		return true
	}

	// ASCII 32..126
	for i := int64(32); i < 127; i++ {
		w, letter, err := loadChar()
		if err != nil {
			return err
		}
		if i == 32 || !empty(letter) {
			f.chars[i] = letter
			f.width[i] = w
		}
	}
	// German characters directly after 126
	if len(data) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, letter, err := loadChar()
			if err != nil {
				return err
			}
			if !empty(letter) {
				f.chars[int64(c)] = letter
				f.width[int64(c)] = w
			}
		}
	}
	// Extended characters tagged with 0x codes
	for len(data) > 0 {
		line, _ := pop()
		line = pyStrip(line)
		code := line
		for k, r := range line {
			if r == ' ' {
				code = line[:k]
				break
			}
		}
		if len(code) == 0 {
			continue
		}
		if len(code) >= 2 && code[0] == '0' && (code[1] == 'x' || code[1] == 'X') {
			n, err := pyInt(code, 16)
			if err != nil {
				return err
			}
			w, letter, err := loadChar()
			if err != nil {
				return err
			}
			if !empty(letter) {
				f.chars[int64(n)] = letter
				f.width[int64(n)] = w
			}
		}
	}
	return nil
}

// cutEndMarker is re.sub(re.escape(end) + r'{1,2}\s*$', '', line).
func cutEndMarker(line []rune, end rune) []rune {
	for i := 0; i < len(line); i++ {
		if line[i] != end {
			continue
		}
		if allSpace(line[i+1:]) || (i+1 < len(line) && line[i+1] == end && allSpace(line[i+2:])) {
			return line[:i]
		}
	}
	return line
}

// pyInt is int(s, base) for the forms that appear in font files.
func pyInt(s []rune, base int) (int, error) {
	t := strings.ReplaceAll(string(pyStrip(s)), "_", "")
	if base == 16 && len(t) >= 2 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X') {
		t = t[2:]
	}
	n, err := strconv.ParseInt(t, base, 64)
	if err != nil {
		var ne *strconv.NumError
		if errors.As(err, &ne) && ne.Err == strconv.ErrRange {
			return -1, nil // Python has big ints; such a code can never be typed anyway
		}
		return 0, fmt.Errorf("invalid literal for int() with base %d: %q", base, string(s))
	}
	return int(n), nil
}
