#!/bin/bash
# Reference solution: port pyfiglet to Go in /workspace/repo.
#   1. move the .flf fonts into fonts/ (embedded into the binary with go:embed),
#   2. write go.mod and main.go (Go standard library only),
#   3. delete every .py file,
#   4. check that `go build ./...` and `go build -o /tmp/figlet .` pass.
set -euo pipefail
cd /workspace/repo

# 1. Fonts: every .flf from the pyfiglet package, kept inside the repository.
src=pyfiglet/fonts
ls "$src"/*.flf >/dev/null 2>&1 || src=pyfiglet/fonts-standard
mkdir -p fonts
mv "$src"/*.flf fonts/

# 2. Go module and source.
cat > go.mod <<'MODEOF'
module figlet

go 1.23
MODEOF

cat > main.go <<'GOEOF'
// Command figlet is a Go port of pyfiglet 1.0.4 (commit 255827e).
//
// It reproduces pyfiglet's output byte for byte, including its quirks. The
// structure follows pyfiglet/__init__.py: FigletFont (font loading),
// FigletBuilder (layout and smushing), FigletString (reverse, flip,
// newline trimming) and main() (an optparse-compatible command line).
//
// Fonts are embedded from fonts/*.flf, so the program works from any directory.
package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

const (
	version       = "1.0.4"
	defaultFont   = "standard"
	sharedDir     = "/usr/local/share/pyfiglet/"
	resetColors   = "\033[0m"
	noRune        = rune(-1) // Python '' (no character)
	neverHardblnk = rune(-2) // hardblank token longer than one character
)

var fontDirectories = []string{sharedDir, "/usr/local/share/figlet", "/usr/share/figlet"}

// ---------------------------------------------------------------------------
// Python exceptions. Uncaught ones end the program with exit status 1 and no
// stdout, exactly like a Python traceback would.

type pyException struct{ kind, msg string }

func raise(kind, format string, a ...any) {
	panic(pyException{kind, fmt.Sprintf(format, a...)})
}

type fontNotFound struct{ font string }

// ---------------------------------------------------------------------------
// Python string helpers (str.isspace, str.splitlines, str.strip, ...).

func pyIsSpace(r rune) bool {
	switch {
	case r >= '\t' && r <= '\r', r >= 0x1c && r <= 0x20:
		return true
	case r == 0x85, r == 0xa0, r == 0x1680, r >= 0x2000 && r <= 0x200a,
		r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pySplitlines is str.splitlines().
func pySplitlines(s []rune) [][]rune {
	var out [][]rune
	start := 0
	for i := 0; i < len(s); i++ {
		if !isLineBreak(s[i]) {
			continue
		}
		out = append(out, s[start:i])
		if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			i++
		}
		start = i + 1
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func pyStrip(s []rune) []rune { return pyLStrip(pyRStrip(s)) }
func pyRStrip(s []rune) []rune {
	i := len(s)
	for i > 0 && pyIsSpace(s[i-1]) {
		i--
	}
	return s[:i]
}
func pyLStrip(s []rune) []rune {
	i := 0
	for i < len(s) && pyIsSpace(s[i]) {
		i++
	}
	return s[i:]
}

func rstripSpace(s []rune) []rune {
	i := len(s)
	for i > 0 && s[i-1] == ' ' {
		i--
	}
	return s[:i]
}
func lstripSpace(s []rune) []rune {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return s[i:]
}

// pySplitWS is str.split() with no arguments.
func pySplitWS(s []rune) [][]rune {
	var out [][]rune
	i := 0
	for i < len(s) {
		for i < len(s) && pyIsSpace(s[i]) {
			i++
		}
		j := i
		for j < len(s) && !pyIsSpace(s[j]) {
			j++
		}
		if j > i {
			out = append(out, s[i:j])
		}
		i = j
	}
	return out
}

func joinRows(rows [][]rune) string {
	var sb strings.Builder
	for i, r := range rows {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(string(r))
	}
	return sb.String()
}

// pyDecodeReplace is bytes.decode('UTF-8', 'replace'): each maximal invalid
// subsequence becomes one U+FFFD.
func pyDecodeReplace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			out = append(out, rune(c))
			i++
			continue
		}
		need, lo, hi := 0, byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c == 0xE0:
			need, lo = 2, 0xA0
		case c == 0xED:
			need, hi = 2, 0x9F
		case c >= 0xE1 && c <= 0xEF:
			need = 2
		case c == 0xF0:
			need, lo = 3, 0x90
		case c == 0xF4:
			need, hi = 3, 0x8F
		case c >= 0xF1 && c <= 0xF3:
			need = 3
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

// decodeArg mirrors how Python decodes argv on POSIX (UTF-8, surrogateescape).
func decodeArg(s string) []rune {
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 {
			r = 0xDC00 + rune(s[i])
		}
		out = append(out, r)
		i += n
	}
	return out
}

// pyInt is int(s, base) for base 2, 8, 10 or 16.
func pyInt(str string, base int) (int, bool) {
	s := string(pyStrip([]rune(str)))
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	afterPrefix := false
	if len(s) >= 2 && s[0] == '0' {
		p := s[1] | 0x20
		if (base == 16 && p == 'x') || (base == 8 && p == 'o') || (base == 2 && p == 'b') {
			s = s[2:]
			afterPrefix = true
		}
	}
	if afterPrefix && strings.HasPrefix(s, "_") {
		s = s[1:]
	}
	if s == "" {
		return 0, false
	}
	v := 0
	prevUnderscore := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			if prevUnderscore || i == 0 || i == len(s)-1 {
				return 0, false
			}
			prevUnderscore = true
			continue
		}
		prevUnderscore = false
		d := -1
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'z':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'Z':
			d = int(c-'A') + 10
		}
		if d < 0 || d >= base {
			return 0, false
		}
		if v > (1<<62)/base {
			return 0, false // far beyond anything meaningful
		}
		v = v*base + d
	}
	if neg {
		v = -v
	}
	return v, true
}

// pyRepr is a simplified repr() for str.
func pyRepr(s string) string {
	q := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = '"'
	}
	var sb strings.Builder
	sb.WriteByte(q)
	for _, r := range s {
		switch {
		case r == rune(q) || r == '\\':
			sb.WriteByte('\\')
			sb.WriteRune(r)
		case r == '\n':
			sb.WriteString(`\n`)
		case r == '\r':
			sb.WriteString(`\r`)
		case r == '\t':
			sb.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&sb, `\x%02x`, r)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte(q)
	return sb.String()
}

// ---------------------------------------------------------------------------
// FigletFont

type figletFont struct {
	name           string
	height         int
	hardBlank      string
	hbRune         rune
	printDirection int
	hasDirection   bool
	smushMode      int
	comment        string
	chars          map[int][][]rune
	width          map[int]int
}

func pyJoin(dir, name string) string {
	if strings.HasPrefix(name, "/") {
		return name
	}
	if dir == "" || strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func embedded(fn string) ([]byte, bool) {
	if fn == "" || strings.ContainsAny(fn, "/\\") {
		return nil, false
	}
	b, err := embeddedFonts.ReadFile("fonts/" + fn)
	return b, err == nil
}

// unzipFirst returns the first member of a ZIP archive, if b is one.
func unzipFirst(b []byte) ([]byte, bool) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, false
	}
	if len(zr.File) == 0 {
		raise("IndexError", "list index out of range")
	}
	f, err := zr.File[0].Open()
	if err != nil {
		raise("BadZipFile", "%v", err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		raise("BadZipFile", "%v", err)
	}
	return data, true
}

// preloadFont follows FigletFont.preloadFont: for each extension (tlf, then
// flf) the package copy wins and stops the search; otherwise a file in ./ or
// a font directory is taken, and a later extension overrides an earlier one.
func preloadFont(font string) []rune {
	var data []byte
	found := false
	for _, ext := range []string{"tlf", "flf"} {
		fn := font + "." + ext
		if b, ok := embedded(fn); ok {
			data, found = b, true
			break
		}
		for _, loc := range append([]string{"./"}, fontDirectories...) {
			full := pyJoin(loc, fn)
			if isFile(full) {
				b, err := os.ReadFile(full)
				if err != nil {
					raise("OSError", "%v", err)
				}
				data, found = b, true
				break
			}
		}
	}
	if found {
		if z, ok := unzipFirst(data); ok {
			data = z
		}
	}
	if !found || len(data) == 0 {
		panic(fontNotFound{font})
	}
	return pyDecodeReplace(data)
}

func hasMagic(line []rune) bool {
	return len(line) >= 5 && (line[0] == 't' || line[0] == 'f') &&
		line[1] == 'l' && line[2] == 'f' && line[3] == '2' && line[4] != '\n'
}

func loadFigletFont(font string) *figletFont {
	f := &figletFont{name: font, chars: map[int][][]rune{}, width: map[int]int{}}
	raw := preloadFont(font)
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(fontNotFound); ok {
					panic(r)
				}
				raise("FontError", "problem parsing %s font: %v", font, r)
			}
		}()
		f.parse(raw)
	}()
	return f
}

func (f *figletFont) parse(raw []rune) {
	data := make([]rune, len(raw))
	for i, r := range raw {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			r = ' '
		}
		data[i] = r
	}
	lines := pySplitlines(data)
	pop := func() []rune {
		if len(lines) == 0 {
			raise("IndexError", "pop from empty list")
		}
		l := lines[0]
		lines = lines[1:]
		return l
	}

	header := pop()
	if !hasMagic(header) {
		raise("FontError", "%s is not a valid figlet font", f.name)
	}
	fields := pySplitWS(header[5:])
	if len(fields) < 6 {
		raise("FontError", "malformed header for %s", f.name)
	}
	f.hardBlank = string(fields[0])
	f.hbRune = neverHardblnk
	if len(fields[0]) == 1 {
		f.hbRune = fields[0][0]
	}
	nums := make([]int, 5)
	for i := range nums {
		v, ok := pyInt(string(fields[i+1]), 10)
		if !ok {
			raise("ValueError", "invalid literal for int()")
		}
		nums[i] = v
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	fullLayout, hasFull := 0, false
	if len(fields) > 6 {
		v, ok := pyInt(string(fields[6]), 10)
		if !ok {
			raise("ValueError", "invalid literal for int()")
		}
		f.printDirection, f.hasDirection = v, true
	}
	if len(fields) > 7 {
		v, ok := pyInt(string(fields[7]), 10)
		if !ok {
			raise("ValueError", "invalid literal for int()")
		}
		fullLayout, hasFull = v, true
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
		f.comment += string(pop())
	}

	loadChar := func() (int, [][]rune) {
		width := 0
		var rows [][]rune
		end, haveEnd := rune(0), false
		for n := 0; n < height; n++ {
			line := pop()
			if !haveEnd {
				// reEndMarker = (.)\s*$  -> last non-space char, else the first char
				if len(line) == 0 {
					raise("AttributeError", "'NoneType' object has no attribute 'group'")
				}
				end = line[0]
				for i := len(line) - 1; i >= 0; i-- {
					if !pyIsSpace(line[i]) {
						end = line[i]
						break
					}
				}
				haveEnd = true
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
		w, letter := loadChar()
		if i == 32 || nonEmpty(letter) {
			f.chars[i] = letter
			f.width[i] = w
		}
	}
	if len(lines) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, letter := loadChar()
			if nonEmpty(letter) {
				f.chars[int(c)] = letter
				f.width[int(c)] = w
			}
		}
	}
	for len(lines) > 0 {
		line := pyStrip(pop())
		code := line
		for i, r := range line {
			if r == ' ' {
				code = line[:i]
				break
			}
		}
		if len(code) == 0 {
			continue
		}
		if len(code) >= 2 && code[0] == '0' && (code[1] == 'x' || code[1] == 'X') {
			v, ok := pyInt(string(code), 16)
			if !ok {
				raise("ValueError", "invalid literal for int() with base 16")
			}
			w, letter := loadChar()
			if nonEmpty(letter) {
				f.chars[v] = letter
				f.width[v] = w
			}
		}
	}
}

// stripEndMarker is re.sub(re.escape(end) + r'{1,2}\s*$', ”, line).
func stripEndMarker(line []rune, end rune) []rune {
	restIsSpace := func(from int) bool {
		for _, r := range line[from:] {
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
		if i+1 < len(line) && line[i+1] == end && restIsSpace(i+2) {
			return line[:i]
		}
		if restIsSpace(i + 1) {
			return line[:i]
		}
	}
	return line
}

// ---------------------------------------------------------------------------
// FigletBuilder

const (
	smEqual     = 1   // smush equal chars (not hardblanks)
	smLowline   = 2   // smush _ with any char in hierarchy
	smHierarchy = 4   // hierarchy: |, /\, [], {}, (), <>
	smPair      = 8   // hierarchy: [ + ] -> |, { + } -> |, ( + ) -> |
	smBigX      = 16  // / + \ -> |, \ + / -> Y, > + < -> X
	smHardblank = 32  // hardblank + hardblank -> hardblank
	smKern      = 64  // kerning
	smSmush     = 128 // smushing
)

type marker struct {
	buffer   [][]rune
	iterator int
}

type figletBuilder struct {
	text    []rune
	rtl     bool
	width   int
	font    *figletFont
	justify string

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []marker
	queue        [][][]rune
	buffer       [][]rune
}

func emptyBuffer(h int) [][]rune {
	if h < 0 {
		h = 0
	}
	return make([][]rune, h)
}

func copyBuffer(b [][]rune) [][]rune { return append([][]rune(nil), b...) }

func newBuilder(text []rune, font *figletFont, direction string, width int, justify string) *figletBuilder {
	return &figletBuilder{
		text: text, rtl: direction == "right-to-left", width: width,
		font: font, justify: justify, buffer: emptyBuffer(font.height),
	}
}

func (b *figletBuilder) render() string {
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	return b.returnProduct()
}

func (b *figletBuilder) getCharAt(i int) ([][]rune, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	c, ok := b.font.chars[int(b.text[i])]
	return c, ok
}

func (b *figletBuilder) getCharWidthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	w, ok := b.font.width[int(b.text[i])]
	return w, ok
}

func (b *figletBuilder) addCharToProduct() {
	curChar, ok := b.getCharAt(b.iterator)

	// if the character is a newline, we flush the buffer
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if !ok {
		return
	}
	w, _ := b.getCharWidthAt(b.iterator)
	if b.width < w {
		raise("CharNotPrinted", "Width is not enough to print this character")
	}
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, curChar)

	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
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

func (b *figletBuilder) returnProduct() string {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		sb.WriteString(b.replaceHardblanks(b.justifyString(buf)))
	}
	return sb.String()
}

func (b *figletBuilder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.rtl {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := noRune
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		right := addRight[i] // IndexError in pyfiglet too
		smushed, ok := b.smushChars(left, right)
		addLeft = updateSmushedCharInLeftBuffer(addLeft, idx, smushed, ok)
	}
	return addLeft, addRight
}

func updateSmushedCharInLeftBuffer(addLeft []rune, idx int, smushed rune, ok bool) []rune {
	if idx < 0 || idx > len(addLeft) {
		return addLeft
	}
	l := append([]rune(nil), addLeft...)
	_ = l[idx] // IndexError when idx == len, as in pyfiglet
	if !ok {
		raise("TypeError", "sequence item %d: expected str instance, NoneType found", idx)
	}
	if smushed == noRune { // l[idx] = '' removes the character on join
		return append(l[:idx], l[idx+1:]...)
	}
	l[idx] = smushed
	return l
}

func (b *figletBuilder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	tail := []rune{}
	if b.maxSmush < len(addRight) {
		tail = addRight[b.maxSmush:]
	}
	row2 := make([]rune, 0, len(addLeft)+len(tail))
	row2 = append(append(row2, addLeft...), tail...)
	b.buffer[row] = row2
}

func (b *figletBuilder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = emptyBuffer(b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, ok := b.getCharAt(b.iterator)
	if !ok {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

func (b *figletBuilder) cutBufferAtLastBlank(saved [][]rune, savedIterator int) {
	b.queue = append(b.queue, saved)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *figletBuilder) cutBufferAtLastChar() {
	// With an empty buffer pyfiglet re-processes the same character from the
	// same state forever (a glyph exactly as wide as -w). Stop instead.
	empty := true
	for _, r := range b.buffer {
		if len(r) > 0 {
			empty = false
			break
		}
	}
	if empty {
		raise("RecursionHang", "pyfiglet never terminates for this input (glyph as wide as the output width)")
	}
	b.queue = append(b.queue, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *figletBuilder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.cutBufferAtLastBlank(m.buffer, m.iterator)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *figletBuilder) justifyString(buffer [][]rune) [][]rune {
	out := copyBuffer(buffer)
	for row := 0; row < b.font.height; row++ {
		pad := 0
		switch b.justify {
		case "right":
			pad = b.width - len(out[row]) - 1
		case "center":
			pad = (b.width - len(out[row])) / 2 // int() truncates toward zero, like Go
		default:
			continue
		}
		if pad < 0 {
			pad = 0
		}
		out[row] = append([]rune(strings.Repeat(" ", pad)), out[row]...)
	}
	return out
}

func (b *figletBuilder) replaceHardblanks(buffer [][]rune) string {
	s := joinRows(buffer) + "\n"
	return strings.ReplaceAll(s, b.font.hardBlank, " ")
}

func (b *figletBuilder) smushAmount(buffer [][]rune, curChar [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.rtl {
			lineLeft, lineRight = lineRight, lineLeft
		}

		linebd := len(rstripSpace(lineLeft)) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := noRune
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}

		charbd := len(lineRight) - len(lstripSpace(lineRight))
		ch2 := noRune
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
		}

		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == noRune || ch1 == ' ' {
			amt++
		} else if ch2 != noRune {
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

// in is Python's `ch in s` for a one-character (or empty) string ch.
func in(ch rune, s string) bool { return ch == noRune || strings.ContainsRune(s, ch) }

func runeStr(r rune) string {
	if r == noRune {
		return ""
	}
	return string(r)
}

var hierarchy = [][2]string{
	{"|", `/\[]{}()<>`},
	{`\/`, "[]{}()<>"},
	{"[]", "{}()<>"},
	{"{}", "()<>"},
	{"()", "<>"},
}

// smushChars ports FigletBuilder.smushChars rule by rule.
// The bool result is false where pyfiglet returns None.
func (b *figletBuilder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}

	// Disallows overlapping if previous or current char has a width of 1 or zero
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}

	mode := b.font.smushMode
	// kerning only
	if mode&smSmush == 0 {
		return 0, false
	}

	hb := b.font.hbRune
	// smushing by universal overlapping
	if mode&63 == 0 {
		if left == hb {
			return right, true
		}
		if right == hb {
			return left, true
		}
		if b.rtl {
			return left, true
		}
		return right, true
	}

	// Rule 6: hardblank
	if mode&smHardblank != 0 {
		if left == hb && right == hb {
			return left, true
		}
	}
	if left == hb || right == hb {
		return 0, false
	}

	// Rule 1: equal character
	if mode&smEqual != 0 {
		if left == right {
			return left, true
		}
	}

	// Rules 2 and 3: underscore and hierarchy
	var smushes [][2]string
	if mode&smLowline != 0 {
		smushes = append(smushes, [2]string{"_", `|/\[]{}()<>`})
	}
	if mode&smHierarchy != 0 {
		smushes = append(smushes, hierarchy...)
	}
	for _, ab := range smushes {
		if in(left, ab[0]) && in(right, ab[1]) {
			return right, true
		}
		if in(right, ab[0]) && in(left, ab[1]) {
			return left, true
		}
	}

	// Rule 4: opposite pair
	if mode&smPair != 0 {
		for _, pair := range []string{runeStr(left) + runeStr(right), runeStr(right) + runeStr(left)} {
			if pair == "[]" || pair == "{}" || pair == "()" {
				return '|', true
			}
		}
	}

	// Rule 5: big X
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

// ---------------------------------------------------------------------------
// FigletString

var reverseMap, flipMap [256]rune

func init() {
	for i := range reverseMap {
		reverseMap[i] = rune(i)
		flipMap[i] = rune(i)
	}
	for _, p := range []string{"()", ")(", "/\\", "\\/", "<>", "><", "[]", "][", "{}", "}{"} {
		reverseMap[p[0]] = rune(p[1])
	}
	for _, p := range []string{"/\\", "AV", "MW", "Pb", "Rb", "VA", "WM", "\\/", "^v", "_-", "bP", "mw", "v^", "wm"} {
		flipMap[p[0]] = rune(p[1])
	}
}

func translate(row []rune, table *[256]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if r >= 0 && r < 256 {
			r = table[r]
		}
		out[i] = r
	}
	return out
}

func newFromList(rows [][]rune) string { return joinRows(rows) + "\n" }

func reverse(s string) string {
	var out [][]rune
	for _, row := range pySplitlines([]rune(s)) {
		t := translate(row, &reverseMap)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		out = append(out, t)
	}
	return newFromList(out)
}

func flip(s string) string {
	rows := pySplitlines([]rune(s))
	var out [][]rune
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translate(rows[i], &flipMap))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s string) string {
	var out [][]rune
	charsSeen := false
	for _, row := range pySplitlines([]rune(s)) {
		if len(pyStrip(row)) != 0 || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}
	return string(pyRStrip([]rune(newFromList(out))))
}

func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}

// ---------------------------------------------------------------------------
// Colours (-c)

var colorCodes = map[string]int{"BLACK": 30, "RED": 31, "GREEN": 32, "YELLOW": 33, "BLUE": 34, "MAGENTA": 35, "CYAN": 36, "LIGHT_GRAY": 37,
	"DEFAULT": 39, "DARK_GRAY": 90, "LIGHT_RED": 91, "LIGHT_GREEN": 92, "LIGHT_YELLOW": 93, "LIGHT_BLUE": 94,
	"LIGHT_MAGENTA": 95, "LIGHT_CYAN": 96, "WHITE": 97, "RESET": 0}

func colorToANSI(color string, background bool) string {
	if color == "" {
		return ""
	}
	color = strings.ToUpper(color)
	n := strings.Count(color, ";")
	_, known := colorCodes[color]
	if n > 0 && n != 2 {
		raise("InvalidColor", "Specified color '{}' not a valid color in R;G;B format")
	}
	if n == 0 && !known {
		raise("InvalidColor", "Specified color '%s' not found in ANSI COLOR_CODES list", color)
	}
	if known {
		code := colorCodes[color]
		if background {
			code += 10
		}
		return fmt.Sprintf("\033[%dm", code)
	}
	code := 38
	if background {
		code = 48
	}
	return fmt.Sprintf("\033[%d;2;%sm", code, color)
}

func parseColor(color string) string {
	fg, bg, _ := strings.Cut(color, ":")
	return colorToANSI(fg, false) + colorToANSI(bg, true)
}

// ---------------------------------------------------------------------------
// -l, -i, -L

func headerOf(b []byte) []rune {
	if z, ok := unzipFirst(b); ok {
		b = z
	}
	line := b
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		line = b[:i+1]
	}
	return pyDecodeReplace(line)
}

func listFonts() []string {
	var names []string
	entries, _ := embeddedFonts.ReadDir("fonts")
	for _, e := range entries {
		names = append(names, strings.SplitN(e.Name(), ".", 3)[0])
	}
	for _, dir := range fontDirectories {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			n := e.Name()
			if !strings.HasSuffix(n, ".flf") && !strings.HasSuffix(n, ".tlf") {
				continue
			}
			if b, err := os.ReadFile(pyJoin(dir, n)); err == nil && e.Type().IsRegular() && hasMagic(headerOf(b)) {
				names = append(names, strings.SplitN(n, ".", 3)[0])
			}
		}
	}
	sort.Strings(names)
	return names
}

var infoStartMarkers = []string{"FONT", "COMMENT", "FONTNAME_REGISTRY", "FAMILY_NAME", "FOUNDRY", "WEIGHT_NAME",
	"SETWIDTH_NAME", "SLANT", "ADD_STYLE_NAME", "PIXEL_SIZE", "POINT_SIZE", "RESOLUTION_X", "RESOLUTION_Y",
	"SPACING", "AVERAGE_WIDTH", "FONT_DESCENT", "FONT_ASCENT", "CAP_HEIGHT", "X_HEIGHT", "FACE_NAME",
	"FULL_NAME", "COPYRIGHT", "_DEC_", "DEFAULT_CHAR", "NOTICE", "RELATIVE_"}

func infoFont(font string) string {
	lines := pySplitlines(preloadFont(font))
	if len(lines) > 100 {
		lines = lines[:100]
	}
	var infos []string
	for _, line := range lines {
		s := string(line)
		start := false
		for _, m := range infoStartMarkers {
			if strings.HasPrefix(s, m) {
				start = true
				break
			}
		}
		end := len(line) > 0 && strings.ContainsRune("@#$", line[len(line)-1])
		if !hasMagic(line) && !start && !end {
			infos = append(infos, s)
		}
	}
	return strings.Join(infos, "\n")
}

func installFonts(fileName string) {
	fmt.Printf("Installing %s to %s\n", fileName, sharedDir)
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		raise("OSError", "%v", err)
	}
	b, err := os.ReadFile(fileName)
	if err != nil {
		raise("FileNotFoundError", "%v", err)
	}
	base := fileName[strings.LastIndex(fileName, "/")+1:]
	if strings.EqualFold(base[max(0, strings.LastIndex(base, ".")):], ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			raise("BadZipFile", "%v", err)
		}
		for _, zf := range zr.File {
			name := zf.Name[strings.LastIndex(zf.Name, "/")+1:]
			if name == "" {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				raise("BadZipFile", "%v", err)
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			if err := os.WriteFile(pyJoin(sharedDir, name), data, 0o644); err != nil {
				raise("OSError", "%v", err)
			}
		}
		return
	}
	if err := os.WriteFile(pyJoin(sharedDir, base), b, 0o644); err != nil {
		raise("OSError", "%v", err)
	}
}

// ---------------------------------------------------------------------------
// Command line (optparse-compatible)

type options struct {
	font, direction, justify, color, load string
	width                                 int
	reverse, normalize, strip, flip       bool
	listFonts, infoFont                   bool
}

type optSpec struct {
	short, long string
	takesValue  bool
	choices     []string
	isInt       bool
	set         func(o *options, v string)
}

const usage = "Usage: figlet [options] [text..]"

var helpText = usage + `

Options:
  --version             show program's version number and exit
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
  -i, --info_font       show font's information, use with -f FONT
  -L LOAD, --load=LOAD  load and install the specified font definition
  -c COLOR, --color=COLOR
                        prints text with passed foreground color
`

func optError(msg string) {
	fmt.Fprintf(os.Stderr, "%s\n\nfiglet: error: %s\n", usage, msg)
	os.Exit(2)
}

var specs = []optSpec{
	{"", "--version", false, nil, false, func(*options, string) { fmt.Println(version); os.Exit(0) }},
	{"-h", "--help", false, nil, false, func(*options, string) { fmt.Print(helpText); os.Exit(0) }},
	{"-f", "--font", true, nil, false, func(o *options, v string) { o.font = v }},
	{"-D", "--direction", true, []string{"auto", "left-to-right", "right-to-left"}, false, func(o *options, v string) { o.direction = v }},
	{"-j", "--justify", true, []string{"auto", "left", "center", "right"}, false, func(o *options, v string) { o.justify = v }},
	{"-w", "--width", true, nil, true, nil},
	{"-r", "--reverse", false, nil, false, func(o *options, _ string) { o.reverse = true }},
	{"-n", "--normalize-surrounding-newlines", false, nil, false, func(o *options, _ string) { o.normalize = true }},
	{"-s", "--strip-surrounding-newlines", false, nil, false, func(o *options, _ string) { o.strip = true }},
	{"-F", "--flip", false, nil, false, func(o *options, _ string) { o.flip = true }},
	{"-l", "--list_fonts", false, nil, false, func(o *options, _ string) { o.listFonts = true }},
	{"-i", "--info_font", false, nil, false, func(o *options, _ string) { o.infoFont = true }},
	{"-L", "--load", true, nil, false, func(o *options, v string) { o.load = v }},
	{"-c", "--color", true, nil, false, func(o *options, v string) { o.color = v }},
}

// parseNum is optparse's _parse_num(val, int).
func parseNum(val string) (int, bool) {
	r := []rune(val)
	prefix := strings.ToLower(string(r[:min(2, len(r))]))
	switch {
	case prefix == "0x":
		return pyInt(val, 16)
	case prefix == "0b":
		v := string(r[2:])
		if v == "" {
			v = "0"
		}
		return pyInt(v, 2)
	case strings.HasPrefix(val, "0"):
		return pyInt(val, 8)
	}
	return pyInt(val, 10)
}

func (s *optSpec) name() string {
	if s.short != "" {
		return s.short + "/" + s.long
	}
	return s.long
}

func (s *optSpec) apply(o *options, optName, value string) {
	if s.choices != nil {
		okChoice := false
		for _, c := range s.choices {
			okChoice = okChoice || c == value
		}
		if !okChoice {
			q := make([]string, len(s.choices))
			for i, c := range s.choices {
				q[i] = pyRepr(c)
			}
			optError(fmt.Sprintf("option %s: invalid choice: %s (choose from %s)", optName, pyRepr(value), strings.Join(q, ", ")))
		}
	}
	if s.isInt {
		v, ok := parseNum(value)
		if !ok {
			optError(fmt.Sprintf("option %s: invalid integer value: %s", optName, pyRepr(value)))
		}
		o.width = v
		return
	}
	s.set(o, value)
}

func parseArgs(argv []string) (*options, []string) {
	o := &options{font: defaultFont, direction: "auto", justify: "auto", width: 80, color: ":"}
	var positional []string
	rargs := append([]string(nil), argv...)
	popArg := func() string { a := rargs[0]; rargs = rargs[1:]; return a }
	needValue := func(opt string) string {
		if len(rargs) < 1 {
			optError(opt + " option requires 1 argument")
		}
		return popArg()
	}

	for len(rargs) > 0 {
		arg := rargs[0]
		switch {
		case arg == "--":
			popArg()
			positional = append(positional, rargs...)
			rargs = nil
		case strings.HasPrefix(arg, "--"):
			popArg()
			opt, val, explicit := strings.Cut(arg, "=")
			if explicit {
				rargs = append([]string{val}, rargs...)
			}
			var spec *optSpec
			var matches []string
			for i := range specs {
				if specs[i].long == opt {
					spec, matches = &specs[i], nil
					break
				}
				if strings.HasPrefix(specs[i].long, opt) {
					matches = append(matches, specs[i].long)
					spec = &specs[i]
				}
			}
			if spec == nil {
				optError("no such option: " + opt)
			}
			if len(matches) > 1 {
				sort.Strings(matches)
				optError(fmt.Sprintf("ambiguous option: %s (%s?)", opt, strings.Join(matches, ", ")))
			}
			opt = spec.long
			value := ""
			if spec.takesValue {
				value = needValue(opt)
			} else if explicit {
				optError(opt + " option does not take a value")
			}
			spec.apply(o, opt, value)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			popArg()
			chars := []rune(arg)
			for i := 1; i < len(chars); i++ {
				opt := "-" + string(chars[i])
				var spec *optSpec
				for k := range specs {
					if specs[k].short == opt {
						spec = &specs[k]
					}
				}
				if spec == nil {
					optError("no such option: " + opt)
				}
				value := ""
				if spec.takesValue {
					if i+1 < len(chars) {
						rargs = append([]string{string(chars[i+1:])}, rargs...)
					}
					value = needValue(opt)
					spec.apply(o, opt, value)
					break
				}
				spec.apply(o, opt, value)
			}
		default:
			positional = append(positional, popArg())
		}
	}
	return o, positional
}

func run(argv []string, stdout io.Writer) int {
	o, args := parseArgs(argv)

	if o.listFonts {
		fmt.Fprintln(stdout, strings.Join(listFonts(), "\n"))
		return 0
	}
	if o.color == "list" {
		keys := make([]string, 0, len(colorCodes))
		for k := range colorCodes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintln(stdout, "[0-255];[0-255];[0-255] # RGB\n"+strings.Join(keys, "\n"))
		return 0
	}
	if o.infoFont {
		fmt.Fprintln(stdout, infoFont(o.font))
		return 0
	}
	if o.load != "" {
		installFonts(o.load)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprint(stdout, helpText)
		return 1
	}

	var text []rune
	for i, a := range args {
		if i > 0 {
			text = append(text, ' ')
		}
		text = append(text, decodeArg(a)...)
	}

	var font *figletFont
	func() {
		defer func() {
			if r := recover(); r != nil {
				if nf, ok := r.(fontNotFound); ok {
					fmt.Fprintf(stdout, "pyfiglet error: requested font %s not found.\n", pyRepr(nf.font))
					font = nil
					return
				}
				panic(r)
			}
		}()
		font = loadFigletFont(o.font)
	}()
	if font == nil {
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
		justify = "left"
		if direction == "right-to-left" {
			justify = "right"
		}
	}

	r := newBuilder(text, font, direction, o.width, justify).render()
	if o.reverse {
		r = reverse(r)
	}
	if o.flip {
		r = flip(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = normalizeSurroundingNewlines(r)
	}

	ansi := parseColor(o.color)
	var out bytes.Buffer
	out.WriteString(ansi)
	out.WriteString(r)
	out.WriteByte('\n')
	if ansi != "" {
		out.WriteString(resetColors)
	}
	stdout.Write(out.Bytes())
	return 0
}

func main() {
	var out bytes.Buffer
	code := func() (code int) {
		defer func() {
			if r := recover(); r != nil {
				switch e := r.(type) {
				case pyException:
					fmt.Fprintf(os.Stderr, "Traceback (most recent call last):\n%s: %s\n", e.kind, e.msg)
				case fontNotFound:
					fmt.Fprintf(os.Stderr, "Traceback (most recent call last):\nFontNotFound: %s\n", e.font)
				default:
					fmt.Fprintf(os.Stderr, "Traceback (most recent call last):\n%v\n", r)
				}
				out.Reset() // pyfiglet writes nothing to stdout when it crashes
				code = 1
			}
		}()
		return run(os.Args[1:], &out)
	}()
	os.Stdout.Write(out.Bytes())
	os.Exit(code)
}
GOEOF

# 3. No Python left anywhere in the repository.
find . -name '*.py' -type f -delete

# 4. Sanity checks.
gofmt -l . | (! grep .) || { echo "gofmt needed" >&2; exit 1; }
go vet ./...
go build ./...
go build -o /tmp/figlet .
/tmp/figlet -f slant "solved"
