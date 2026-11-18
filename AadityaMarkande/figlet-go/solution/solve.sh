#!/bin/bash
# Oracle solution: Go port of pyfiglet (standard library only).
set -e
cd /workspace/repo

# fonts: copied into a folder inside the repo so go:embed can include them
mkdir -p fontdata
find pyfiglet/fonts -maxdepth 1 -type f \( -name '*.flf' -o -name '*.tlf' \) -exec cp {} fontdata/ \;

cat > go.mod <<'GOMOD'
module figlet

go 1.23
GOMOD

cat > main.go <<'GOEOF'
package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

//go:embed fontdata
var fontFS embed.FS

// noChar stands for Python's empty string '' where a single character is expected.
const noChar rune = -1

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

// failure is used to mimic an uncaught Python exception (exit code 1, no output).
type failure struct{ msg string }

func fail(msg string) { panic(failure{msg}) }

// ---------------------------------------------------------------------------
// Python-like string helpers (all work on []rune, like Python str)
// ---------------------------------------------------------------------------

// isPySpace mirrors Python's str.isspace() / regex \s for str.
func isPySpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0, 0x1680,
		0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// splitLines mirrors Python's str.splitlines() (without keepends).
func splitLines(s []rune) [][]rune {
	var lines [][]rune
	start := 0
	i := 0
	for i < len(s) {
		if isLineBreak(s[i]) {
			lines = append(lines, s[start:i])
			if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			i++
			start = i
			continue
		}
		i++
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func allSpace(s []rune) bool {
	for _, r := range s {
		if !isPySpace(r) {
			return false
		}
	}
	return true
}

func stripPy(s []rune) []rune {
	a, b := 0, len(s)
	for a < b && isPySpace(s[a]) {
		a++
	}
	for b > a && isPySpace(s[b-1]) {
		b--
	}
	return s[a:b]
}

func rstripPy(s []rune) []rune {
	b := len(s)
	for b > 0 && isPySpace(s[b-1]) {
		b--
	}
	return s[:b]
}

// length of s.rstrip(' ')
func rstripSpaceLen(s []rune) int {
	n := len(s)
	for n > 0 && s[n-1] == ' ' {
		n--
	}
	return n
}

// len(s) - len(s.lstrip(' '))
func lstripSpaceCount(s []rune) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

// sliceFrom mirrors Python's s[n:] for n >= 0 (never fails).
func sliceFrom(s []rune, n int) []rune {
	if n <= 0 {
		return s
	}
	if n >= len(s) {
		return nil
	}
	return s[n:]
}

func concat(a, b []rune) []rune {
	out := make([]rune, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}

func padLeft(row []rune, n int) []rune {
	if n <= 0 {
		return row
	}
	out := make([]rune, 0, n+len(row))
	for i := 0; i < n; i++ {
		out = append(out, ' ')
	}
	return append(out, row...)
}

// ---------------------------------------------------------------------------
// Font loading (port of FigletFont.preloadFont / loadFont)
// ---------------------------------------------------------------------------

type figFont struct {
	height    int
	hardBlank string
	printDir  int // -1 when the header has no print direction
	smushMode int
	chars     map[int][][]rune
	widths    map[int]int
}

func readFontFile(name string) ([]byte, bool) {
	for _, ext := range []string{"tlf", "flf"} {
		data, err := fontFS.ReadFile("fontdata/" + name + "." + ext)
		if err == nil {
			return data, true
		}
	}
	return nil, false
}

func unzipFirst(data []byte) []byte {
	if len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 3 && data[3] == 4 {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err == nil && len(zr.File) > 0 {
			rc, err := zr.File[0].Open()
			if err == nil {
				defer rc.Close()
				out, err := io.ReadAll(rc)
				if err == nil {
					return out
				}
			}
		}
	}
	return data
}

type fontParser struct {
	lines [][]rune
	pos   int
}

func (p *fontParser) next() ([]rune, error) {
	if p.pos >= len(p.lines) {
		return nil, fmt.Errorf("font data ended early")
	}
	l := p.lines[p.pos]
	p.pos++
	return l, nil
}

// stripEndMarker mirrors re.compile(re.escape(end) + r'{1,2}\s*$').sub('', line).
func stripEndMarker(line []rune, x rune) []rune {
	n := len(line)
	for i := 0; i < n; i++ {
		if line[i] != x {
			continue
		}
		if i+1 < n && line[i+1] == x && allSpace(line[i+2:]) {
			return line[:i]
		}
		if allSpace(line[i+1:]) {
			return line[:i]
		}
	}
	return line
}

func (p *fontParser) loadChar(height int) (int, [][]rune, error) {
	var end rune
	haveEnd := false
	width := 0
	rows := make([][]rune, 0, height)
	for k := 0; k < height; k++ {
		line, err := p.next()
		if err != nil {
			return 0, nil, err
		}
		if !haveEnd {
			// reEndMarker = (.)\s*$ : the last non-whitespace char
			if len(line) == 0 {
				return 0, nil, fmt.Errorf("no end marker")
			}
			j := len(line) - 1
			for j >= 0 && isPySpace(line[j]) {
				j--
			}
			if j < 0 {
				j = 0
			}
			end = line[j]
			haveEnd = true
		}
		line = stripEndMarker(line, end)
		if len(line) > width {
			width = len(line)
		}
		rows = append(rows, line)
	}
	return width, rows, nil
}

func allEmpty(rows [][]rune) bool {
	for _, r := range rows {
		if len(r) != 0 {
			return false
		}
	}
	return true
}

func parseHex(s []rune) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	for _, r := range s {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !ok {
			return 0, false
		}
	}
	v, err := strconv.ParseInt(string(s), 16, 64)
	if err != nil {
		return 0, false
	}
	return int(v), true
}

func parseFont(raw []byte) (*figFont, error) {
	text := []rune(string(raw))
	for i, r := range text {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			text[i] = ' '
		}
	}
	lines := splitLines(text)
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font")
	}
	header := lines[0]
	lines = lines[1:]
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') ||
		header[1] != 'l' || header[2] != 'f' || header[3] != '2' {
		return nil, fmt.Errorf("not a valid figlet font")
	}
	fields := strings.FieldsFunc(string(header[5:]), isPySpace)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header")
	}
	f := &figFont{
		hardBlank: fields[0],
		printDir:  -1,
		chars:     map[int][][]rune{},
		widths:    map[int]int{},
	}
	nums := make([]int, 5)
	for i := 0; i < 5; i++ {
		n, err := strconv.Atoi(fields[1+i])
		if err != nil {
			return nil, err
		}
		nums[i] = n
	}
	height := nums[0]
	oldLayout := nums[3]
	commentLines := nums[4]
	if len(fields) > 6 {
		n, err := strconv.Atoi(fields[6])
		if err != nil {
			return nil, err
		}
		f.printDir = n
	}
	fullLayout := 0
	haveFull := false
	if len(fields) > 7 {
		n, err := strconv.Atoi(fields[7])
		if err != nil {
			return nil, err
		}
		fullLayout = n
		haveFull = true
	}
	if !haveFull {
		if oldLayout == 0 {
			fullLayout = 64
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | 128
		}
	}
	f.height = height
	f.smushMode = fullLayout

	p := &fontParser{lines: lines}
	for i := 0; i < commentLines; i++ {
		if _, err := p.next(); err != nil {
			return nil, err
		}
	}

	for i := 32; i < 127; i++ {
		w, letter, err := p.loadChar(height)
		if err != nil {
			return nil, err
		}
		if i == 32 || !allEmpty(letter) {
			f.chars[i] = letter
			f.widths[i] = w
		}
	}

	if p.pos < len(p.lines) {
		for _, c := range []rune("\u00c4\u00d6\u00dc\u00e4\u00f6\u00fc\u00df") {
			w, letter, err := p.loadChar(height)
			if err != nil {
				return nil, err
			}
			if !allEmpty(letter) {
				f.chars[int(c)] = letter
				f.widths[int(c)] = w
			}
		}
	}

	for p.pos < len(p.lines) {
		line := stripPy(p.lines[p.pos])
		p.pos++
		tok := line
		for k, r := range line {
			if r == ' ' {
				tok = line[:k]
				break
			}
		}
		if len(tok) == 0 {
			continue
		}
		if len(tok) >= 2 && tok[0] == '0' && (tok[1] == 'x' || tok[1] == 'X') {
			code, ok := parseHex(tok[2:])
			if !ok {
				return nil, fmt.Errorf("bad character code")
			}
			w, letter, err := p.loadChar(height)
			if err != nil {
				return nil, err
			}
			if !allEmpty(letter) {
				f.chars[code] = letter
				f.widths[code] = w
			}
		}
	}
	return f, nil
}

// ---------------------------------------------------------------------------
// Rendering (port of FigletBuilder)
// ---------------------------------------------------------------------------

type marker struct {
	buf [][]rune
	it  int
}

type builder struct {
	text      []int
	direction string
	width     int
	font      *figFont
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

func emptyBuf(h int) [][]rune {
	b := make([][]rune, h)
	for i := range b {
		b[i] = []rune{}
	}
	return b
}

func copyBuf(b [][]rune) [][]rune {
	c := make([][]rune, len(b))
	copy(c, b)
	return c
}

func newBuilder(text []rune, f *figFont, direction string, width int, justify string) *builder {
	t := make([]int, len(text))
	for i, r := range text {
		t[i] = int(r)
	}
	return &builder{
		text:      t,
		direction: direction,
		width:     width,
		font:      f,
		justify:   justify,
		buffer:    emptyBuf(f.height),
	}
}

func (b *builder) charAt(i int) ([][]rune, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	rows, ok := b.font.chars[b.text[i]]
	return rows, ok
}

func (b *builder) appendQueue(buf [][]rune) {
	if len(b.queue) > 1000000 {
		fail("runaway output")
	}
	b.queue = append(b.queue, buf)
}

func (b *builder) addCharToProduct() {
	curChar, haveChar := b.charAt(b.iterator)
	ch := b.text[b.iterator]

	if ch == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}

	if !haveChar {
		return
	}
	w := b.font.widths[ch]
	if b.width < w {
		fail("width is not enough to print this character")
	}
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, curChar)

	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if ch == ' ' {
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

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.cutBufferAtLastBlank(m.buf, m.it)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *builder) cutBufferAtLastBlank(saved [][]rune, savedIterator int) {
	b.appendQueue(saved)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *builder) cutBufferAtLastChar() {
	b.appendQueue(b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = emptyBuf(b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	cur, ok := b.charAt(b.iterator)
	if !ok {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, cur)
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
		if i >= len(addRight) {
			fail("index out of range")
		}
		right := addRight[i]
		smushed, ok := b.smushChars(left, right)
		addLeft = updateSmushed(addLeft, idx, smushed, ok)
	}
	return addLeft, addRight
}

func updateSmushed(addLeft []rune, idx int, smushed rune, ok bool) []rune {
	if idx < 0 || idx > len(addLeft) {
		return addLeft
	}
	if idx == len(addLeft) {
		fail("index out of range")
	}
	if !ok {
		fail("cannot join None")
	}
	out := make([]rune, len(addLeft))
	copy(out, addLeft)
	out[idx] = smushed
	return out
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	b.buffer[row] = concat(addLeft, sliceFrom(addRight, b.maxSmush))
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

		linebd := rstripSpaceLen(lineLeft) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := noChar
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}

		charbd := lstripSpaceCount(lineRight)
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

type smushPair struct{ a, b string }

func (b *builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}

	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}

	sm := b.font.smushMode
	if sm&smSmush == 0 {
		return 0, false
	}

	hb := b.font.hardBlank
	isHB := func(r rune) bool { return r != noChar && string(r) == hb }

	if sm&63 == 0 {
		if isHB(left) {
			return right, true
		}
		if isHB(right) {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}

	if sm&smHardblank != 0 {
		if isHB(left) && isHB(right) {
			return left, true
		}
	}

	if isHB(left) || isHB(right) {
		return 0, false
	}

	if sm&smEqual != 0 {
		if left == right {
			return left, true
		}
	}

	var smushes []smushPair
	if sm&smLowline != 0 {
		smushes = append(smushes, smushPair{"_", "|/\\[]{}()<>"})
	}
	if sm&smHierarchy != 0 {
		smushes = append(smushes,
			smushPair{"|", "/\\[]{}()<>"},
			smushPair{"\\/", "[]{}()<>"},
			smushPair{"[]", "{}()<>"},
			smushPair{"{}", "()<>"},
			smushPair{"()", "<>"},
		)
	}
	for _, p := range smushes {
		if strings.ContainsRune(p.a, left) && strings.ContainsRune(p.b, right) {
			return right, true
		}
		if strings.ContainsRune(p.a, right) && strings.ContainsRune(p.b, left) {
			return left, true
		}
	}

	if sm&smPair != 0 {
		l, r := string(left), string(right)
		for _, pr := range []string{l + r, r + l} {
			if pr == "[]" || pr == "{}" || pr == "()" {
				return '|', true
			}
		}
	}

	if sm&smBigX != 0 {
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

func (b *builder) justifyRows(buf [][]rune) [][]rune {
	switch b.justify {
	case "right":
		for row := 0; row < b.font.height; row++ {
			buf[row] = padLeft(buf[row], b.width-len(buf[row])-1)
		}
	case "center":
		for row := 0; row < b.font.height; row++ {
			buf[row] = padLeft(buf[row], (b.width-len(buf[row]))/2)
		}
	}
	return buf
}

func (b *builder) replaceHardblanks(buf [][]rune) string {
	var sb strings.Builder
	for _, row := range buf {
		sb.WriteString(string(row))
		sb.WriteByte('\n')
	}
	return strings.ReplaceAll(sb.String(), b.font.hardBlank, " ")
}

func (b *builder) returnProduct() string {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		buf = b.justifyRows(buf)
		sb.WriteString(b.replaceHardblanks(buf))
	}
	return sb.String()
}

func render(text []rune, f *figFont, direction string, width int, justify string) string {
	b := newBuilder(text, f, direction, width, justify)
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	return b.returnProduct()
}

// ---------------------------------------------------------------------------
// FigletString operations: reverse, flip, strip / normalize newlines
// ---------------------------------------------------------------------------

var reverseMap, flipMap [256]rune

func init() {
	for i := range reverseMap {
		reverseMap[i] = rune(i)
		flipMap[i] = rune(i)
	}
	rev := []string{
		" !\"#$%&')(*+,-.\\",
		"0123456789:;>=<?",
		"@ABCDEFGHIJKLMNO",
		"PQRSTUVWXYZ]/[^_",
		"`abcdefghijklmno",
		"pqrstuvwxyz}|{~\x7f",
	}
	flp := []string{
		" !\"#$%&'()*+,-.\\",
		"0123456789:;<=>?",
		"@VBCDEFGHIJKLWNO",
		"bQbSTUAMXYZ[/]v-",
		"`aPcdefghijklwno",
		"pqrstu^mxyz{|}~\x7f",
	}
	for seg, s := range rev {
		for k, c := range []rune(s) {
			reverseMap[0x20+16*seg+k] = c
		}
	}
	for seg, s := range flp {
		for k, c := range []rune(s) {
			flipMap[0x20+16*seg+k] = c
		}
	}
}

func translateRow(row []rune, table *[256]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if r >= 0 && r < 256 {
			out[i] = table[r]
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
	var out [][]rune
	for _, row := range splitLines(s) {
		t := translateRow(row, &reverseMap)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		out = append(out, t)
	}
	return newFromList(out)
}

func flipText(s []rune) []rune {
	rows := splitLines(s)
	var out [][]rune
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translateRow(rows[i], &flipMap))
	}
	return newFromList(out)
}

func stripSurrounding(s []rune) []rune {
	var out [][]rune
	seen := false
	for _, row := range splitLines(s) {
		if len(stripPy(row)) != 0 || seen {
			seen = true
			out = append(out, row)
		}
	}
	return rstripPy(newFromList(out))
}

func normalizeSurrounding(s []rune) []rune {
	inner := stripSurrounding(s)
	out := make([]rune, 0, len(inner)+2)
	out = append(out, '\n')
	out = append(out, inner...)
	return append(out, '\n')
}

// ---------------------------------------------------------------------------
// Command line (a small subset of Python's optparse behaviour)
// ---------------------------------------------------------------------------

type options struct {
	font      string
	direction string
	justify   string
	width     int
	reverse   bool
	normalize bool
	strip     bool
	flip      bool
}

var longNames = []string{
	"font", "direction", "justify", "width", "reverse",
	"normalize-surrounding-newlines", "strip-surrounding-newlines", "flip",
	"list_fonts", "info_font", "load", "color",
}

func shortToLong(c byte) string {
	switch c {
	case 'f':
		return "font"
	case 'D':
		return "direction"
	case 'j':
		return "justify"
	case 'w':
		return "width"
	case 'r':
		return "reverse"
	case 'n':
		return "normalize-surrounding-newlines"
	case 's':
		return "strip-surrounding-newlines"
	case 'F':
		return "flip"
	case 'l':
		return "list_fonts"
	case 'i':
		return "info_font"
	case 'L':
		return "load"
	case 'c':
		return "color"
	}
	return ""
}

func needsValue(long string) bool {
	switch long {
	case "font", "direction", "justify", "width", "load", "color":
		return true
	}
	return false
}

func matchLong(name string) (string, error) {
	var found []string
	for _, n := range longNames {
		if n == name {
			return n, nil
		}
		if strings.HasPrefix(n, name) {
			found = append(found, n)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	return "", fmt.Errorf("no such option: --%s", name)
}

func applyOption(o *options, long, val string) error {
	switch long {
	case "font":
		o.font = val
	case "direction":
		if val != "auto" && val != "left-to-right" && val != "right-to-left" {
			return fmt.Errorf("invalid direction: %s", val)
		}
		o.direction = val
	case "justify":
		if val != "auto" && val != "left" && val != "center" && val != "right" {
			return fmt.Errorf("invalid justify: %s", val)
		}
		o.justify = val
	case "width":
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return fmt.Errorf("invalid width: %s", val)
		}
		o.width = n
	case "reverse":
		o.reverse = true
	case "normalize-surrounding-newlines":
		o.normalize = true
	case "strip-surrounding-newlines":
		o.strip = true
	case "flip":
		o.flip = true
	}
	return nil
}

func parseArgs(args []string) (options, []string, error) {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			return o, rest, nil
		}
		if strings.HasPrefix(a, "--") {
			name := a[2:]
			val := ""
			hasVal := false
			if k := strings.Index(name, "="); k >= 0 {
				val = name[k+1:]
				name = name[:k]
				hasVal = true
			}
			long, err := matchLong(name)
			if err != nil {
				return o, nil, err
			}
			if needsValue(long) && !hasVal {
				if i+1 >= len(args) {
					return o, nil, fmt.Errorf("option --%s requires an argument", long)
				}
				i++
				val = args[i]
			}
			if err := applyOption(&o, long, val); err != nil {
				return o, nil, err
			}
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			for k := 1; k < len(a); k++ {
				long := shortToLong(a[k])
				if long == "" {
					return o, nil, fmt.Errorf("no such option: -%c", a[k])
				}
				if needsValue(long) {
					val := a[k+1:]
					if val == "" {
						if i+1 >= len(args) {
							return o, nil, fmt.Errorf("option -%c requires an argument", a[k])
						}
						i++
						val = args[i]
					}
					if err := applyOption(&o, long, val); err != nil {
						return o, nil, err
					}
					break
				}
				if err := applyOption(&o, long, ""); err != nil {
					return o, nil, err
				}
			}
			continue
		}
		rest = append(rest, a)
	}
	return o, rest, nil
}

// ---------------------------------------------------------------------------

func run(args []string) int {
	o, rest, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(rest) == 0 {
		fmt.Println("Usage: figlet [options] [text..]")
		return 1
	}
	text := []rune(strings.Join(rest, " "))

	raw, ok := readFontFile(o.font)
	if ok {
		raw = unzipFirst(raw)
	}
	if !ok || len(raw) == 0 {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", o.font)
		return 1
	}
	f, err := parseFont(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "problem parsing font:", err)
		return 1
	}

	direction := o.direction
	if direction == "auto" {
		direction = "left-to-right"
		if f.printDir == 1 {
			direction = "right-to-left"
		}
	}
	justify := o.justify
	if justify == "auto" {
		if direction == "left-to-right" {
			justify = "left"
		} else if direction == "right-to-left" {
			justify = "right"
		}
	}

	r := []rune(render(text, f, direction, o.width, justify))
	if o.reverse {
		r = reverseText(r)
	}
	if o.flip {
		r = flipText(r)
	}
	if o.strip {
		r = stripSurrounding(r)
	} else if o.normalize {
		r = normalizeSurrounding(r)
	}

	out := append([]byte(string(r)), '\n')
	os.Stdout.Write(out)
	return 0
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			os.Exit(1)
		}
	}()
	os.Exit(run(os.Args[1:]))
}
GOEOF

# the repository must contain no Python files
find /workspace/repo -name '*.py' -delete
