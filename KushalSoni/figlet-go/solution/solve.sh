#!/bin/bash
# Oracle for figlet-go: writes a Go port of pyfiglet into /workspace/repo.
# Runs as user "player" (owner of /workspace/repo); writes nothing outside it except /tmp.
set -euo pipefail
cd /workspace/repo

# 1. Fonts: move the .flf files to ./fonts (embedded into the binary with go:embed).
mkdir -p fonts
for f in pyfiglet/fonts/*.flf; do mv -f "$f" fonts/; done

# 2. Go sources (standard library only).
cat > go.mod <<'FIGLET_GO_GO_MOD_EOF'
module figlet

go 1.23
FIGLET_GO_GO_MOD_EOF

cat > pyutil.go <<'FIGLET_GO_PYUTIL_GO_EOF'
package main

import "unicode"

// Helpers that reproduce Python str semantics on []rune.

// pyIsSpace mirrors str.isspace() (what Python's re \s, strip() and split() use).
func pyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85:
		return true
	}
	return r > 0x7f && unicode.IsSpace(r)
}

// isLineBreak mirrors the boundaries used by str.splitlines().
func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// splitLines mirrors str.splitlines() (no keepends).
func splitLines(s []rune) [][]rune {
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

// strip / rstrip with Python's default whitespace set.
func pyStrip(s []rune) []rune { return pyLStrip(pyRStrip(s)) }
func pyRStrip(s []rune) []rune {
	e := len(s)
	for e > 0 && pyIsSpace(s[e-1]) {
		e--
	}
	return s[:e]
}
func pyLStrip(s []rune) []rune {
	b := 0
	for b < len(s) && pyIsSpace(s[b]) {
		b++
	}
	return s[b:]
}

// rstrip(' ') / lstrip(' ') — ASCII space only.
func rstripSpace(s []rune) []rune {
	e := len(s)
	for e > 0 && s[e-1] == ' ' {
		e--
	}
	return s[:e]
}
func lstripSpace(s []rune) []rune {
	b := 0
	for b < len(s) && s[b] == ' ' {
		b++
	}
	return s[b:]
}

// pySplit mirrors str.split() with no separator.
func pySplit(s []rune) [][]rune {
	var out [][]rune
	i := 0
	for i < len(s) {
		for i < len(s) && pyIsSpace(s[i]) {
			i++
		}
		if i >= len(s) {
			break
		}
		j := i
		for j < len(s) && !pyIsSpace(s[j]) {
			j++
		}
		out = append(out, s[i:j])
		i = j
	}
	return out
}

func allSpace(s []rune) bool {
	for _, r := range s {
		if !pyIsSpace(r) {
			return false
		}
	}
	return true
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cloneRunes(s []rune) []rune { return append([]rune(nil), s...) }

func cloneRows(rows [][]rune) [][]rune {
	out := make([][]rune, len(rows))
	for i, r := range rows {
		out[i] = cloneRunes(r)
	}
	return out
}
FIGLET_GO_PYUTIL_GO_EOF

cat > font.go <<'FIGLET_GO_FONT_GO_EOF'
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
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

var fontDirectories = []string{"./", "/usr/local/share/pyfiglet/", "/usr/local/share/figlet", "/usr/share/figlet"}

// Layout bits (figlet222).
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

var errFontNotFound = errors.New("font not found")

// Font mirrors pyfiglet.FigletFont.
type Font struct {
	name           string
	height         int
	hardBlank      []rune // header[0]; normally one rune
	printDirection int    // -1 when the header has no print-direction field
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

func (f *Font) isHardBlank(r rune) bool {
	return len(f.hardBlank) == 1 && r == f.hardBlank[0]
}

// preloadFont mirrors FigletFont.preloadFont: tlf, then flf; package fonts first.
func preloadFont(font string) ([]byte, error) {
	var data []byte
	found := false
	for _, ext := range []string{"tlf", "flf"} {
		fn := font + "." + ext
		if b, err := embeddedFonts.ReadFile("fonts/" + fn); err == nil && !strings.Contains(font, "/") {
			data, found = b, true
			break
		}
		for _, loc := range fontDirectories {
			p := filepath.Join(loc, fn)
			if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
				if b, err := os.ReadFile(p); err == nil {
					data, found = b, true
				}
				break
			}
		}
	}
	if !found {
		return nil, errFontNotFound
	}
	if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil && len(zr.File) > 0 {
		rc, err := zr.File[0].Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err = io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
	}
	if len(data) == 0 {
		return nil, errFontNotFound
	}
	return data, nil
}

// LoadFont mirrors FigletFont.__init__ + loadFont.
func LoadFont(name string) (*Font, error) {
	raw, err := preloadFont(name)
	if err != nil {
		return nil, err
	}
	f := &Font{name: name, chars: map[int][][]rune{}, width: map[int]int{}, printDirection: -1}
	if err := f.parse([]rune(string(raw))); err != nil {
		return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
	}
	return f, nil
}

type lineQueue struct{ lines [][]rune }

func (q *lineQueue) empty() bool { return len(q.lines) == 0 }
func (q *lineQueue) pop() ([]rune, error) {
	if len(q.lines) == 0 {
		return nil, errors.New("pop from empty list")
	}
	l := q.lines[0]
	q.lines = q.lines[1:]
	return l, nil
}

func (f *Font) parse(data []rune) error {
	for i, r := range data {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			data[i] = ' '
		}
	}
	q := &lineQueue{lines: splitLines(data)}

	header, err := q.pop()
	if err != nil {
		return err
	}
	// ^[tf]lf2.
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || header[1] != 'l' ||
		header[2] != 'f' || header[3] != '2' || header[4] == '\n' {
		return fmt.Errorf("%s is not a valid figlet font", f.name)
	}
	fields := pySplit(header[5:])
	if len(fields) < 6 {
		return fmt.Errorf("malformed header for %s", f.name)
	}
	f.hardBlank = fields[0]
	nums := make([]int, 0, 7)
	for _, fld := range fields[1:] {
		if len(nums) == 7 {
			break
		}
		n, err := pyInt(string(fld), 10)
		if err != nil {
			if len(nums) < 5 {
				return err
			}
			// optional fields: Python would raise too
			return err
		}
		nums = append(nums, n)
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	fullLayout, haveFull := 0, false
	if len(nums) > 5 {
		f.printDirection = nums[5]
	}
	if len(nums) > 6 {
		fullLayout, haveFull = nums[6], true
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
		if _, err := q.pop(); err != nil {
			return err
		}
	}

	readChar := func() (int, [][]rune, error) {
		var endRune rune
		haveEnd := false
		width := 0
		rows := make([][]rune, 0, max(height, 0))
		for i := 0; i < height; i++ {
			line, err := q.pop()
			if err != nil {
				return 0, nil, err
			}
			if !haveEnd {
				// reEndMarker = (.)\s*$
				p := len(line) - 1
				for p >= 0 && pyIsSpace(line[p]) {
					p--
				}
				if p < 0 {
					if len(line) == 0 {
						return 0, nil, errors.New("'NoneType' object has no attribute 'group'")
					}
					p = 0
				}
				endRune, haveEnd = line[p], true
			}
			line = stripEndMarker(line, endRune)
			if len(line) > width {
				width = len(line)
			}
			rows = append(rows, line)
		}
		return width, rows, nil
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
		w, letter, err := readChar()
		if err != nil {
			return err
		}
		if i == 32 || nonEmpty(letter) {
			f.chars[i] = letter
			f.width[i] = w
		}
	}
	if !q.empty() {
		for _, c := range []rune("ÄÖÜäöüß") {
			w, letter, err := readChar()
			if err != nil {
				return err
			}
			if nonEmpty(letter) {
				f.chars[int(c)] = letter
				f.width[int(c)] = w
			}
		}
	}
	for !q.empty() {
		line, _ := q.pop()
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
			n, err := pyInt(string(code), 16)
			if err != nil {
				return err
			}
			w, letter, err := readChar()
			if err != nil {
				return err
			}
			if nonEmpty(letter) {
				f.chars[n] = letter
				f.width[n] = w
			}
		}
	}
	return nil
}

// stripEndMarker mirrors re.sub(re.escape(end) + r'{1,2}\s*$', ”, line).
func stripEndMarker(line []rune, end rune) []rune {
	restSpace := func(from int) bool {
		for k := from; k < len(line); k++ {
			if !pyIsSpace(line[k]) {
				return false
			}
		}
		return true
	}
	for s := 0; s < len(line); s++ {
		if line[s] != end {
			continue
		}
		if s+1 < len(line) && line[s+1] == end && restSpace(s+2) {
			return line[:s]
		}
		if restSpace(s + 1) {
			return line[:s]
		}
	}
	return line
}

// pyInt mirrors int(s, base) for the inputs fonts contain (optional sign,
// optional 0x prefix when base is 16, surrounding whitespace, underscores).
func pyInt(s string, base int) (int, error) {
	t := strings.TrimFunc(s, pyIsSpace)
	neg := false
	if strings.HasPrefix(t, "+") || strings.HasPrefix(t, "-") {
		neg = t[0] == '-'
		t = t[1:]
	}
	if base == 16 && (strings.HasPrefix(t, "0x") || strings.HasPrefix(t, "0X")) {
		t = t[2:]
		t = strings.TrimPrefix(t, "_")
	}
	if t == "" || strings.HasPrefix(t, "_") || strings.HasSuffix(t, "_") || strings.Contains(t, "__") {
		return 0, fmt.Errorf("invalid literal for int() with base %d: %q", base, s)
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(t, "_", ""), base, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid literal for int() with base %d: %q", base, s)
	}
	if neg {
		n = -n
	}
	return int(n), nil
}

// fontNames lists the embedded fonts (used by -l).
func fontNames() []string {
	entries, _ := embeddedFonts.ReadDir("fonts")
	var names []string
	for _, e := range entries {
		names = append(names, strings.SplitN(e.Name(), ".", 2)[0])
	}
	return names
}
FIGLET_GO_FONT_GO_EOF

cat > render.go <<'FIGLET_GO_RENDER_GO_EOF'
package main

// Port of pyfiglet.FigletBuilder. Method names and order follow the Python.

// smush results: a rune, or one of these markers.
const (
	runeEmpty rune = -1 // Python ''
	runeNone  rune = -2 // Python None (cannot smush)
)

type pyCrash struct{ msg string }

type blankMarker struct {
	buffer   [][]rune
	iterator int
}

type builder struct {
	text      []int
	direction string
	width     int
	font      *Font
	justify   string

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []blankMarker
	queue        [][][]rune
	buffer       [][]rune

	// guard against pyfiglet's endless loop (a letter exactly as wide as -w)
	lastEmptyCut int
	emptyCuts    int
}

func newBuilder(text []rune, font *Font, direction string, width int, justify string) *builder {
	b := &builder{direction: direction, width: width, font: font, justify: justify, lastEmptyCut: -2}
	for _, r := range text {
		b.text = append(b.text, int(r))
	}
	b.buffer = emptyBuffer(font.height)
	return b
}

func emptyBuffer(h int) [][]rune {
	if h < 0 {
		h = 0
	}
	return make([][]rune, h)
}

func (b *builder) rtl() bool { return b.direction == "right-to-left" }

// render mirrors FigletRenderingEngine.render.
func (b *builder) render() string {
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	if len(b.buffer) > 0 && len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	return b.formatProduct()
}

func (b *builder) addCharToProduct() {
	curChar := b.getCharAt(b.iterator)

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneRows(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if curChar == nil {
		return
	}
	cw, _ := b.getCharWidthAt(b.iterator)
	if b.width < cw {
		panic(pyCrash{"pyfiglet.CharNotPrinted: Width is not enough to print this character"})
	}
	b.curCharWidth = cw
	b.maxSmush = b.smushAmount(b.buffer, curChar)

	b.currentTotalWidth = rowLen(b.buffer, 0) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneRows(b.buffer), b.iterator})
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

func rowLen(buf [][]rune, row int) int {
	if row < len(buf) {
		return len(buf[row])
	}
	panic(pyCrash{"IndexError: list index out of range"})
}

func (b *builder) getCharAt(i int) [][]rune {
	if i < 0 || i >= len(b.text) {
		return nil
	}
	c, ok := b.font.chars[b.text[i]]
	if !ok {
		return nil
	}
	return c
}

func (b *builder) getCharWidthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	w, ok := b.font.width[b.text[i]]
	return w, ok
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := cloneRunes(b.buffer[row])
	addRight := curChar[row]
	if b.rtl() {
		addLeft, addRight = cloneRunes(addRight), addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := runeEmpty
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		if i >= len(addRight) {
			panic(pyCrash{"IndexError: string index out of range"})
		}
		right := addRight[i]
		smushed := b.smushChars(left, right)
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if idx == len(addLeft) {
			panic(pyCrash{"IndexError: list assignment index out of range"})
		}
		switch smushed {
		case runeNone:
			panic(pyCrash{"TypeError: sequence item: expected str instance, NoneType found"})
		case runeEmpty:
			addLeft = append(addLeft[:idx:idx], addLeft[idx+1:]...)
		default:
			addLeft[idx] = smushed
		}
	}
	return addLeft, addRight
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	rest := []rune{}
	if b.maxSmush < len(addRight) {
		rest = addRight[b.maxSmush:]
	}
	nr := make([]rune, 0, len(addLeft)+len(rest))
	nr = append(nr, addLeft...)
	nr = append(nr, rest...)
	b.buffer[row] = nr
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = emptyBuffer(b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar := b.getCharAt(b.iterator)
	if curChar == nil {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

func (b *builder) cutBufferAtLastBlank(saved [][]rune, savedIterator int) {
	b.queue = append(b.queue, saved)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *builder) cutBufferAtLastChar() {
	empty := true
	for _, r := range b.buffer {
		if len(r) > 0 {
			empty = false
		}
	}
	if empty && b.iterator == b.lastEmptyCut {
		b.emptyCuts++
		if b.emptyCuts > 2 {
			panic(pyCrash{"endless loop (pyfiglet never terminates on this input)"})
		}
	} else if empty {
		b.lastEmptyCut, b.emptyCuts = b.iterator, 1
	}
	b.queue = append(b.queue, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) handleNewLine() {
	n := len(b.blankMarkers)
	if n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		if m.iterator != -1 {
			b.cutBufferAtLastBlank(m.buffer, m.iterator)
			return
		}
	}
	b.cutBufferAtLastChar()
}

func (b *builder) formatProduct() string {
	var out []rune
	for _, buf := range b.queue {
		buf = b.justifyString(buf)
		out = append(out, b.replaceHardblanks(buf)...)
	}
	return string(out)
}

func spaces(n int) []rune {
	if n <= 0 {
		return nil
	}
	s := make([]rune, n)
	for i := range s {
		s[i] = ' '
	}
	return s
}

func (b *builder) justifyString(buf [][]rune) [][]rune {
	out := cloneRows(buf)
	switch b.justify {
	case "right":
		for row := 0; row < b.font.height; row++ {
			out[row] = append(spaces(b.width-len(out[row])-1), out[row]...)
		}
	case "center":
		for row := 0; row < b.font.height; row++ {
			out[row] = append(spaces((b.width-len(out[row]))/2), out[row]...)
		}
	}
	return out
}

func (b *builder) replaceHardblanks(buf [][]rune) []rune {
	var s []rune
	for i, row := range buf {
		if i > 0 {
			s = append(s, '\n')
		}
		s = append(s, row...)
	}
	s = append(s, '\n')
	return replaceAll(s, b.font.hardBlank, []rune{' '})
}

func replaceAll(s, old, repl []rune) []rune {
	if len(old) == 0 {
		return s
	}
	var out []rune
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && runesEqual(s[i:i+len(old)], old) {
			out = append(out, repl...)
			i += len(old)
			continue
		}
		out = append(out, s[i])
		i++
	}
	return out
}

func (b *builder) smushAmount(buffer [][]rune, curChar [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.rtl() {
			lineLeft, lineRight = lineRight, lineLeft
		}

		linebd := len(rstripSpace(lineLeft)) - 1
		if linebd < 0 {
			linebd = 0
		}
		var ch1 rune
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
			ch1 = runeEmpty
		}

		charbd := len(lineRight) - len(lstripSpace(lineRight))
		var ch2 rune
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
			ch2 = runeEmpty
		}

		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == runeEmpty || ch1 == ' ' {
			amt++
		} else if ch2 != runeEmpty && b.smushChars(ch1, ch2) != runeNone {
			amt++
		}
		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

// smushChars mirrors FigletBuilder.smushChars. left may be runeEmpty (”).
func (b *builder) smushChars(left, right rune) rune {
	if left == ' ' {
		return right
	}
	if right == ' ' {
		return left
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return runeNone
	}
	mode := b.font.smushMode
	if mode&smSmush == 0 {
		return runeNone
	}
	hb := b.font.isHardBlank
	if mode&63 == 0 {
		if left != runeEmpty && hb(left) {
			return right
		}
		if hb(right) {
			return left
		}
		if b.rtl() {
			return left
		}
		return right
	}
	if mode&smHardblank != 0 {
		if left != runeEmpty && hb(left) && hb(right) {
			return left
		}
	}
	if (left != runeEmpty && hb(left)) || hb(right) {
		return runeNone
	}
	if mode&smEqual != 0 {
		if left == right {
			return left
		}
	}

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
	// Python `x in "..."` is a substring test; '' is in every string.
	in := func(r rune, s string) bool {
		if r == runeEmpty {
			return true
		}
		for _, c := range s {
			if c == r {
				return true
			}
		}
		return false
	}
	for _, c := range smushes {
		if in(left, c.a) && in(right, c.b) {
			return right
		}
		if in(right, c.a) && in(left, c.b) {
			return left
		}
	}

	if mode&smPair != 0 {
		l, r := string(left), string(right)
		if left == runeEmpty {
			l = ""
		}
		for _, pair := range []string{l + r, r + l} {
			if pair == "[]" || pair == "{}" || pair == "()" {
				return '|'
			}
		}
	}
	if mode&smBigX != 0 {
		if left == '/' && right == '\\' {
			return '|'
		}
		if right == '/' && left == '\\' {
			return 'Y'
		}
		if left == '>' && right == '<' {
			return 'X'
		}
	}
	return runeNone
}
FIGLET_GO_RENDER_GO_EOF

cat > figstring.go <<'FIGLET_GO_FIGSTRING_GO_EOF'
package main

// Port of pyfiglet.FigletString (reverse / flip / strip / normalize).

// Only characters below U+0100 are translated (str.translate with a 256-char table).
var reverseMap = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<',
	'[': ']', ']': '[', '{': '}', '}': '{',
}

var flipMap = map[rune]rune{
	'/': '\\', '\\': '/', 'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M', 'P': 'b', 'R': 'b',
	'b': 'P', '^': 'v', 'v': '^', '_': '-', 'm': 'w', 'w': 'm',
}

func translate(row []rune, table map[rune]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if t, ok := table[r]; ok {
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

func figReverse(s []rune) []rune {
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

func figFlip(s []rune) []rune {
	lines := splitLines(s)
	var rows [][]rune
	for i := len(lines) - 1; i >= 0; i-- {
		rows = append(rows, translate(lines[i], flipMap))
	}
	return newFromList(rows)
}

func figStripSurroundingNewlines(s []rune) []rune {
	var rows [][]rune
	seen := false
	for _, row := range splitLines(s) {
		if !allSpace(row) || seen {
			seen = true
			rows = append(rows, row)
		}
	}
	return pyRStrip(newFromList(rows))
}

func figNormalizeSurroundingNewlines(s []rune) []rune {
	out := []rune{'\n'}
	out = append(out, figStripSurroundingNewlines(s)...)
	return append(out, '\n')
}
FIGLET_GO_FIGSTRING_GO_EOF

cat > main.go <<'FIGLET_GO_MAIN_GO_EOF'
// figlet: a Go port of pyfiglet's command-line program (pyfiglet 255827ef).
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

const version = "1.0.4"

const usage = `Usage: pyfiglet [options] [text..]

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

type options struct {
	font, direction, justify string
	width                    int
	reverse, normalize       bool
	strip, flip              bool
	listFonts, infoFont      bool
	load, color              string
}

type optSpec struct {
	short, long string
	takesValue  bool
	dest        string
	choices     []string
}

var specs = []optSpec{
	{"", "--version", false, "version", nil},
	{"-h", "--help", false, "help", nil},
	{"-f", "--font", true, "font", nil},
	{"-D", "--direction", true, "direction", []string{"auto", "left-to-right", "right-to-left"}},
	{"-j", "--justify", true, "justify", []string{"auto", "left", "center", "right"}},
	{"-w", "--width", true, "width", nil},
	{"-r", "--reverse", false, "reverse", nil},
	{"-n", "--normalize-surrounding-newlines", false, "normalize", nil},
	{"-s", "--strip-surrounding-newlines", false, "strip", nil},
	{"-F", "--flip", false, "flip", nil},
	{"-l", "--list_fonts", false, "listFonts", nil},
	{"-i", "--info_font", false, "infoFont", nil},
	{"-L", "--load", true, "load", nil},
	{"-c", "--color", true, "color", nil},
}

func optError(msg string) {
	fmt.Fprintf(os.Stderr, "Usage: pyfiglet [options] [text..]\n\npyfiglet: error: %s\n", msg)
	os.Exit(2)
}

func findShort(c string) *optSpec {
	for i := range specs {
		if specs[i].short == "-"+c {
			return &specs[i]
		}
	}
	return nil
}

func findLong(name string) *optSpec {
	var matches []*optSpec
	for i := range specs {
		if specs[i].long == name {
			return &specs[i]
		}
		if strings.HasPrefix(specs[i].long, name) {
			matches = append(matches, &specs[i])
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) > 1 {
		var names []string
		for _, m := range matches {
			names = append(names, m.long)
		}
		optError(fmt.Sprintf("ambiguous option: %s (%s?)", name, strings.Join(names, ", ")))
	}
	optError("no such option: " + name)
	return nil
}

// parseNum mirrors optparse._parse_num(val, int).
func parseNum(val string) (int, error) {
	low := strings.ToLower(val)
	switch {
	case strings.HasPrefix(low, "0x"):
		return pyInt(val, 16)
	case strings.HasPrefix(low, "0b"):
		v := val[2:]
		if v == "" {
			v = "0"
		}
		return pyInt(v, 2)
	case strings.HasPrefix(val, "0"):
		return pyInt(val, 8)
	}
	return pyInt(val, 10)
}

func apply(o *options, s *optSpec, name, val string) {
	if s.choices != nil {
		ok := false
		for _, c := range s.choices {
			ok = ok || c == val
		}
		if !ok {
			quoted := make([]string, len(s.choices))
			for i, c := range s.choices {
				quoted[i] = "'" + c + "'"
			}
			optError(fmt.Sprintf("option %s: invalid choice: '%s' (choose from %s)", name, val, strings.Join(quoted, ", ")))
		}
	}
	switch s.dest {
	case "version":
		fmt.Println(version)
		os.Exit(0)
	case "help":
		fmt.Print(usage)
		os.Exit(0)
	case "font":
		o.font = val
	case "direction":
		o.direction = val
	case "justify":
		o.justify = val
	case "width":
		n, err := parseNum(val)
		if err != nil {
			optError(fmt.Sprintf("option %s: invalid integer value: '%s'", name, val))
		}
		o.width = n
	case "reverse":
		o.reverse = true
	case "normalize":
		o.normalize = true
	case "strip":
		o.strip = true
	case "flip":
		o.flip = true
	case "listFonts":
		o.listFonts = true
	case "infoFont":
		o.infoFont = true
	case "load":
		o.load = val
	case "color":
		o.color = val
	}
}

// parseArgs mirrors optparse with interspersed arguments.
func parseArgs(argv []string) (options, []string) {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80, color: ":"}
	var rest []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			rest = append(rest, argv[i+1:]...)
			return o, rest
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a, "=")
			s := findLong(name)
			if s.takesValue {
				if !hasVal {
					if i+1 >= len(argv) {
						optError(s.long + " option requires 1 argument")
					}
					i++
					val = argv[i]
				}
				apply(&o, s, s.long, val)
			} else {
				if hasVal {
					optError(s.long + " option does not take a value")
				}
				apply(&o, s, s.long, "")
			}
		case strings.HasPrefix(a, "-") && a != "-":
			chars := []rune(a[1:])
			for k := 0; k < len(chars); k++ {
				c := string(chars[k])
				s := findShort(c)
				if s == nil {
					optError("no such option: -" + c)
				}
				if s.takesValue {
					val := string(chars[k+1:])
					if val == "" {
						if i+1 >= len(argv) {
							optError("-" + c + " option requires 1 argument")
						}
						i++
						val = argv[i]
					}
					apply(&o, s, "-"+c, val)
					break
				}
				apply(&o, s, "-"+c, "")
			}
		default:
			rest = append(rest, a)
		}
	}
	return o, rest
}

func resolveDirection(o options, f *Font) string {
	if o.direction == "auto" {
		if f.printDirection == 1 {
			return "right-to-left"
		}
		return "left-to-right"
	}
	return o.direction
}

func resolveJustify(o options, direction string) string {
	if o.justify == "auto" {
		if direction == "right-to-left" {
			return "right"
		}
		return "left"
	}
	return o.justify
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) (code int) {
	o, args := parseArgs(argv)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	if o.listFonts {
		names := fontNames()
		sort.Strings(names)
		fmt.Fprintln(out, strings.Join(names, "\n"))
		return 0
	}
	if o.color == "list" {
		fmt.Fprintln(out, "[0-255];[0-255];[0-255] # RGB")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprint(out, usage)
		return 1
	}
	text := []rune(strings.Join(args, " "))

	font, err := LoadFont(o.font)
	if err == errFontNotFound {
		fmt.Fprintf(out, "pyfiglet error: requested font '%s' not found.\n", o.font)
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet.FontError:", err)
		return 1
	}

	defer func() {
		if r := recover(); r != nil {
			if c, ok := r.(pyCrash); ok {
				fmt.Fprintln(os.Stderr, "Traceback (most recent call last):\n ", c.msg)
				out.Reset(os.Stdout) // pyfiglet prints nothing before it crashes
				code = 1
				return
			}
			panic(r)
		}
	}()

	direction := resolveDirection(o, font)
	b := newBuilder(text, font, direction, o.width, resolveJustify(o, direction))
	r := []rune(b.render())
	if o.reverse {
		r = figReverse(r)
	}
	if o.flip {
		r = figFlip(r)
	}
	if o.strip {
		r = figStripSurroundingNewlines(r)
	} else if o.normalize {
		r = figNormalizeSurroundingNewlines(r)
	}
	out.WriteString(string(r))
	out.WriteString("\n")
	return 0
}
FIGLET_GO_MAIN_GO_EOF

# 3. The original is gone: delete every Python file.
find /workspace/repo -name '*.py' -type f -delete
find /workspace/repo -name '__pycache__' -type d -prune -exec rm -rf {} +

# 4. Sanity check.
go build ./...
go build -o /tmp/figlet .
/tmp/figlet -f slant "port done"
