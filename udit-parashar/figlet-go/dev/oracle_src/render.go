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
