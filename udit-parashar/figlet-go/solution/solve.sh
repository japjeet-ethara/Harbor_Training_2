#!/bin/bash
# Oracle solution: a Go port of pyfiglet's FIGlet rendering (FigletFont parsing, FigletBuilder
# smushing / wrapping / justification, FigletString reverse / flip / strip / normalize, and the
# option handling of main()). Standard library only.
set -euo pipefail
cd /workspace/repo

# 1. fonts: keep the .flf files inside the repository, in a folder that is embedded with go:embed
mkdir -p fonts
cp pyfiglet/fonts-standard/*.flf fonts/

# 2. the Go source
cat > go.mod <<'__GO_SOURCE_EOF__'
module figlet

go 1.21
__GO_SOURCE_EOF__

cat > main.go <<'__GO_SOURCE_EOF__'
package main

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var fontFS embed.FS

const usage = "Usage: figlet [options] [text..]\n"

type options struct {
	font      string
	direction string
	justify   string
	width     int
	reverse   bool
	flip      bool
	strip     bool
	normalize bool
}

func die(code int, format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(code)
}

var longNames = []string{
	"font", "direction", "justify", "width", "reverse", "normalize-surrounding-newlines",
	"strip-surrounding-newlines", "flip",
}

func longToShort(name string) (byte, bool) {
	m := map[string]byte{
		"font": 'f', "direction": 'D', "justify": 'j', "width": 'w', "reverse": 'r',
		"normalize-surrounding-newlines": 'n', "strip-surrounding-newlines": 's', "flip": 'F',
	}
	if c, ok := m[name]; ok {
		return c, true
	}
	var hit []string
	for _, n := range longNames {
		if strings.HasPrefix(n, name) {
			hit = append(hit, n)
		}
	}
	if len(hit) == 1 {
		return m[hit[0]], true
	}
	return 0, false
}

func parseArgs(args []string) (options, []string) {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var pos []string
	set := func(c byte, val string) {
		switch c {
		case 'f':
			o.font = val
		case 'D':
			if val != "auto" && val != "left-to-right" && val != "right-to-left" {
				die(2, "figlet: error: option -D: invalid choice: %q", val)
			}
			o.direction = val
		case 'j':
			if val != "auto" && val != "left" && val != "center" && val != "right" {
				die(2, "figlet: error: option -j: invalid choice: %q", val)
			}
			o.justify = val
		case 'w':
			n, err := strconv.Atoi(val)
			if err != nil {
				die(2, "figlet: error: option -w: invalid integer value: %q", val)
			}
			o.width = n
		}
	}
	takesValue := func(c byte) bool { return c == 'f' || c == 'D' || c == 'j' || c == 'w' }
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			pos = append(pos, args[i+1:]...)
			return o, pos
		case strings.HasPrefix(a, "--"):
			name, val, hasEq := strings.Cut(a[2:], "=")
			c, ok := longToShort(name)
			if !ok {
				die(2, "figlet: error: no such option: %s", a)
			}
			if takesValue(c) {
				if !hasEq {
					i++
					if i >= len(args) {
						die(2, "figlet: error: %s option requires an argument", a)
					}
					val = args[i]
				}
				set(c, val)
			} else {
				applyFlag(&o, c)
			}
		case len(a) > 1 && a[0] == '-':
			for k := 1; k < len(a); k++ {
				c := a[k]
				if takesValue(c) {
					val := a[k+1:]
					if val == "" {
						i++
						if i >= len(args) {
							die(2, "figlet: error: -%c option requires an argument", c)
						}
						val = args[i]
					}
					set(c, val)
					break
				}
				if c != 'r' && c != 'F' && c != 's' && c != 'n' {
					die(2, "figlet: error: no such option: -%c (not supported by this port)", c)
				}
				applyFlag(&o, c)
			}
		default:
			pos = append(pos, a)
		}
	}
	return o, pos
}

func applyFlag(o *options, c byte) {
	switch c {
	case 'r':
		o.reverse = true
	case 'F':
		o.flip = true
	case 's':
		o.strip = true
	case 'n':
		o.normalize = true
	}
}

func loadFont(name string) (*Font, bool) {
	for _, ext := range []string{"tlf", "flf"} {
		data, err := fontFS.ReadFile("fonts/" + name + "." + ext)
		if err == nil {
			f, perr := parseFont(name, data)
			if perr != nil {
				die(1, "pyfiglet: %v", perr)
			}
			return f, true
		}
	}
	return nil, false
}

func run() int {
	o, pos := parseArgs(os.Args[1:])
	if len(pos) == 0 {
		fmt.Print(usage)
		return 1
	}
	text := strings.Join(pos, " ")

	font, ok := loadFont(o.font)
	if !ok {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", o.font)
		return 1
	}

	direction := o.direction
	if direction == "auto" {
		direction = "left-to-right"
		if font.printDirection == 1 {
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

	r := newBuilder(text, font, direction, o.width, justify).render()
	if o.reverse {
		r = reverseString(r)
	}
	if o.flip {
		r = flipString(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = normalizeSurroundingNewlines(r)
	}

	w := bufio.NewWriter(os.Stdout)
	w.WriteString(r)
	w.WriteString("\n")
	w.Flush()
	return 0
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(pyError); ok {
				fmt.Fprintln(os.Stderr, "pyfiglet:", e.msg)
			} else {
				fmt.Fprintln(os.Stderr, "pyfiglet: internal error:", r)
			}
			os.Exit(1)
		}
	}()
	os.Exit(run())
}
__GO_SOURCE_EOF__

cat > font.go <<'__GO_SOURCE_EOF__'
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
__GO_SOURCE_EOF__

cat > render.go <<'__GO_SOURCE_EOF__'
package main

import "strings"

// Port of pyfiglet.FigletBuilder, FigletRenderingEngine and FigletString.
// Rows are never modified in place; every change builds a new slice, like Python strings.
// A rune value of -1 stands for Python's empty string '' (a missing neighbour character).

const none rune = -1

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
	product      [][][]rune
	buffer       [][]rune
}

type pyError struct{ msg string }

func fail(msg string) { panic(pyError{msg}) }

func newBuilder(text string, font *Font, direction string, width int, justify string) *builder {
	return &builder{
		text: []rune(text), direction: direction, width: width, font: font, justify: justify,
		buffer: make([][]rune, font.height),
	}
}

func copyBuf(b [][]rune) [][]rune {
	c := make([][]rune, len(b))
	copy(c, b)
	return c
}

func (b *builder) rtl() bool { return b.direction == "right-to-left" }

func (b *builder) getCharAt(i int) ([][]rune, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	g, ok := b.font.chars[b.text[i]]
	return g, ok
}

func (b *builder) getCurWidth() int { return b.font.width[b.text[b.iterator]] }

func inSet(c rune, set string) bool { // Python: c in set (a substring test; '' is always "in")
	return c == none || strings.ContainsRune(set, c)
}

func (b *builder) isHB(c rune) bool { return c != none && string(c) == b.font.hardBlank }

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
	m := b.font.smushMode
	if m&smSmush == 0 {
		return 0, false
	}
	if m&63 == 0 {
		if b.isHB(left) {
			return right, true
		}
		if b.isHB(right) {
			return left, true
		}
		if b.rtl() {
			return left, true
		}
		return right, true
	}
	if m&smHardblank != 0 {
		if b.isHB(left) && b.isHB(right) {
			return left, true
		}
	}
	if b.isHB(left) || b.isHB(right) {
		return 0, false
	}
	if m&smEqual != 0 && left == right {
		return left, true
	}
	type pair struct{ a, b string }
	var smushes []pair
	if m&smLowline != 0 {
		smushes = append(smushes, pair{"_", "|/\\[]{}()<>"})
	}
	if m&smHierarchy != 0 {
		smushes = append(smushes,
			pair{"|", "/\\[]{}()<>"},
			pair{"\\/", "[]{}()<>"},
			pair{"[]", "{}()<>"},
			pair{"{}", "()<>"},
			pair{"()", "<>"},
		)
	}
	for _, p := range smushes {
		if inSet(left, p.a) && inSet(right, p.b) {
			return right, true
		}
		if inSet(right, p.a) && inSet(left, p.b) {
			return left, true
		}
	}
	if m&smPair != 0 && left != none {
		for _, s := range []string{string(left) + string(right), string(right) + string(left)} {
			if s == "[]" || s == "{}" || s == "()" {
				return '|', true
			}
		}
	}
	if m&smBigX != 0 {
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

func rstripSpace(r []rune) []rune {
	n := len(r)
	for n > 0 && r[n-1] == ' ' {
		n--
	}
	return r[:n]
}

func lstripSpaceLen(r []rune) int {
	i := 0
	for i < len(r) && r[i] == ' ' {
		i++
	}
	return len(r) - i
}

func (b *builder) smushAmount(buffer, curChar [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft, lineRight := buffer[row], curChar[row]
		if b.rtl() {
			lineLeft, lineRight = lineRight, lineLeft
		}
		linebd := len(rstripSpace(lineLeft)) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := none
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}
		charbd := len(lineRight) - lstripSpaceLen(lineRight)
		ch2 := none
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
		}
		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == none || ch1 == ' ' {
			amt++
		} else if ch2 != none {
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

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft := append([]rune(nil), b.buffer[row]...)
	addRight := append([]rune(nil), curChar[row]...)
	if b.rtl() {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := none
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		if i >= len(addRight) {
			fail("IndexError: string index out of range")
		}
		right := addRight[i]
		smushed, ok := b.smushChars(left, right)
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if idx == len(addLeft) {
			fail("IndexError: list assignment index out of range")
		}
		if !ok {
			fail("TypeError: sequence item: expected str instance, NoneType found")
		}
		addLeft[idx] = smushed
	}
	rest := addRight
	if b.maxSmush <= len(addRight) {
		rest = addRight[b.maxSmush:]
	} else {
		rest = nil
	}
	b.buffer[row] = append(addLeft, rest...)
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([][]rune, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	// Python also recomputes maxSmush here, but the value is always recomputed before use.
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.product = append(b.product, m.buf)
		b.iterator = m.it
		b.cutBufferCommon()
	} else {
		b.product = append(b.product, b.buffer)
		b.iterator--
		b.cutBufferCommon()
	}
	if len(b.product) > 5000000 {
		fail("output too large (the original loops forever here)")
	}
}

func (b *builder) addCharToProduct() {
	curChar, ok := b.getCharAt(b.iterator)
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if !ok {
		return
	}
	if b.width < b.getCurWidth() {
		fail("CharNotPrinted: Width is not enough to print this character")
	}
	b.curCharWidth = b.getCurWidth()
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush
	if b.text[b.iterator] == ' ' {
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

func (b *builder) justifyBuffer(buffer [][]rune) [][]rune {
	out := make([][]rune, len(buffer))
	for row := range buffer {
		pad := 0
		switch b.justify {
		case "right":
			pad = b.width - len(buffer[row]) - 1
		case "center":
			if d := b.width - len(buffer[row]); d > 0 {
				pad = d / 2
			}
		}
		if pad < 0 {
			pad = 0
		}
		out[row] = append([]rune(strings.Repeat(" ", pad)), buffer[row]...)
	}
	return out
}

func (b *builder) render() string {
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	if len(b.buffer[0]) != 0 {
		b.product = append(b.product, b.buffer)
	}
	var acc strings.Builder
	for _, buf := range b.product {
		buf = b.justifyBuffer(buf)
		rows := make([]string, len(buf))
		for i, r := range buf {
			rows[i] = string(r)
		}
		s := strings.Join(rows, "\n") + "\n"
		acc.WriteString(strings.ReplaceAll(s, b.font.hardBlank, " "))
	}
	return acc.String()
}

// ---- FigletString methods ------------------------------------------------

func translate(row string, table map[rune]rune) string {
	rs := []rune(row)
	for i, r := range rs {
		if t, ok := table[r]; ok {
			rs[i] = t
		}
	}
	return string(rs)
}

var reverseMap = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '<': '>', '>': '<', '[': ']', '\\': '/', ']': '[', '{': '}', '}': '{',
}

var flipMap = map[rune]rune{
	'/': '\\', 'A': 'V', 'M': 'W', 'P': 'b', 'R': 'b', 'V': 'A', 'W': 'M', '\\': '/', '^': 'v', '_': '-',
	'b': 'P', 'm': 'w', 'v': '^', 'w': 'm',
}

func newFromList(rows []string) string { return strings.Join(rows, "\n") + "\n" }

func reverseString(s string) string {
	var out []string
	for _, row := range pySplitLines(s) {
		rs := []rune(translate(row, reverseMap))
		for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
			rs[i], rs[j] = rs[j], rs[i]
		}
		out = append(out, string(rs))
	}
	return newFromList(out)
}

func flipString(s string) string {
	lines := pySplitLines(s)
	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		out = append(out, translate(lines[i], flipMap))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s string) string {
	var out []string
	seen := false
	for _, row := range pySplitLines(s) {
		if pyStrip(row) != "" || seen {
			seen = true
			out = append(out, row)
		}
	}
	return pyRstrip(newFromList(out))
}

func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}
__GO_SOURCE_EOF__

# 3. the Python is gone
find /workspace/repo -name '*.py' -delete

# 4. self-check, and make sure the files belong to whoever the verifier runs as
go build ./... && go build -o /tmp/figlet .
if [ "$(id -u)" = "0" ]; then chown -R player:player /workspace/repo 2>/dev/null || true; fi
chmod -R a+rX /workspace/repo
echo "oracle: Go port written and built"
