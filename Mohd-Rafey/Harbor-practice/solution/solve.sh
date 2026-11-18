#!/bin/bash
# Reference solution: port pyfiglet's FIGlet rendering to Go (standard library
# only), embed the fonts, delete every .py file.
set -euo pipefail

REPO=/workspace/repo
cd "$REPO"

# ---------------------------------------------------------------- fonts ----
# Move the .flf fonts into fonts/ at the repository root; they are embedded
# into the binary with go:embed, so it runs from any working directory.
mkdir -p fonts
if ls pyfiglet/fonts/*.flf >/dev/null 2>&1; then
  mv pyfiglet/fonts/*.flf fonts/
else
  cp pyfiglet/fonts-standard/*.flf fonts/
fi

# --------------------------------------------------------------- go.mod ----
cat > go.mod <<'EOF'
module figlet

go 1.23
EOF

# -------------------------------------------------------------- main.go ----
cat > main.go <<'EOF'
// Command figlet is a Go port of pyfiglet (commit 255827ef). It follows
// pyfiglet/__init__.py line by line: FigletFont.loadFont, FigletBuilder
// (including every smushing rule in smushChars) and the FigletString
// transforms used by main(). Standard library only.
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
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

const defaultFont = "standard"

// Layout bits, lifted from figlet222 via pyfiglet.
const (
	smEqual     = 1   // smush equal chars (not hardblanks)
	smLowline   = 2   // smush _ with any char in hierarchy
	smHierarchy = 4   // hierarchy: |, /\, [], {}, (), <>
	smPair      = 8   // [ + ] -> |, { + } -> |, ( + ) -> |
	smBigX      = 16  // / + \ -> |, \ + / -> Y, > + < -> X
	smHardblank = 32  // hardblank + hardblank -> hardblank
	smKern      = 64
	smSmush     = 128
)

// noChar stands for Python's empty string '' where a character is expected.
const noChar rune = -1

// fatal mirrors an uncaught Python exception: message on stderr, no stdout.
type fatal struct{ msg string }

func die(format string, a ...any) { panic(fatal{fmt.Sprintf(format, a...)}) }

// ------------------------------------------------------ Python helpers ----

// pyIsSpace matches Python's str.isspace() (and re's \s for str patterns).
func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

func allSpace(s []rune) bool {
	for _, r := range s {
		if !pyIsSpace(r) {
			return false
		}
	}
	return true
}

func pyStrip(s []rune) []rune  { return pyLStrip(pyRStrip(s)) }
func pyRStrip(s []rune) []rune {
	for len(s) > 0 && pyIsSpace(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}
func pyLStrip(s []rune) []rune {
	for len(s) > 0 && pyIsSpace(s[0]) {
		s = s[1:]
	}
	return s
}

// rstrip(' ') / lstrip(' '): ASCII space only.
func rstripSpace(s []rune) []rune {
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}
func lstripSpace(s []rune) []rune {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}

// pyFields matches str.split() with no arguments.
func pyFields(s []rune) [][]rune {
	var out [][]rune
	start := -1
	for i, r := range s {
		if pyIsSpace(r) {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// splitLines matches str.splitlines().
func splitLines(s []rune) [][]rune {
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

// decodeUTF8Replace matches bytes.decode('UTF-8', 'replace'): each maximal
// ill-formed subpart becomes a single U+FFFD.
func decodeUTF8Replace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			out = append(out, rune(c))
			i++
			continue
		}
		need := 0
		lo, hi := byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c == 0xE0:
			need, lo = 2, 0xA0
		case (c >= 0xE1 && c <= 0xEC) || c == 0xEE || c == 0xEF:
			need = 2
		case c == 0xED:
			need, hi = 2, 0x9F
		case c == 0xF0:
			need, lo = 3, 0x90
		case c >= 0xF1 && c <= 0xF3:
			need = 3
		case c == 0xF4:
			need, hi = 3, 0x8F
		default:
			out = append(out, utf8.RuneError)
			i++
			continue
		}
		j, ok := i+1, true
		for k := 0; k < need; k++ {
			l, h := byte(0x80), byte(0xBF)
			if k == 0 {
				l, h = lo, hi
			}
			if j >= len(b) || b[j] < l || b[j] > h {
				ok = false
				break
			}
			j++
		}
		if !ok {
			out = append(out, utf8.RuneError)
			i = j
			continue
		}
		r, _ := utf8.DecodeRune(b[i:j])
		out = append(out, r)
		i = j
	}
	return out
}

func runesIn(r rune, set string) bool { // Python: r in set ('' in s is True)
	return r == noChar || strings.ContainsRune(set, r)
}

func runeStr(r rune) string {
	if r == noChar {
		return ""
	}
	return string(r)
}

// --------------------------------------------------------------- fonts ----

type Font struct {
	name           string
	height         int
	hardBlank      string
	printDirection int
	hasDirection   bool
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

var errFontNotFound = errors.New("font not found")

// readFontData follows FigletFont.preloadFont: for each extension, the
// package fonts (embedded here) win, otherwise the search directories; a
// later extension that is found overrides an earlier one.
func readFontData(name string) ([]byte, error) {
	dirs := []string{"./", "/usr/local/share/pyfiglet/", "/usr/local/share/figlet", "/usr/share/figlet"}
	var data []byte
	found := false
	for _, ext := range []string{"tlf", "flf"} {
		fn := name + "." + ext
		if b, err := embeddedFonts.ReadFile("fonts/" + fn); err == nil {
			data, found = b, true
			continue
		}
		for _, d := range dirs {
			p := filepath.Join(d, fn)
			if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
				b, err := os.ReadFile(p)
				if err != nil {
					return nil, err
				}
				data, found = b, true
				break
			}
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
		if data, err = io.ReadAll(f); err != nil {
			return nil, err
		}
	}
	if len(data) == 0 {
		return nil, errFontNotFound
	}
	return data, nil
}

// endMarker: reEndMarker = (.)\s*$ -> the last non-whitespace character, or
// the first character when the line is all whitespace.
func endMarker(line []rune) (rune, bool) {
	if len(line) == 0 {
		return 0, false
	}
	for i := len(line) - 1; i >= 0; i-- {
		if !pyIsSpace(line[i]) {
			return line[i], true
		}
	}
	return line[0], true
}

// stripEnd: re.sub(escape(end) + r'{1,2}\s*$', '', line).
func stripEnd(line []rune, end rune) []rune {
	n := len(line)
	for p := 0; p < n; p++ {
		if line[p] != end {
			continue
		}
		if allSpace(line[p+1:]) || (p+1 < n && line[p+1] == end && allSpace(line[p+2:])) {
			return line[:p]
		}
	}
	return line
}

func parsePyInt(s []rune) int {
	v, err := strconv.Atoi(strings.ReplaceAll(string(pyStrip(s)), "_", ""))
	if err != nil {
		die("invalid literal for int() with base 10: %q", string(s))
	}
	return v
}

func loadFont(name string) (*Font, error) {
	raw, err := readFontData(name)
	if err != nil {
		return nil, err
	}
	f := &Font{name: name, chars: map[int][][]rune{}, width: map[int]int{}}

	data := decodeUTF8Replace(raw)
	for i, r := range data {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			data[i] = ' '
		}
	}
	lines := splitLines(data)
	pop := func() []rune {
		if len(lines) == 0 {
			die("problem parsing %s font: pop from empty list", name)
		}
		l := lines[0]
		lines = lines[1:]
		return l
	}

	header := pop()
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') ||
		header[1] != 'l' || header[2] != 'f' || header[3] != '2' || header[4] == '\n' {
		die("%s is not a valid figlet font", name)
	}
	fields := pyFields(header[5:])
	if len(fields) < 6 {
		die("malformed header for %s", name)
	}
	f.hardBlank = string(fields[0])
	height := parsePyInt(fields[1])
	_ = parsePyInt(fields[2]) // baseLine
	_ = parsePyInt(fields[3]) // maxLength
	oldLayout := parsePyInt(fields[4])
	commentLines := parsePyInt(fields[5])
	fullLayout, hasFull := 0, false
	if len(fields) > 6 {
		f.printDirection, f.hasDirection = parsePyInt(fields[6]), true
	}
	if len(fields) > 7 {
		fullLayout, hasFull = parsePyInt(fields[7]), true
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
		width := 0
		rows := make([][]rune, 0, height)
		var end rune
		haveEnd := false
		for k := 0; k < height; k++ {
			line := pop()
			if !haveEnd {
				e, ok := endMarker(line)
				if !ok {
					die("problem parsing %s font: no end marker", name)
				}
				end, haveEnd = e, true
			}
			line = stripEnd(line, end)
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

	// ASCII 32..126 (space is always kept).
	for i := 32; i < 127; i++ {
		w, letter := readChar()
		if i == 32 || nonEmpty(letter) {
			f.chars[i], f.width[i] = letter, w
		}
	}
	// German characters follow directly.
	if len(lines) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, letter := readChar()
			if nonEmpty(letter) {
				f.chars[int(c)], f.width[int(c)] = letter, w
			}
		}
	}
	// Code-tagged characters: only hexadecimal codes (0x...) are loaded.
	for len(lines) > 0 {
		line := pyStrip(pop())
		first := line
		for k, r := range line {
			if r == ' ' {
				first = line[:k]
				break
			}
		}
		if len(first) == 0 {
			continue
		}
		if len(first) >= 2 && first[0] == '0' && (first[1] == 'x' || first[1] == 'X') {
			s := string(pyStrip(first))
			v, err := strconv.ParseInt(strings.ReplaceAll(s[2:], "_", ""), 16, 64)
			if err != nil || s[2:] == "" {
				die("problem parsing %s font: invalid literal for int() with base 16: %q", name, s)
			}
			w, letter := readChar()
			if nonEmpty(letter) {
				f.chars[int(v)], f.width[int(v)] = letter, w
			}
		}
	}
	return f, nil
}

// ------------------------------------------------------------- builder ----

type marker struct {
	buf [][]rune
	it  int
}

type builder struct {
	text      []rune
	direction string
	width     int
	font      *Font
	justify   string

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []marker
	queue        [][][]rune
	buffer       [][]rune
}

func newBuilder(text []rune, font *Font, direction string, width int, justify string) *builder {
	b := &builder{text: text, font: font, direction: direction, width: width, justify: justify}
	b.buffer = make([][]rune, font.height)
	return b
}

func copyBuf(buf [][]rune) [][]rune { return append([][]rune(nil), buf...) }

func (b *builder) getCharAt(i int) ([][]rune, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	c, ok := b.font.chars[int(b.text[i])]
	return c, ok
}

func (b *builder) addCharToProduct() {
	curChar, ok := b.getCharAt(b.iterator)
	c := b.text[b.iterator]

	if c == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if !ok {
		return
	}
	w := b.font.width[int(c)]
	if b.width < w {
		die("CharNotPrinted: Width is not enough to print this character")
	}
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if c == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
	}
	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}
	b.prevCharWidth = b.curCharWidth
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := noChar
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		right := addRight[i] // IndexError in Python -> panic here too
		smushed, ok := b.smushChars(left, right)
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if !ok || smushed == noChar {
			die("TypeError: sequence item %d: expected str instance, NoneType found", idx)
		}
		nl := append([]rune(nil), addLeft...)
		nl[idx] = smushed
		addLeft = nl
	}
	return addLeft, addRight
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	tail := []rune{}
	if b.maxSmush < len(addRight) {
		tail = addRight[b.maxSmush:]
	}
	nr := make([]rune, 0, len(addLeft)+len(tail))
	nr = append(nr, addLeft...)
	b.buffer[row] = append(nr, tail...)
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([][]rune, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	if cur, ok := b.getCharAt(b.iterator); ok {
		b.maxSmush = b.smushAmount(b.buffer, cur)
	}
}

func (b *builder) cutBufferAtLastBlank(saved [][]rune, it int) {
	b.queue = append(b.queue, saved)
	b.iterator = it
	b.cutBufferCommon()
}

func (b *builder) cutBufferAtLastChar() {
	empty := true
	for _, r := range b.buffer {
		if len(r) > 0 {
			empty = false
			break
		}
	}
	if empty {
		// pyfiglet re-tries the same character on an empty line forever.
		die("character does not fit in width %d (pyfiglet never terminates here)", b.width)
	}
	b.queue = append(b.queue, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.cutBufferAtLastBlank(m.buf, m.it)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *builder) justifyString(buf [][]rune) [][]rune {
	out := make([][]rune, len(buf))
	copy(out, buf)
	for row := 0; row < b.font.height; row++ {
		pad := 0
		switch b.justify {
		case "right":
			pad = b.width - len(buf[row]) - 1
		case "center":
			pad = (b.width - len(buf[row])) / 2 // int() truncates toward zero
		}
		if pad > 0 {
			nr := make([]rune, 0, pad+len(buf[row]))
			for k := 0; k < pad; k++ {
				nr = append(nr, ' ')
			}
			out[row] = append(nr, buf[row]...)
		}
	}
	return out
}

func (b *builder) replaceHardblanks(buf [][]rune) string {
	rows := make([]string, len(buf))
	for i, r := range buf {
		rows[i] = string(r)
	}
	return strings.ReplaceAll(strings.Join(rows, "\n")+"\n", b.font.hardBlank, " ")
}

func (b *builder) returnProduct() string {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		sb.WriteString(b.replaceHardblanks(b.justifyString(buf)))
	}
	return sb.String()
}

// smushAmount: how far the next character can slide left (pyfiglet's port of
// figlet's smushamt).
func (b *builder) smushAmount(buffer, curChar [][]rune) int {
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

		linebd := len(rstripSpace(lineLeft)) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := noChar
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}

		charbd := len(lineRight) - len(lstripSpace(lineRight))
		ch2 := noChar
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
		}

		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == noChar || ch1 == ' ' {
			amt++
		} else if ch2 != noChar {
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

func (b *builder) isHardBlank(r rune) bool {
	return r != noChar && string(r) == b.font.hardBlank
}

// smushChars ports FigletBuilder.smushChars rule by rule. ok=false is
// Python's None (cannot smush).
func (b *builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}

	// No overlapping if the previous or current character is narrower than 2.
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}

	mode := b.font.smushMode

	// Kerning only.
	if mode&smSmush == 0 {
		return 0, false
	}

	// Universal overlapping (no rules 1-32 set).
	if mode&63 == 0 {
		if b.isHardBlank(left) {
			return right, true
		}
		if b.isHardBlank(right) {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}

	// Rule 6: hardblank.
	if mode&smHardblank != 0 {
		if b.isHardBlank(left) && b.isHardBlank(right) {
			return left, true
		}
	}
	if b.isHardBlank(left) || b.isHardBlank(right) {
		return 0, false
	}

	// Rule 1: equal character.
	if mode&smEqual != 0 {
		if left == right {
			return left, true
		}
	}

	// Rules 2 (underscore) and 3 (hierarchy).
	type class struct{ a, b string }
	var smushes []class
	if mode&smLowline != 0 {
		smushes = append(smushes, class{"_", `|/\[]{}()<>`})
	}
	if mode&smHierarchy != 0 {
		smushes = append(smushes,
			class{"|", `/\[]{}()<>`},
			class{`\/`, "[]{}()<>"},
			class{"[]", "{}()<>"},
			class{"{}", "()<>"},
			class{"()", "<>"},
		)
	}
	for _, s := range smushes {
		if runesIn(left, s.a) && runesIn(right, s.b) {
			return right, true
		}
		if runesIn(right, s.a) && runesIn(left, s.b) {
			return left, true
		}
	}

	// Rule 4: opposite pair.
	if mode&smPair != 0 {
		for _, p := range []string{runeStr(left) + runeStr(right), runeStr(right) + runeStr(left)} {
			if p == "[]" || p == "{}" || p == "()" {
				return '|', true
			}
		}
	}

	// Rule 5: big X.
	if mode&smBigX != 0 {
		if left == '/' && right == '\\' {
			return '|', true
		}
		if right == '/' && left == '\\' {
			return 'Y', true
		}
		if left == '>' && right == '<' {
			return 'X', true
		}
	}
	return 0, false
}

// ----------------------------------------------------- FigletString ops ----

var reverseMap = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '<': '>', '>': '<',
	'[': ']', '\\': '/', ']': '[', '{': '}', '}': '{',
}

var flipMap = map[rune]rune{
	'/': '\\', 'A': 'V', 'M': 'W', 'P': 'b', 'R': 'b', 'V': 'A', 'W': 'M',
	'\\': '/', '^': 'v', '_': '-', 'b': 'P', 'm': 'w', 'v': '^', 'w': 'm',
}

func translate(row []rune, m map[rune]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if t, ok := m[r]; ok {
			out[i] = t
		} else {
			out[i] = r
		}
	}
	return out
}

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

func reverseText(s []rune) []rune {
	var rows [][]rune
	for _, row := range splitLines(s) {
		t := translate(row, reverseMap)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		rows = append(rows, t)
	}
	return newFromList(rows)
}

func flipText(s []rune) []rune {
	lines := splitLines(s)
	var rows [][]rune
	for i := len(lines) - 1; i >= 0; i-- {
		rows = append(rows, translate(lines[i], flipMap))
	}
	return newFromList(rows)
}

func stripSurroundingNewlines(s []rune) []rune {
	var rows [][]rune
	seen := false
	for _, row := range splitLines(s) {
		if len(pyStrip(row)) != 0 || seen {
			seen = true
			rows = append(rows, row)
		}
	}
	return pyRStrip(newFromList(rows))
}

func normalizeSurroundingNewlines(s []rune) []rune {
	out := []rune{'\n'}
	out = append(out, stripSurroundingNewlines(s)...)
	return append(out, '\n')
}

// ------------------------------------------------------ option parsing ----

type options struct {
	font, direction, justify string
	width                    int
	reverse, flip            bool
	strip, normalize         bool
	listFonts, infoFont      bool
	load                     string
	color                    string
	help                     bool
}

var longOpts = []string{
	"--font", "--direction", "--justify", "--width", "--reverse",
	"--normalize-surrounding-newlines", "--strip-surrounding-newlines",
	"--flip", "--list_fonts", "--info_font", "--load", "--color", "--help",
}

var shortOpts = map[byte]string{
	'f': "--font", 'D': "--direction", 'j': "--justify", 'w': "--width",
	'r': "--reverse", 'n': "--normalize-surrounding-newlines",
	's': "--strip-surrounding-newlines", 'F': "--flip", 'l': "--list_fonts",
	'i': "--info_font", 'L': "--load", 'c': "--color", 'h': "--help",
}

func takesValue(opt string) bool {
	switch opt {
	case "--font", "--direction", "--justify", "--width", "--load", "--color":
		return true
	}
	return false
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func matchLong(name string) (string, error) {
	var hits []string
	for _, o := range longOpts {
		if o == name {
			return o, nil
		}
		if strings.HasPrefix(o, name) {
			hits = append(hits, o)
		}
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	if len(hits) > 1 {
		return "", usageError{fmt.Sprintf("ambiguous option: %s (%s?)", name, strings.Join(hits, ", "))}
	}
	return "", usageError{"no such option: " + name}
}

// parseInt follows optparse's _parse_num for type='int'.
func parseInt(v string) (int, error) {
	s := v
	base := 10
	switch {
	case len(s) >= 2 && strings.ToLower(s[:2]) == "0x":
		base, s = 16, s[2:]
	case len(s) >= 2 && strings.ToLower(s[:2]) == "0b":
		base, s = 2, s[2:]
		if s == "" {
			s = "0"
		}
	case strings.HasPrefix(s, "0"):
		base = 8
	}
	n, err := strconv.ParseInt(strings.TrimSpace(strings.ReplaceAll(s, "_", "")), base, 64)
	if err != nil {
		return 0, usageError{fmt.Sprintf("option -w: invalid integer value: '%s'", v)}
	}
	return int(n), nil
}

func (o *options) set(opt, val string) error {
	switch opt {
	case "--font":
		o.font = val
	case "--direction":
		if val != "auto" && val != "left-to-right" && val != "right-to-left" {
			return usageError{fmt.Sprintf("option -D: invalid choice: '%s' (choose from 'auto', 'left-to-right', 'right-to-left')", val)}
		}
		o.direction = val
	case "--justify":
		if val != "auto" && val != "left" && val != "center" && val != "right" {
			return usageError{fmt.Sprintf("option -j: invalid choice: '%s' (choose from 'auto', 'left', 'center', 'right')", val)}
		}
		o.justify = val
	case "--width":
		n, err := parseInt(val)
		if err != nil {
			return err
		}
		o.width = n
	case "--reverse":
		o.reverse = true
	case "--flip":
		o.flip = true
	case "--strip-surrounding-newlines":
		o.strip = true
	case "--normalize-surrounding-newlines":
		o.normalize = true
	case "--list_fonts":
		o.listFonts = true
	case "--info_font":
		o.infoFont = true
	case "--load":
		o.load = val
	case "--color":
		o.color = val
	case "--help":
		o.help = true
	}
	return nil
}

func parseArgs(argv []string) (options, []string, error) {
	o := options{font: defaultFont, direction: "auto", justify: "auto", width: 80, color: ":"}
	var pos []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			return o, append(pos, argv[i+1:]...), nil
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a, "=")
			opt, err := matchLong(name)
			if err != nil {
				return o, nil, err
			}
			if takesValue(opt) {
				if !hasVal {
					if i+1 >= len(argv) {
						return o, nil, usageError{opt + " option requires 1 argument"}
					}
					i++
					val = argv[i]
				}
			} else if hasVal {
				return o, nil, usageError{opt + " option does not take a value"}
			}
			if err := o.set(opt, val); err != nil {
				return o, nil, err
			}
		case strings.HasPrefix(a, "-") && a != "-":
			for k := 1; k < len(a); k++ {
				opt, ok := shortOpts[a[k]]
				if !ok {
					r, _ := utf8.DecodeRuneInString(a[k:])
					return o, nil, usageError{"no such option: -" + string(r)}
				}
				if takesValue(opt) {
					val := a[k+1:]
					if val == "" {
						if i+1 >= len(argv) {
							return o, nil, usageError{"-" + string(a[k]) + " option requires 1 argument"}
						}
						i++
						val = argv[i]
					}
					if err := o.set(opt, val); err != nil {
						return o, nil, err
					}
					break
				}
				if err := o.set(opt, ""); err != nil {
					return o, nil, err
				}
			}
		default:
			pos = append(pos, a)
		}
	}
	return o, pos, nil
}

const usage = `Usage: pyfiglet [options] [text..]

Options:
  -h, --help            show this help message and exit
  -f FONT, --font=FONT  font to render with (default: standard)
  -D DIRECTION, --direction=DIRECTION
                        set direction text will be formatted in (default:
                        auto)
  -j SIDE, --justify=SIDE
                        set justification, defaults to print direction
  -w COLS, --width=COLS
                        set terminal width for wrapping/justification
                        (default: 80)
  -r, --reverse         shows mirror image of output text
  -n, --normalize-surrounding-newlines
                        output has one empty line before and after
  -s, --strip-surrounding-newlines
                        removes empty leading and trailing lines
  -F, --flip            flips rendered output text over
  -l, --list_fonts      show installed fonts list
`

func listFonts() []string {
	entries, _ := embeddedFonts.ReadDir("fonts")
	var names []string
	for _, e := range entries {
		names = append(names, strings.SplitN(e.Name(), ".", 2)[0])
	}
	sort.Strings(names)
	return names
}

func pyRepr(s string) string {
	q := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		q = `"`
	}
	s = strings.ReplaceAll(s, `\`, `\\`)
	if q == "'" {
		s = strings.ReplaceAll(s, "'", `\'`)
	}
	return q + s + q
}

// ---------------------------------------------------------------- main ----

func run() int {
	o, args, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Usage: pyfiglet [options] [text..]\n\npyfiglet: error: %s\n", err)
		return 2
	}
	if o.help {
		fmt.Print(usage)
		return 0
	}
	if o.listFonts {
		fmt.Println(strings.Join(listFonts(), "\n"))
		return 0
	}
	if o.infoFont || o.load != "" {
		fmt.Fprintln(os.Stderr, "figlet: -i and -L are not supported by this port")
		return 1
	}
	if len(args) == 0 {
		fmt.Print(usage)
		return 1
	}
	text := []rune(strings.Join(args, " "))

	font, err := loadFont(o.font)
	if err != nil {
		fmt.Printf("pyfiglet error: requested font %s not found.\n", pyRepr(o.font))
		return 1
	}

	direction := o.direction
	if direction == "auto" {
		direction = "left-to-right"
		if font.hasDirection && font.printDirection == 1 {
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

	b := newBuilder(text, font, direction, o.width, justify)
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	r := []rune(b.returnProduct())

	if o.reverse {
		r = reverseText(r)
	}
	if o.flip {
		r = flipText(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = normalizeSurroundingNewlines(r)
	}

	// -c is accepted for compatibility; colours are not part of this port.
	os.Stdout.Write([]byte(string(r) + "\n"))
	return 0
}

func main() {
	defer func() {
		if p := recover(); p != nil {
			if f, ok := p.(fatal); ok {
				fmt.Fprintln(os.Stderr, "figlet:", f.msg)
			} else {
				fmt.Fprintln(os.Stderr, "figlet: internal error:", p)
			}
			os.Exit(1)
		}
	}()
	os.Exit(run())
}
EOF

gofmt -w main.go 2>/dev/null || true

# ------------------------------------------------------ remove Python ----
find "$REPO" -type f -name '*.py' -delete

# ---------------------------------------------------------- self-check ----
go build ./...
go build -o /tmp/figlet .
/tmp/figlet -f slant "solved"
