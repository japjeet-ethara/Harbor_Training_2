package main

// Port of pyfiglet.FigletBuilder: placing, smushing, wrapping and justifying
// characters. Kept deliberately close to the Python, quirks included.

import "errors"

var errCharNotPrinted = errors.New("Width is not enough to print this character")

// crash mirrors an uncaught Python exception (IndexError, TypeError) inside the builder.
type crash struct{ msg string }

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

type marker struct {
	buffer   [][]rune
	iterator int
}

type builder struct {
	text      []rune
	rtl       bool
	width     int
	font      *Font
	justify   string
	hardBlank rune // noChar when the font's hardblank is not a single character

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []marker
	queue        [][][]rune
	buffer       [][]rune
}

func render(text string, font *Font, direction, justify string, width int) (string, error) {
	b := &builder{
		text: []rune(text), rtl: direction == "right-to-left", width: width,
		font: font, justify: justify, hardBlank: noChar,
	}
	if len(font.hardBlank) == 1 {
		b.hardBlank = font.hardBlank[0]
	}
	b.buffer = emptyBuffer(font.height)
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
		b.iterator++
	}
	return b.returnProduct(), nil
}

func emptyBuffer(h int) [][]rune {
	if h < 0 {
		h = 0
	}
	return make([][]rune, h)
}

func copyBuffer(buf [][]rune) [][]rune {
	out := make([][]rune, len(buf))
	copy(out, buf) // rows are never modified in place, so sharing them is safe
	return out
}

func (b *builder) addCharToProduct() error {
	curChar := b.getCharAt(b.iterator)

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}
	if curChar == nil {
		return nil
	}
	w := b.getCharWidthAt(b.iterator)
	if b.width < w {
		return errCharNotPrinted
	}
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = len(b.row(0)) + b.curCharWidth - b.maxSmush

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
	return nil
}

// row returns buffer[i]; Python would raise IndexError for a font of height 0.
func (b *builder) row(i int) []rune {
	if i >= len(b.buffer) {
		panic(crash{"IndexError: list index out of range"})
	}
	return b.buffer[i]
}

func (b *builder) returnProduct() string {
	if len(b.row(0)) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	out := make([]rune, 0, 256)
	for _, buf := range b.queue {
		buf = b.justifyString(buf)
		out = append(out, b.replaceHardblanks(buf)...)
	}
	return string(out)
}

func (b *builder) getCharAt(i int) [][]rune {
	if i < 0 || i >= len(b.text) {
		return nil
	}
	return b.font.chars[int64(b.text[i])]
}

func (b *builder) getCharWidthAt(i int) int {
	return b.font.width[int64(b.text[i])]
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.rtl {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := noChar
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		if i >= len(addRight) {
			panic(crash{"IndexError: string index out of range"})
		}
		right := addRight[i]
		smushed, ok := b.smushChars(left, right)
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if !ok || smushed == noChar {
			panic(crash{"TypeError: sequence item: expected str instance, NoneType found"})
		}
		l := make([]rune, len(addLeft))
		copy(l, addLeft)
		l[idx] = smushed
		addLeft = l
	}
	return addLeft, addRight
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	tail := []rune{}
	if b.maxSmush < len(addRight) {
		tail = addRight[b.maxSmush:]
	}
	joined := make([]rune, 0, len(addLeft)+len(tail))
	joined = append(joined, addLeft...)
	b.buffer[row] = append(joined, tail...)
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

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		if m.iterator != -1 {
			b.queue = append(b.queue, m.buffer)
			b.iterator = m.iterator
			b.cutBufferCommon()
			return
		}
	}
	b.queue = append(b.queue, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) justifyString(buf [][]rune) [][]rune {
	out := copyBuffer(buf)
	for row := 0; row < b.font.height && row < len(out); row++ {
		pad := 0
		switch b.justify {
		case "right":
			pad = b.width - len(out[row]) - 1
		case "center":
			pad = (b.width - len(out[row])) / 2 // Go and Python int() both truncate toward zero
		}
		if pad > 0 {
			r := make([]rune, 0, pad+len(out[row]))
			for k := 0; k < pad; k++ {
				r = append(r, ' ')
			}
			out[row] = append(r, out[row]...)
		}
	}
	return out
}

func (b *builder) replaceHardblanks(buf [][]rune) []rune {
	var s []rune
	for i, r := range buf {
		if i > 0 {
			s = append(s, '\n')
		}
		s = append(s, r...)
	}
	s = append(s, '\n')
	return replaceAllRunes(s, b.font.hardBlank, []rune{' '})
}

func replaceAllRunes(s, old, repl []rune) []rune {
	if len(old) == 0 {
		return s
	}
	var out []rune
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && equalRunes(s[i:i+len(old)], old) {
			out = append(out, repl...)
			i += len(old)
		} else {
			out = append(out, s[i])
			i++
		}
	}
	return out
}

func equalRunes(a, b []rune) bool {
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

func (b *builder) smushAmount(buffer [][]rune, curChar [][]rune) int {
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

		// Only ASCII space is stripped, to match figlet.
		trimmed := len(lineLeft)
		for trimmed > 0 && lineLeft[trimmed-1] == ' ' {
			trimmed--
		}
		linebd := trimmed - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := noChar
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}

		charbd := 0
		for charbd < len(lineRight) && lineRight[charbd] == ' ' {
			charbd++
		}
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

// in is Python's `ch in s` for a one-character string ch ('' is in every string).
func in(ch rune, s string) bool {
	if ch == noChar {
		return true
	}
	for _, r := range s {
		if r == ch {
			return true
		}
	}
	return false
}

// smushChars returns (result, true), or (_, false) for Python's None.
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
	mode := b.font.smushMode
	if mode&smSmush == 0 {
		return 0, false
	}
	hb := b.hardBlank
	isHB := func(c rune) bool { return hb != noChar && c == hb }

	if mode&63 == 0 { // universal overlapping
		if isHB(left) {
			return right, true
		}
		if isHB(right) {
			return left, true
		}
		if b.rtl {
			return left, true
		}
		return right, true
	}
	if mode&smHardblank != 0 && isHB(left) && isHB(right) {
		return left, true
	}
	if isHB(left) || isHB(right) {
		return 0, false
	}
	if mode&smEqual != 0 && left == right {
		return left, true
	}

	type rule struct{ a, b string }
	var smushes []rule
	if mode&smLowline != 0 {
		smushes = append(smushes, rule{"_", `|/\[]{}()<>`})
	}
	if mode&smHierarchy != 0 {
		smushes = append(smushes,
			rule{"|", `/\[]{}()<>`},
			rule{`\/`, "[]{}()<>"},
			rule{"[]", "{}()<>"},
			rule{"{}", "()<>"},
			rule{"()", "<>"},
		)
	}
	for _, s := range smushes {
		if in(left, s.a) && in(right, s.b) {
			return right, true
		}
		if in(right, s.a) && in(left, s.b) {
			return left, true
		}
	}

	if mode&smPair != 0 && left != noChar && right != noChar {
		for _, pair := range [][2]rune{{left, right}, {right, left}} {
			switch string([]rune{pair[0], pair[1]}) {
			case "[]", "{}", "()":
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
	return 0, false
}
