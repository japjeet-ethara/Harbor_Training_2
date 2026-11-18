package main

// FIGfont loading: a port of pyfiglet's FigletFont.preloadFont and
// FigletFont.loadFont.

import (
	"archive/zip"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
)

// The provided fonts are compiled into the binary, so it works from any
// working directory.
//
//go:embed pyfiglet/fonts/*.flf
var fontFS embed.FS

// Layout bits (FIGfont "full layout"), as in figlet 2.2.2.
const (
	smEqual     = 1   // rule 1: equal characters
	smLowline   = 2   // rule 2: underscore
	smHierarchy = 4   // rule 3: hierarchy
	smPair      = 8   // rule 4: opposite pair
	smBigX      = 16  // rule 5: big X
	smHardblank = 32  // rule 6: hardblank
	smKern      = 64  // fitting
	smSmush     = 128 // smushing
)

// errFontNotFound corresponds to pyfiglet.FontNotFound.
var errFontNotFound = errors.New("font not found")

// Font is a parsed FIGfont.
type Font struct {
	Name           string
	HardBlank      []rune // the header token; normally one character
	Height         int
	PrintDirection int  // valid only if HasPrintDir
	HasPrintDir    bool // header had a print-direction field
	SmushMode      int
	Chars          map[int64][][]rune // code point -> glyph rows
	Width          map[int64]int      // code point -> widest row
}

// hardBlankRune returns the hardblank as a single rune, or -1 when the
// header token is longer than one character (it then never equals a single
// sub-character, exactly as in Python).
func (f *Font) hardBlankRune() rune {
	if len(f.HardBlank) == 1 {
		return f.HardBlank[0]
	}
	return -1
}

// LoadFont finds a font by name (".tlf" then ".flf", as pyfiglet does) and
// parses it.
func LoadFont(name string) (*Font, error) {
	data, err := readFontData(name)
	if err != nil {
		return nil, err
	}
	return parseFont(name, decodeUTF8Replace(data))
}

func readFontData(name string) ([]byte, error) {
	var data []byte
	for _, ext := range []string{"tlf", "flf"} {
		b, err := fontFS.ReadFile("pyfiglet/fonts/" + name + "." + ext)
		if err == nil {
			data = b
			break
		}
	}
	// A zipped font: use the first member, like zipfile.ZipFile.namelist()[0].
	if len(data) > 0 {
		if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil && len(zr.File) > 0 {
			rc, err := zr.File[0].Open()
			if err != nil {
				return nil, err
			}
			data, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
		}
	}
	if len(data) == 0 {
		return nil, errFontNotFound
	}
	return data, nil
}

// parseFont is FigletFont.loadFont.
func parseFont(name string, text []rune) (*Font, error) {
	f, err := doParseFont(name, text)
	if err != nil {
		return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
	}
	return f, nil
}

func doParseFont(name string, text []rune) (*Font, error) {
	// Like figlet, NEL, LINE SEPARATOR and PARAGRAPH SEPARATOR are not line
	// breaks in a font file.
	clean := make([]rune, len(text))
	for i, r := range text {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			r = ' '
		}
		clean[i] = r
	}
	lines := pySplitLines(clean)
	pos := 0
	pop := func() ([]rune, error) {
		if pos >= len(lines) {
			return nil, errors.New("pop from empty list")
		}
		pos++
		return lines[pos-1], nil
	}

	header, err := pop()
	if err != nil {
		return nil, err
	}
	// reMagicNumber = ^[tf]lf2.
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || string(header[1:4]) != "lf2" {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}
	fields := pySplitWhitespace(header[5:])
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}

	f := &Font{
		Name:      name,
		HardBlank: fields[0],
		Chars:     map[int64][][]rune{},
		Width:     map[int64]int{},
	}
	var nums [5]int64
	for i := range nums {
		if nums[i], err = pyInt(fields[1+i], 10); err != nil {
			return nil, err
		}
	}
	height, oldLayout, commentLines := int(nums[0]), nums[3], int(nums[4])
	var fullLayout int64
	hasFull := false
	if len(fields) > 6 {
		pd, err := pyInt(fields[6], 10)
		if err != nil {
			return nil, err
		}
		f.PrintDirection, f.HasPrintDir = int(pd), true
	}
	if len(fields) > 7 {
		if fullLayout, err = pyInt(fields[7], 10); err != nil {
			return nil, err
		}
		hasFull = true
	}
	// Derive the full layout from the old layout when it is missing.
	if !hasFull {
		switch {
		case oldLayout == 0:
			fullLayout = smKern
		case oldLayout < 0:
			fullLayout = 0
		default:
			fullLayout = (oldLayout & 31) | smSmush
		}
	}
	f.Height = height
	f.SmushMode = int(fullLayout)

	for i := 0; i < commentLines; i++ {
		if _, err := pop(); err != nil {
			return nil, err
		}
	}

	// readChar is the nested __char(): it reads `height` rows and removes
	// the end marks. The end mark is the last non-blank character of the
	// glyph's FIRST row (regex `(.)\s*$`) and is then stripped from every
	// row (regex `<end>{1,2}\s*$`).
	readChar := func() (int, [][]rune, error) {
		rows := make([][]rune, 0, max(height, 0))
		width := 0
		var end rune
		haveEnd := false
		for k := 0; k < height; k++ {
			line, err := pop()
			if err != nil {
				return 0, nil, err
			}
			if !haveEnd {
				if len(line) == 0 {
					return 0, nil, errors.New("'NoneType' object has no attribute 'group'")
				}
				end = line[endMarkIndex(line)]
				haveEnd = true
			}
			line = stripEndMarks(line, end)
			if len(line) > width {
				width = len(line)
			}
			rows = append(rows, line)
		}
		return width, rows, nil
	}
	store := func(code int64, width int, rows [][]rune) {
		f.Chars[code] = rows
		f.Width[code] = width
	}

	// ASCII 32..126. The space is always kept: the line-breaking logic
	// depends on it.
	for code := int64(32); code < 127; code++ {
		width, rows, err := readChar()
		if err != nil {
			return nil, err
		}
		if code == 32 || !allEmpty(rows) {
			store(code, width, rows)
		}
	}

	// The seven German characters follow directly after ASCII 126.
	if pos < len(lines) {
		for _, r := range "ÄÖÜäöüß" {
			width, rows, err := readChar()
			if err != nil {
				return nil, err
			}
			if !allEmpty(rows) {
				store(int64(r), width, rows)
			}
		}
	}

	// Code-tagged characters. Only hexadecimal tags ("0x...") are used;
	// every other line, including the rows of skipped glyphs, is ignored.
	for pos < len(lines) {
		line, _ := pop()
		line = pyStrip(line)
		tag := line
		for i, r := range line {
			if r == ' ' {
				tag = line[:i]
				break
			}
		}
		if len(tag) == 0 {
			continue
		}
		if len(tag) >= 2 && tag[0] == '0' && (tag[1] == 'x' || tag[1] == 'X') {
			code, err := pyInt(tag, 16)
			if err != nil {
				return nil, err
			}
			width, rows, err := readChar()
			if err != nil {
				return nil, err
			}
			if !allEmpty(rows) {
				store(code, width, rows)
			}
		}
	}
	return f, nil
}

// endMarkIndex is the position matched by `(.)\s*$`: the last non-blank
// character, or the first character if the line is all blank.
func endMarkIndex(line []rune) int {
	for i := len(line) - 1; i >= 0; i-- {
		if !pyIsSpace(line[i]) {
			return i
		}
	}
	return 0
}

// stripEndMarks is re.sub(escape(end) + r'{1,2}\s*$', ”, line): cut the
// line at the leftmost position where one or two end marks are followed
// only by whitespace.
func stripEndMarks(line []rune, end rune) []rune {
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

func allEmpty(rows [][]rune) bool {
	for _, r := range rows {
		if len(r) > 0 {
			return false
		}
	}
	return true
}
