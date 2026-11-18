package main

// Port of pyfiglet's FigletFont (preloadFont + loadFont).

import (
	"archive/zip"
	"bytes"
	"embed"
	"errors"
	"io"
	"strconv"
	"strings"
)

//go:embed fonts
var fontFS embed.FS

var errFontNotFound = errors.New("font not found")

type fontError struct{ msg string }

func (e fontError) Error() string { return e.msg }

type Font struct {
	name           string
	height         int
	hardBlank      []rune
	printDirection int // -1 when the header has no print direction (Python None)
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

// readFontData mirrors preloadFont: FONT.tlf, then FONT.flf; unzip if needed.
func readFontData(name string) ([]byte, error) {
	for _, ext := range []string{"tlf", "flf"} {
		data, err := fontFS.ReadFile("fonts/" + name + "." + ext)
		if err != nil {
			continue
		}
		if zr, zerr := zip.NewReader(bytes.NewReader(data), int64(len(data))); zerr == nil && len(zr.File) > 0 {
			f, err := zr.File[0].Open()
			if err != nil {
				return nil, err
			}
			defer f.Close()
			data, err = io.ReadAll(f)
			if err != nil {
				return nil, err
			}
		}
		if len(data) == 0 {
			break
		}
		return data, nil
	}
	return nil, errFontNotFound
}

func pyInt(s []rune) (int, error) {
	return strconv.Atoi(strings.TrimSpace(string(s)))
}

// endMarker mirrors re.search(r'(.)\s*$', line).group(1).
func endMarker(line []rune) (rune, bool) {
	for i := 0; i < len(line); i++ {
		rest := true
		for _, r := range line[i+1:] {
			if !pyIsSpace(r) {
				rest = false
				break
			}
		}
		if rest {
			return line[i], true
		}
	}
	return 0, false
}

// stripEnd mirrors re.sub(re.escape(end) + r'{1,2}\s*$', '', line).
func stripEnd(line []rune, end rune) []rune {
	allSpace := func(s []rune) bool {
		for _, r := range s {
			if !pyIsSpace(r) {
				return false
			}
		}
		return true
	}
	for i := 0; i < len(line); i++ {
		if line[i] != end {
			continue
		}
		if i+1 < len(line) && line[i+1] == end && allSpace(line[i+2:]) {
			return line[:i]
		}
		if allSpace(line[i+1:]) {
			return line[:i]
		}
	}
	return line
}

func loadFont(name string) (*Font, error) {
	raw, err := readFontData(name)
	if err != nil {
		return nil, err
	}
	text := decodeUTF8Replace(raw)
	for i, r := range text {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			text[i] = ' '
		}
	}
	data := splitLines(text)
	bad := func(msg string) (*Font, error) {
		return nil, fontError{"problem parsing " + name + " font: " + msg}
	}
	pop := func() ([]rune, bool) {
		if len(data) == 0 {
			return nil, false
		}
		l := data[0]
		data = data[1:]
		return l, true
	}

	header, ok := pop()
	if !ok {
		return bad("empty")
	}
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') ||
		header[1] != 'l' || header[2] != 'f' || header[3] != '2' {
		return bad(name + " is not a valid figlet font")
	}
	fields := splitWS(header[5:])
	if len(fields) < 6 {
		return bad("malformed header")
	}
	f := &Font{name: name, printDirection: -1, chars: map[int][][]rune{}, width: map[int]int{}}
	f.hardBlank = fields[0]
	nums := make([]int, 5)
	for i := 0; i < 5; i++ {
		if nums[i], err = pyInt(fields[1+i]); err != nil {
			return bad("bad header number")
		}
	}
	f.height = nums[0]
	oldLayout, commentLines := nums[3], nums[4]
	fullLayout, haveFull := 0, false
	if len(fields) > 6 {
		if f.printDirection, err = pyInt(fields[6]); err != nil {
			return bad("bad print direction")
		}
	}
	if len(fields) > 7 {
		if fullLayout, err = pyInt(fields[7]); err != nil {
			return bad("bad full layout")
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
	f.smushMode = fullLayout

	for i := 0; i < commentLines; i++ {
		if _, ok := pop(); !ok {
			return bad("pop from empty list")
		}
	}

	readChar := func() (int, [][]rune, bool) {
		width := 0
		var rows [][]rune
		var end rune
		haveEnd := false
		for i := 0; i < f.height; i++ {
			line, ok := pop()
			if !ok {
				return 0, nil, false
			}
			if !haveEnd {
				if end, haveEnd = endMarker(line); !haveEnd {
					return 0, nil, false
				}
			}
			line = stripEnd(line, end)
			if len(line) > width {
				width = len(line)
			}
			rows = append(rows, line)
		}
		return width, rows, true
	}
	nonEmpty := func(rows [][]rune) bool {
		for _, r := range rows {
			if len(r) > 0 {
				return true
			}
		}
		return false
	}

	for i := 32; i < 127; i++ {
		w, rows, ok := readChar()
		if !ok {
			return bad("truncated character data")
		}
		if i == 32 || nonEmpty(rows) {
			f.chars[i] = rows
			f.width[i] = w
		}
	}
	if len(data) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, rows, ok := readChar()
			if !ok {
				return bad("truncated character data")
			}
			if nonEmpty(rows) {
				f.chars[int(c)] = rows
				f.width[int(c)] = w
			}
		}
	}
	for len(data) > 0 {
		line, _ := pop()
		line = stripWS(line)
		code := line
		for j, r := range line {
			if r == ' ' {
				code = line[:j]
				break
			}
		}
		if len(code) == 0 {
			continue
		}
		if len(code) >= 2 && code[0] == '0' && (code[1] == 'x' || code[1] == 'X') {
			n, err := strconv.ParseInt(string(code[2:]), 16, 64)
			if err != nil {
				return bad("invalid literal for int()")
			}
			w, rows, ok := readChar()
			if !ok {
				return bad("truncated character data")
			}
			if nonEmpty(rows) {
				f.chars[int(n)] = rows
				f.width[int(n)] = w
			}
		}
	}
	return f, nil
}
