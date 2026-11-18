package main

// Line-by-line port of pyfiglet's FigletBuilder (layout, wrapping, smushing,
// justification) and FigletString (reverse, flip, strip, normalize).
// Buffer rows are never modified in place: every change builds a new slice,
// matching Python's immutable strings (saved buffers share rows safely).

import "strings"

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

// renderPanic is raised where pyfiglet raises (CharNotPrinted, IndexError, ...).
type renderPanic string

type marker struct {
	buffer   [][]rune
	iterator int
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

func newBuilder(text []rune, font *Font, direction string, width int, justify string) *builder {
	b := &builder{text: text, direction: direction, width: width, font: font, justify: justify}
	b.buffer = make([][]rune, font.height)
	return b
}

func (b *builder) rtl() bool { return b.direction == "right-to-left" }

func (b *builder) copyBuffer() [][]rune {
	c := make([][]rune, len(b.buffer))
	copy(c, b.buffer)
	return c
}

func (b *builder) charAt(i int) ([][]rune, int, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, 0, false
	}
	c := int(b.text[i])
	rows, ok := b.font.chars[c]
	if !ok {
		return nil, 0, false
	}
	return rows, b.font.width[c], true
}

func (b *builder) addCharToProduct() {
	curChar, curWidth, ok := b.charAt(b.iterator)

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{b.copyBuffer(), b.iterator})
		b.handleNewLine()
		return
	}
	if !ok {
		return
	}
	if b.width < curWidth {
		panic(renderPanic("Width is not enough to print this character"))
	}
	b.curCharWidth = curWidth
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{b.copyBuffer(), b.iterator})
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
			panic(renderPanic("string index out of range"))
		}
		right := addRight[i]
		smushed, ok := b.smushChars(left, right)
		// updateSmushedCharInLeftBuffer
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if !ok {
			panic(renderPanic("sequence item: expected str instance, NoneType found"))
		}
		nl := make([]rune, 0, len(addLeft))
		nl = append(nl, addLeft[:idx]...)
		if smushed != none { // Python: l[idx] = '' drops the character
			nl = append(nl, smushed)
		}
		addLeft = append(nl, addLeft[idx+1:]...)
	}
	return addLeft, addRight
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	var tail []rune
	if b.maxSmush < len(addRight) {
		tail = addRight[b.maxSmush:]
	}
	b.buffer[row] = concat(addLeft, tail)
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([][]rune, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	// pyfiglet also recomputes maxSmush here; the value is always recomputed
	// before use and the computation has no side effects, so it is skipped.
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.product = append(b.product, m.buffer)
		b.iterator = m.iterator
	} else {
		b.product = append(b.product, b.buffer)
		b.iterator--
	}
	b.cutBufferCommon()
}

func (b *builder) smushAmount(buffer [][]rune, curChar [][]rune) int {
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
		charbd := len(lineRight) - len(lstripSpace(lineRight))
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

func (b *builder) isHardBlank(r rune) bool {
	return r != none && len(b.font.hardBlank) == 1 && b.font.hardBlank[0] == r
}

// in mirrors Python's `ch in s` for a 0- or 1-character string ch.
func in(ch rune, s string) bool {
	return ch == none || strings.ContainsRune(s, ch)
}

var smushTable = []struct{ a, b string }{
	{"|", `/\[]{}()<>`},
	{`\/`, "[]{}()<>"},
	{"[]", "{}()<>"},
	{"{}", "()<>"},
	{"()", "<>"},
}

// smushChars returns the smushed character, or ok=false for Python's None.
func (b *builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return none, false
	}
	mode := b.font.smushMode
	if mode&smSmush == 0 {
		return none, false
	}
	if mode&63 == 0 {
		if b.isHardBlank(left) {
			return right, true
		}
		if b.isHardBlank(right) {
			return left, true
		}
		if b.rtl() {
			return left, true
		}
		return right, true
	}
	if mode&smHardblank != 0 && b.isHardBlank(left) && b.isHardBlank(right) {
		return left, true
	}
	if b.isHardBlank(left) || b.isHardBlank(right) {
		return none, false
	}
	if mode&smEqual != 0 && left == right {
		return left, true
	}
	var rules []struct{ a, b string }
	if mode&smLowline != 0 {
		rules = append(rules, struct{ a, b string }{"_", `|/\[]{}()<>`})
	}
	if mode&smHierarchy != 0 {
		rules = append(rules, smushTable...)
	}
	for _, r := range rules {
		if in(left, r.a) && in(right, r.b) {
			return right, true
		}
		if in(right, r.a) && in(left, r.b) {
			return left, true
		}
	}
	if mode&smPair != 0 && left != none && right != none {
		for _, p := range []string{string([]rune{left, right}), string([]rune{right, left})} {
			if p == "[]" || p == "{}" || p == "()" {
				return '|', true
			}
		}
	}
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
	return none, false
}

func (b *builder) justifyRows(buffer [][]rune) [][]rune {
	out := make([][]rune, len(buffer))
	copy(out, buffer)
	for row := 0; row < b.font.height && row < len(out); row++ {
		pad := 0
		switch b.justify {
		case "right":
			pad = b.width - len(out[row]) - 1
		case "center":
			pad = (b.width - len(out[row])) / 2 // Go and Python int() both truncate toward 0
		}
		if pad > 0 {
			out[row] = concat([]rune(strings.Repeat(" ", pad)), out[row])
		}
	}
	return out
}

// render runs the builder and returns pyfiglet's renderText() result.
func render(text []rune, font *Font, direction string, width int, justify string) []rune {
	b := newBuilder(text, font, direction, width, justify)
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	if len(b.buffer[0]) != 0 {
		b.product = append(b.product, b.buffer)
	}
	var sb strings.Builder
	hb := string(font.hardBlank)
	for _, buf := range b.product {
		s := string(joinLines(b.justifyRows(buf))) + "\n"
		sb.WriteString(strings.ReplaceAll(s, hb, " "))
	}
	return []rune(sb.String())
}

// ---------------- FigletString ----------------

var reverseMap = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<',
	'[': ']', ']': '[', '{': '}', '}': '{',
}

var flipMap = map[rune]rune{
	'/': '\\', '\\': '/', 'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M',
	'P': 'b', 'R': 'b', 'b': 'P', '^': 'v', 'v': '^', '_': '-',
	'm': 'w', 'w': 'm',
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
	return append(joinLines(rows), '\n')
}

func reverseText(s []rune) []rune {
	var out [][]rune
	for _, row := range splitLines(s) {
		t := translate(row, reverseMap)
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
		out = append(out, translate(rows[i], flipMap))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s []rune) []rune {
	var out [][]rune
	seen := false
	for _, row := range splitLines(s) {
		if len(stripWS(row)) != 0 || seen {
			seen = true
			out = append(out, row)
		}
	}
	return rstripWS(newFromList(out))
}

func normalizeSurroundingNewlines(s []rune) []rune {
	return append(append([]rune{'\n'}, stripSurroundingNewlines(s)...), '\n')
}
