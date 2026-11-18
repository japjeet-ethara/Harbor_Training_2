package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed fonts
var embeddedFonts embed.FS

var errFontNotFound = errors.New("font not found")

// Font is a port of pyfiglet.FigletFont.
type Font struct {
	name           string
	height         int
	hardBlank      string
	printDirection *int
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

// fontDirectories mirrors pyfiglet's FONT_DIRECTORIES on non-Windows systems.
var fontDirectories = []string{"/usr/local/share/pyfiglet/", "/usr/local/share/figlet", "/usr/share/figlet"}

// readFontFile mirrors FigletFont.preloadFont.
func readFontFile(font string) ([]byte, error) {
	var data []byte
	found := false
	for _, ext := range []string{"tlf", "flf"} {
		fn := font + "." + ext
		if b, err := embeddedFonts.ReadFile("fonts/" + fn); err == nil && !strings.Contains(fn, "/") {
			data, found = b, true
			break
		}
		for _, loc := range append([]string{"./"}, fontDirectories...) {
			full := filepath.Join(loc, fn)
			if st, err := os.Stat(full); err == nil && st.Mode().IsRegular() {
				if b, err := os.ReadFile(full); err == nil {
					data, found = b, true
				}
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return nil, errFontNotFound
	}
	if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil && len(zr.File) > 0 {
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
		return nil, errFontNotFound
	}
	return data, nil
}

// decodeReplace decodes UTF-8 replacing invalid bytes with U+FFFD.
func decodeReplace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		out = append(out, r)
		b = b[size:]
	}
	return out
}

// pyInt mirrors int(s) for the plain decimal header fields.
func pyInt(s string) (int, error) {
	return strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(s), "_", ""))
}

// findEndMarker mirrors re.search(r'(.)\s*$', line).group(1).
func findEndMarker(line []rune) (rune, error) {
	if len(line) == 0 {
		return 0, errors.New("no end marker")
	}
	q := len(pyRstrip(line))
	if q > 0 {
		return line[q-1], nil
	}
	return line[0], nil
}

// stripEndMarker mirrors re.sub(re.escape(end) + r'{1,2}\s*$', '', line).
func stripEndMarker(line []rune, end rune) []rune {
	allSpace := func(s []rune) bool {
		for _, r := range s {
			if !pyIsSpace(r) {
				return false
			}
		}
		return true
	}
	for p := 0; p < len(line); p++ {
		if line[p] != end {
			continue
		}
		if p+1 < len(line) && line[p+1] == end && allSpace(line[p+2:]) {
			return line[:p]
		}
		if allSpace(line[p+1:]) {
			return line[:p]
		}
	}
	return line
}

func loadFont(name string) (*Font, error) {
	raw, err := readFontFile(name)
	if err != nil {
		return nil, err
	}
	f := &Font{name: name, chars: map[int][][]rune{}, width: map[int]int{}}
	if err := f.parse(decodeReplace(raw)); err != nil {
		return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
	}
	return f, nil
}

func (f *Font) parse(data []rune) error {
	for i, r := range data {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			data[i] = ' '
		}
	}
	lines := pySplitlines(data)
	pop := func() ([]rune, error) {
		if len(lines) == 0 {
			return nil, errors.New("pop from empty list")
		}
		l := lines[0]
		lines = lines[1:]
		return l, nil
	}

	header, err := pop()
	if err != nil {
		return err
	}
	// reMagicNumber = ^[tf]lf2.
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || string(header[1:4]) != "lf2" || header[4] == '\n' {
		return fmt.Errorf("%s is not a valid figlet font", f.name)
	}
	fields := strings.FieldsFunc(string(header[5:]), pyIsSpace)
	if len(fields) < 6 {
		return fmt.Errorf("malformed header for %s", f.name)
	}
	f.hardBlank = fields[0]
	nums := make([]int, 5)
	for i := 0; i < 5; i++ {
		if nums[i], err = pyInt(fields[1+i]); err != nil {
			return err
		}
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	var fullLayout *int
	if len(fields) > 6 {
		v, err := pyInt(fields[6])
		if err != nil {
			return err
		}
		f.printDirection = &v
	}
	if len(fields) > 7 {
		v, err := pyInt(fields[7])
		if err != nil {
			return err
		}
		fullLayout = &v
	}
	if fullLayout == nil {
		var v int
		switch {
		case oldLayout == 0:
			v = 64
		case oldLayout < 0:
			v = 0
		default:
			v = (oldLayout & 31) | 128
		}
		fullLayout = &v
	}
	f.height = height
	f.smushMode = *fullLayout

	for i := 0; i < commentLines; i++ {
		if _, err := pop(); err != nil {
			return err
		}
	}

	loadChar := func() (int, [][]rune, error) {
		var chars [][]rune
		width := 0
		haveEnd := false
		var end rune
		for i := 0; i < height; i++ {
			line, err := pop()
			if err != nil {
				return 0, nil, err
			}
			if !haveEnd {
				if end, err = findEndMarker(line); err != nil {
					return 0, nil, err
				}
				haveEnd = true
			}
			line = stripEndMarker(line, end)
			if len(line) > width {
				width = len(line)
			}
			chars = append(chars, line)
		}
		return width, chars, nil
	}
	empty := func(rows [][]rune) bool {
		for _, r := range rows {
			if len(r) > 0 {
				return false
			}
		}
		return true
	}

	for i := 32; i < 127; i++ {
		w, letter, err := loadChar()
		if err != nil {
			return err
		}
		if i == 32 || !empty(letter) {
			f.chars[i] = letter
			f.width[i] = w
		}
	}
	if len(lines) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, letter, err := loadChar()
			if err != nil {
				return err
			}
			if !empty(letter) {
				f.chars[int(c)] = letter
				f.width[int(c)] = w
			}
		}
	}
	for len(lines) > 0 {
		line, _ := pop()
		s := string(pyStrip(line))
		tok := s
		if k := strings.IndexByte(s, ' '); k >= 0 {
			tok = s[:k]
		}
		if tok == "" {
			continue
		}
		if len(tok) >= 2 && tok[0] == '0' && (tok[1] == 'x' || tok[1] == 'X') {
			code, err := strconv.ParseInt(strings.ReplaceAll(tok[2:], "_", ""), 16, 64)
			if err != nil {
				return err
			}
			w, letter, err := loadChar()
			if err != nil {
				return err
			}
			if !empty(letter) {
				f.chars[int(code)] = letter
				f.width[int(code)] = w
			}
		}
	}
	return nil
}
