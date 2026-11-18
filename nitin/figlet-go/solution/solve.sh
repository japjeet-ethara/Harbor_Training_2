#!/bin/bash
# Oracle solution: a Go port of pyfiglet's FIGlet rendering (standard library only).
# Fonts are moved into /workspace/repo/fonts and embedded with go:embed; every .py file is deleted.
set -euo pipefail
cd /workspace/repo

mkdir -p fonts
cp pyfiglet/fonts-standard/*.flf fonts/

# The Python is gone: the package directory (code + both font folders) and every other .py file.
rm -rf pyfiglet
find . -name '*.py' -delete

cat > go.mod <<'GO_EOF'
module figlet

go 1.22
GO_EOF

cat > maps.go <<'GO_EOF'
package main

// Generated from pyfiglet's FigletString.__reverse_map__ / __flip_map__
// (only the characters that change are listed; every other character maps to itself).

var reverseMap = map[rune]rune{
	40:  41,
	41:  40,
	47:  92,
	60:  62,
	62:  60,
	91:  93,
	92:  47,
	93:  91,
	123: 125,
	125: 123,
}

var flipMap = map[rune]rune{
	47:  92,
	65:  86,
	77:  87,
	80:  98,
	82:  98,
	86:  65,
	87:  77,
	92:  47,
	94:  118,
	95:  45,
	98:  80,
	109: 119,
	118: 94,
	119: 109,
}
GO_EOF

cat > main.go <<'GO_EOF'
// figlet: a Go port of pyfiglet's FIGlet rendering (pyfiglet commit 255827e).
// Standard library only. Fonts are embedded from ./fonts.
package main

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed fonts
var fontFS embed.FS

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

// ---------------------------------------------------------------- python helpers

// pyIsSpace mirrors Python's str.isspace() / regex \s for a single code point.
func pyIsSpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0d, r >= 0x1c && r <= 0x20:
		return true
	case r == 0x85, r == 0xa0, r == 0x1680, r >= 0x2000 && r <= 0x200a,
		r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', 0x0b, 0x0c, 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pySplitLines mirrors Python's str.splitlines().
func pySplitLines(s []rune) [][]rune {
	var out [][]rune
	start := 0
	for i := 0; i < len(s); i++ {
		if isLineBreak(s[i]) {
			out = append(out, s[start:i])
			if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func pyStrip(s []rune) []rune {
	a, b := 0, len(s)
	for a < b && pyIsSpace(s[a]) {
		a++
	}
	for b > a && pyIsSpace(s[b-1]) {
		b--
	}
	return s[a:b]
}

func pyRstrip(s []rune) []rune {
	b := len(s)
	for b > 0 && pyIsSpace(s[b-1]) {
		b--
	}
	return s[:b]
}

// pyDecode mirrors bytes.decode("UTF-8", "replace"): one U+FFFD per maximal invalid subpart.
func pyDecode(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); {
		r, n := utf8.DecodeRune(b[i:])
		if r != utf8.RuneError || n > 1 {
			sb.WriteRune(r)
			i += n
			continue
		}
		// invalid: work out the length of the longest valid prefix
		c := b[i]
		need := 0
		lo, hi := byte(0x80), byte(0xbf)
		switch {
		case c >= 0xc2 && c <= 0xdf:
			need = 1
		case c == 0xe0:
			need, lo = 2, 0xa0
		case c >= 0xe1 && c <= 0xec, c == 0xee, c == 0xef:
			need = 2
		case c == 0xed:
			need, hi = 2, 0x9f
		case c == 0xf0:
			need, lo = 3, 0x90
		case c >= 0xf1 && c <= 0xf3:
			need = 3
		case c == 0xf4:
			need, hi = 3, 0x8f
		}
		used := 1
		for k := 1; k <= need && i+k < len(b); k++ {
			l, h := byte(0x80), byte(0xbf)
			if k == 1 {
				l, h = lo, hi
			}
			if b[i+k] < l || b[i+k] > h {
				break
			}
			used++
		}
		sb.WriteRune(utf8.RuneError)
		i += used
	}
	return sb.String()
}

// ---------------------------------------------------------------- font

type font struct {
	height         int
	hardBlank      string
	printDirection int // -1 when absent
	smushMode      int
	chars          map[rune][][]rune
	width          map[rune]int
}

type fontError struct{ msg string }

func fail(format string, a ...interface{}) {
	panic(fontError{fmt.Sprintf(format, a...)})
}

func loadFontData(name string) (string, bool) {
	for _, ext := range []string{"tlf", "flf"} {
		b, err := fontFS.ReadFile("fonts/" + name + "." + ext)
		if err == nil {
			return pyDecode(b), true
		}
	}
	return "", false
}

// stripEndMarker mirrors the per-character regex handling in pyfiglet's __char().
func stripEndMarker(line []rune, end *rune, haveEnd *bool) []rune {
	if !*haveEnd {
		// re.search(r'(.)\s*$', line).group(1): leftmost char followed only by whitespace
		last := -1
		for i := len(line) - 1; i >= 0; i-- {
			if !pyIsSpace(line[i]) {
				last = i
				break
			}
		}
		if len(line) == 0 {
			fail("empty line where character data was expected")
		}
		if last < 0 {
			last = 0
		}
		*end = line[last]
		*haveEnd = true
	}
	e := *end
	// end.sub('', line) with end = E{1,2}\s*$ : first (leftmost) match position
	for p := 0; p < len(line); p++ {
		run := 0
		for p+run < len(line) && line[p+run] == e {
			run++
		}
		for k := min(2, run); k >= 1; k-- {
			ok := true
			for q := p + k; q < len(line); q++ {
				if !pyIsSpace(line[q]) {
					ok = false
					break
				}
			}
			if ok {
				return line[:p]
			}
		}
	}
	return line
}

func parseFont(name string) (*font, bool) {
	raw, ok := loadFontData(name)
	if !ok {
		return nil, false
	}
	f := &font{chars: map[rune][][]rune{}, width: map[rune]int{}, printDirection: -1}

	// remove unicode line separators other than CRLF-style ones
	var rs []rune
	for _, r := range raw {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			r = ' '
		}
		rs = append(rs, r)
	}
	data := pySplitLines(rs)
	pop := func() []rune {
		if len(data) == 0 {
			fail("unexpected end of font")
		}
		l := data[0]
		data = data[1:]
		return l
	}

	header := pop()
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || header[1] != 'l' || header[2] != 'f' || header[3] != '2' {
		fail("%s is not a valid figlet font", name)
	}
	fields := strings.FieldsFunc(string(header[5:]), pyIsSpace)
	if len(fields) < 6 {
		fail("malformed header for %s", name)
	}
	atoi := func(s string) int {
		v, err := strconv.Atoi(s)
		if err != nil {
			fail("bad header number %q", s)
		}
		return v
	}
	f.hardBlank = fields[0]
	height := atoi(fields[1])
	oldLayout := atoi(fields[4])
	commentLines := atoi(fields[5])
	_ = atoi(fields[2])
	_ = atoi(fields[3])
	fullLayout := -1
	hasFull := false
	if len(fields) > 6 {
		f.printDirection = atoi(fields[6])
	}
	if len(fields) > 7 {
		fullLayout = atoi(fields[7])
		hasFull = true
	}
	if !hasFull {
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
		pop()
	}

	readChar := func() (int, [][]rune) {
		var end rune
		have := false
		w := 0
		rows := make([][]rune, 0, height)
		for i := 0; i < height; i++ {
			line := pop()
			line = stripEndMarker(line, &end, &have)
			if len(line) > w {
				w = len(line)
			}
			rows = append(rows, line)
		}
		return w, rows
	}
	nonEmpty := func(rows [][]rune) bool {
		for _, r := range rows {
			if len(r) > 0 {
				return true
			}
		}
		return false
	}

	for i := rune(32); i < 127; i++ {
		w, letter := readChar()
		if i == 32 || nonEmpty(letter) {
			f.chars[i] = letter
			f.width[i] = w
		}
	}
	if len(data) > 0 {
		for _, i := range "ÄÖÜäöüß" {
			w, letter := readChar()
			if nonEmpty(letter) {
				f.chars[i] = letter
				f.width[i] = w
			}
		}
	}
	for len(data) > 0 {
		line := string(pyStrip(data[0]))
		data = data[1:]
		tag := line
		if k := strings.IndexByte(line, ' '); k >= 0 {
			tag = line[:k]
		}
		if tag == "" {
			continue
		}
		if len(tag) >= 2 && tag[0] == '0' && (tag[1] == 'x' || tag[1] == 'X') {
			v, err := strconv.ParseInt(tag[2:], 16, 64)
			if err != nil {
				fail("bad code tag %q", tag)
			}
			w, letter := readChar()
			if nonEmpty(letter) {
				f.chars[rune(v)] = letter
				f.width[rune(v)] = w
			}
		}
	}
	return f, true
}

// ---------------------------------------------------------------- engine

type marker struct {
	buf [][]rune
	it  int
}

type builder struct {
	text      []rune
	direction string
	width     int
	font      *font
	justify   string

	iterator, maxSmush                         int
	curCharWidth, prevCharWidth, currentTotalW int
	blankMarkers                               []marker
	queue                                      [][][]rune
	buffer                                     [][]rune
}

func newBuilder(text string, f *font, direction string, width int, justify string) *builder {
	b := &builder{text: []rune(text), font: f, direction: direction, width: width, justify: justify}
	b.buffer = make([][]rune, f.height)
	return b
}

func copyBuf(buf [][]rune) [][]rune {
	c := make([][]rune, len(buf))
	copy(c, buf)
	return c
}

func (b *builder) charAt(i int) ([][]rune, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	c, ok := b.font.chars[b.text[i]]
	return c, ok
}

func (b *builder) widthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	w, ok := b.font.width[b.text[i]]
	return w, ok
}

func (b *builder) smushChars(left, right string) (string, bool) {
	hb := b.font.hardBlank
	if left == " " {
		return right, true
	}
	if right == " " {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return "", false
	}
	sm := b.font.smushMode
	if sm&smSmush == 0 {
		return "", false
	}
	if sm&63 == 0 {
		if left == hb {
			return right, true
		}
		if right == hb {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if sm&smHardblank != 0 {
		if left == hb && right == hb {
			return left, true
		}
	}
	if left == hb || right == hb {
		return "", false
	}
	if sm&smEqual != 0 {
		if left == right {
			return left, true
		}
	}
	type pair struct{ a, b string }
	var smushes []pair
	if sm&smLowline != 0 {
		smushes = append(smushes, pair{"_", `|/\[]{}()<>`})
	}
	if sm&smHierarchy != 0 {
		smushes = append(smushes,
			pair{"|", `/\[]{}()<>`},
			pair{`\/`, "[]{}()<>"},
			pair{"[]", "{}()<>"},
			pair{"{}", "()<>"},
			pair{"()", "<>"},
		)
	}
	for _, s := range smushes {
		if strings.Contains(s.a, left) && strings.Contains(s.b, right) {
			return right, true
		}
		if strings.Contains(s.a, right) && strings.Contains(s.b, left) {
			return left, true
		}
	}
	if sm&smPair != 0 {
		for _, p := range []string{left + right, right + left} {
			if p == "[]" || p == "{}" || p == "()" {
				return "|", true
			}
		}
	}
	if sm&smBigX != 0 {
		if left == "/" && right == `\` {
			return "|", true
		}
		if right == "/" && left == `\` {
			return "Y", true
		}
		if left == ">" && right == "<" {
			return "X", true
		}
	}
	return "", false
}

func (b *builder) smushAmount(buffer [][]rune, curChar [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}
		// only ASCII space is stripped
		t := len(lineLeft)
		for t > 0 && lineLeft[t-1] == ' ' {
			t--
		}
		linebd := t - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := ""
		if linebd < len(lineLeft) {
			ch1 = string(lineLeft[linebd])
		} else {
			linebd = 0
		}
		s := 0
		for s < len(lineRight) && lineRight[s] == ' ' {
			s++
		}
		charbd := s
		ch2 := ""
		if charbd < len(lineRight) {
			ch2 = string(lineRight[charbd])
		} else {
			charbd = len(lineRight)
		}
		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == "" || ch1 == " " {
			amt++
		} else if ch2 != "" {
			if _, ok := b.smushChars(ch1, ch2); ok {
				amt++
			}
		}
		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	if b.maxSmush > 0 {
		addLeft = append([]rune(nil), addLeft...) // never mutate shared rows
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := ""
		if idx >= 0 && idx < len(addLeft) {
			left = string(addLeft[idx])
		}
		if i >= len(addRight) {
			fail("IndexError: smush index out of range")
		}
		right := string(addRight[i])
		smushed, ok := b.smushChars(left, right)
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if idx == len(addLeft) {
			fail("IndexError: list assignment index out of range")
		}
		if !ok {
			fail("TypeError: cannot smush these characters")
		}
		sr := []rune(smushed)
		if len(sr) == 1 {
			addLeft[idx] = sr[0]
		} else {
			nl := append([]rune(nil), addLeft[:idx]...)
			nl = append(nl, sr...)
			nl = append(nl, addLeft[idx+1:]...)
			addLeft = nl
		}
	}
	return addLeft, addRight
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	var tail []rune
	if b.maxSmush < len(addRight) {
		tail = addRight[b.maxSmush:]
	}
	nr := make([]rune, 0, len(addLeft)+len(tail))
	nr = append(nr, addLeft...)
	nr = append(nr, tail...)
	b.buffer[row] = nr
}

func (b *builder) cutBufferCommon() {
	b.currentTotalW = 0
	b.buffer = make([][]rune, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, ok := b.charAt(b.iterator)
	if !ok {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.queue = append(b.queue, m.buf)
		b.iterator = m.it
		b.cutBufferCommon()
		return
	}
	b.queue = append(b.queue, b.buffer)
	b.iterator--
	b.cutBufferCommon()
	if len(b.queue) > 200000 {
		fail("line cannot be wrapped at this width")
	}
}

func (b *builder) addCharToProduct() {
	curChar, have := b.charAt(b.iterator)
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if !have {
		return
	}
	w, _ := b.widthAt(b.iterator)
	if b.width < w {
		fail("Width is not enough to print this character")
	}
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalW = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
	}
	if b.currentTotalW >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}
	b.prevCharWidth = b.curCharWidth
}

func (b *builder) justifyString(buffer [][]rune) {
	switch b.justify {
	case "right":
		for row := 0; row < b.font.height; row++ {
			n := b.width - len(buffer[row]) - 1
			buffer[row] = append(spaces(n), buffer[row]...)
		}
	case "center":
		for row := 0; row < b.font.height; row++ {
			n := (b.width - len(buffer[row])) / 2 // truncates toward zero, like int(x/2)
			buffer[row] = append(spaces(n), buffer[row]...)
		}
	}
}

func spaces(n int) []rune {
	if n < 0 {
		n = 0
	}
	s := make([]rune, n)
	for i := range s {
		s[i] = ' '
	}
	return s
}

func (b *builder) result() []rune {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var acc strings.Builder
	for _, buf := range b.queue {
		buf = copyBuf(buf)
		b.justifyString(buf)
		var sb strings.Builder
		for _, row := range buf {
			sb.WriteString(string(row))
			sb.WriteByte('\n')
		}
		acc.WriteString(strings.ReplaceAll(sb.String(), b.font.hardBlank, " "))
	}
	return []rune(acc.String())
}

func render(text string, f *font, direction string, width int, justify string) []rune {
	b := newBuilder(text, f, direction, width, justify)
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	return b.result()
}

// ---------------------------------------------------------------- post-processing

func newFromList(rows [][]rune) []rune {
	var out []rune
	for i, r := range rows {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, r...)
	}
	return append(out, '\n')
}

func translate(row []rune, m map[rune]rune) []rune {
	o := make([]rune, len(row))
	for i, r := range row {
		if t, ok := m[r]; ok {
			o[i] = t
		} else {
			o[i] = r
		}
	}
	return o
}

func reverse(s []rune) []rune {
	var out [][]rune
	for _, row := range pySplitLines(s) {
		t := translate(row, reverseMap)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		out = append(out, t)
	}
	return newFromList(out)
}

func flip(s []rune) []rune {
	rows := pySplitLines(s)
	var out [][]rune
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translate(rows[i], flipMap))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s []rune) []rune {
	var out [][]rune
	seen := false
	for _, row := range pySplitLines(s) {
		if len(pyStrip(row)) != 0 || seen {
			seen = true
			out = append(out, row)
		}
	}
	return pyRstrip(newFromList(out))
}

// ---------------------------------------------------------------- command line (optparse-like)

type options struct {
	font, direction, justify string
	width                    int
	reverse, norm, strip, fl bool
}

var longNames = []string{"font", "direction", "justify", "width", "reverse",
	"normalize-surrounding-newlines", "strip-surrounding-newlines", "flip"}

var shortToLong = map[byte]string{'f': "font", 'D': "direction", 'j': "justify", 'w': "width",
	'r': "reverse", 'n': "normalize-surrounding-newlines", 's': "strip-surrounding-newlines", 'F': "flip"}

func usageExit(msg string) {
	fmt.Fprintln(os.Stderr, "Usage: figlet [options] [text..]")
	fmt.Fprintln(os.Stderr, "figlet: error: "+msg)
	os.Exit(2)
}

func takesValue(long string) bool {
	switch long {
	case "font", "direction", "justify", "width":
		return true
	}
	return false
}

func (o *options) set(long, val string) {
	switch long {
	case "font":
		o.font = val
	case "direction":
		if val != "auto" && val != "left-to-right" && val != "right-to-left" {
			usageExit("option -D/--direction: invalid choice: '" + val + "'")
		}
		o.direction = val
	case "justify":
		if val != "auto" && val != "left" && val != "center" && val != "right" {
			usageExit("option -j/--justify: invalid choice: '" + val + "'")
		}
		o.justify = val
	case "width":
		n, err := strconv.Atoi(val)
		if err != nil {
			usageExit("option -w/--width: invalid integer value: '" + val + "'")
		}
		o.width = n
	case "reverse":
		o.reverse = true
	case "normalize-surrounding-newlines":
		o.norm = true
	case "strip-surrounding-newlines":
		o.strip = true
	case "flip":
		o.fl = true
	}
}

func parseArgs(args []string) (*options, []string) {
	o := &options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			pos = append(pos, args[i+1:]...)
			return o, pos
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := a[2:], "", false
			if k := strings.IndexByte(name, '='); k >= 0 {
				name, val, hasVal = name[:k], name[k+1:], true
			}
			long := ""
			var cands []string
			for _, l := range longNames {
				if l == name {
					cands = []string{l}
					break
				}
				if strings.HasPrefix(l, name) {
					cands = append(cands, l)
				}
			}
			if len(cands) != 1 {
				usageExit("no such option: --" + name)
			}
			long = cands[0]
			if takesValue(long) {
				if !hasVal {
					if i+1 >= len(args) {
						usageExit("option --" + long + " requires an argument")
					}
					i++
					val = args[i]
				}
			}
			o.set(long, val)
		case len(a) > 1 && a[0] == '-':
			for k := 1; k < len(a); k++ {
				long, ok := shortToLong[a[k]]
				if !ok {
					usageExit("no such option: -" + string(a[k]))
				}
				if takesValue(long) {
					val := a[k+1:]
					if val == "" {
						if i+1 >= len(args) {
							usageExit("option -" + string(a[k]) + " requires an argument")
						}
						i++
						val = args[i]
					}
					o.set(long, val)
					break
				}
				o.set(long, "")
			}
		default:
			pos = append(pos, a)
		}
	}
	return o, pos
}

func main() {
	opts, args := parseArgs(os.Args[1:])
	if len(args) == 0 {
		fmt.Println("Usage: figlet [options] [text..]")
		os.Exit(1)
	}
	text := strings.Join(args, " ")

	defer func() {
		if r := recover(); r != nil {
			if fe, ok := r.(fontError); ok {
				fmt.Fprintln(os.Stderr, "figlet error:", fe.msg)
				os.Exit(1)
			}
			panic(r)
		}
	}()

	f, ok := parseFont(opts.font)
	if !ok {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", opts.font)
		os.Exit(1)
	}

	direction := opts.direction
	if direction == "auto" {
		if f.printDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}
	justify := opts.justify
	if justify == "auto" {
		if direction == "left-to-right" {
			justify = "left"
		} else {
			justify = "right"
		}
	}

	r := render(text, f, direction, opts.width, justify)
	if opts.reverse {
		r = reverse(r)
	}
	if opts.fl {
		r = flip(r)
	}
	if opts.strip {
		r = stripSurroundingNewlines(r)
	} else if opts.norm {
		r = append(append([]rune{'\n'}, stripSurroundingNewlines(r)...), '\n')
	}

	w := bufio.NewWriter(os.Stdout)
	w.WriteString(string(r))
	w.WriteByte('\n')
	w.Flush()
}
GO_EOF
