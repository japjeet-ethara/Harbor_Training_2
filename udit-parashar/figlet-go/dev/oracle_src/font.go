package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Port of pyfiglet.FigletFont (parsing only). Rows are []rune because Python indexes by code point.

const (
	smEqual     = 1
	smLowline   = 2
	smHierarchy = 4
	smPair      = 8
	smBigX      = 16
	smHardblank = 32
	smKern      = 64
	smSmush     = 128
)

type Font struct {
	name           string
	height         int
	hardBlank      string
	printDirection int // -1 means "not present" (Python None)
	smushMode      int
	chars          map[rune][][]rune
	width          map[rune]int
}

// pyIsSpace mirrors str.isspace() / the Unicode-aware \s and str.strip() in Python.
func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

func pyStrip(s string) string  { return strings.TrimFunc(s, pyIsSpace) }
func pyRstrip(s string) string { return strings.TrimRightFunc(s, pyIsSpace) }

// pySplitLines mirrors str.splitlines() (keepends=False).
func pySplitLines(s string) []string {
	var lines []string
	rs := []rune(s)
	start := 0
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			lines = append(lines, string(rs[start:i]))
			start = i + 1
		case '\r':
			lines = append(lines, string(rs[start:i]))
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	if start < len(rs) {
		lines = append(lines, string(rs[start:]))
	}
	return lines
}

// decodeReplace mirrors bytes.decode('UTF-8', 'replace') (one U+FFFD per invalid byte).
func decodeReplace(b []byte) string {
	var sb strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size <= 1 {
			sb.WriteRune(utf8.RuneError)
			b = b[1:]
			continue
		}
		sb.WriteRune(r)
		b = b[size:]
	}
	return sb.String()
}

func allSpace(rs []rune) bool {
	for _, r := range rs {
		if !pyIsSpace(r) {
			return false
		}
	}
	return true
}

// findEndMarker mirrors re.search(r'(.)\s*$', line).group(1).
func findEndMarker(line []rune) (rune, bool) {
	for p := 0; p < len(line); p++ {
		if allSpace(line[p+1:]) {
			return line[p], true
		}
	}
	return 0, false
}

// stripEndMarker mirrors re.compile(re.escape(E) + r'{1,2}\s*$').sub('', line).
func stripEndMarker(line []rune, e rune) []rune {
	for p := 0; p < len(line); p++ {
		for j := 2; j >= 1; j-- {
			if p+j > len(line) {
				continue
			}
			ok := true
			for k := p; k < p+j; k++ {
				if line[k] != e {
					ok = false
					break
				}
			}
			if ok && allSpace(line[p+j:]) {
				return line[:p]
			}
		}
	}
	return line
}

func parseFont(name string, raw []byte) (f *Font, err error) {
	defer func() {
		if r := recover(); r != nil {
			f, err = nil, fmt.Errorf("problem parsing %s font: %v", name, r)
		}
	}()
	data := decodeReplace(raw)
	data = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(data)
	lines := pySplitLines(data)
	if len(lines) == 0 {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}
	header := []rune(lines[0])
	lines = lines[1:]
	if !(len(header) >= 5 && (header[0] == 't' || header[0] == 'f') && header[1] == 'l' && header[2] == 'f' && header[3] == '2') {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}
	fields := strings.FieldsFunc(string(header[5:]), pyIsSpace)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	atoi := func(s string) int {
		n, e := strconv.Atoi(s)
		if e != nil {
			panic(e)
		}
		return n
	}
	hardBlank := fields[0]
	height := atoi(fields[1])
	oldLayout := atoi(fields[4])
	commentLines := atoi(fields[5])
	printDirection := -1
	fullLayout := 0
	haveFull := false
	if len(fields) > 6 {
		printDirection = atoi(fields[6])
	}
	if len(fields) > 7 {
		fullLayout = atoi(fields[7])
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
	f = &Font{
		name: name, height: height, hardBlank: hardBlank, printDirection: printDirection,
		smushMode: fullLayout, chars: map[rune][][]rune{}, width: map[rune]int{},
	}

	pos := 0
	pop := func() string {
		if pos >= len(lines) {
			panic("pop from empty list")
		}
		s := lines[pos]
		pos++
		return s
	}
	for i := 0; i < commentLines; i++ {
		pop()
	}

	readChar := func() (int, [][]rune) {
		var end rune
		haveEnd := false
		width := 0
		rows := make([][]rune, 0, height)
		for k := 0; k < height; k++ {
			line := []rune(pop())
			if !haveEnd {
				e, ok := findEndMarker(line)
				if !ok {
					panic("no end marker")
				}
				end, haveEnd = e, true
			}
			line = stripEndMarker(line, end)
			if len(line) > width {
				width = len(line)
			}
			rows = append(rows, line)
		}
		return width, rows
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
		w, letter := readChar()
		if i == 32 || nonEmpty(letter) {
			f.chars[rune(i)] = letter
			f.width[rune(i)] = w
		}
	}
	if pos < len(lines) {
		for _, r := range "ÄÖÜäöüß" {
			w, letter := readChar()
			if nonEmpty(letter) {
				f.chars[r] = letter
				f.width[r] = w
			}
		}
	}
	for pos < len(lines) {
		line := pyStrip(pop())
		tok := line
		if idx := strings.IndexByte(line, ' '); idx >= 0 {
			tok = line[:idx]
		}
		if tok == "" {
			continue
		}
		if len(tok) >= 2 && tok[0] == '0' && (tok[1] == 'x' || tok[1] == 'X') {
			code, e := strconv.ParseInt(tok[2:], 16, 64)
			if e != nil {
				panic(e)
			}
			w, letter := readChar()
			if nonEmpty(letter) {
				f.chars[rune(code)] = letter
				f.width[rune(code)] = w
			}
		}
	}
	return f, nil
}
