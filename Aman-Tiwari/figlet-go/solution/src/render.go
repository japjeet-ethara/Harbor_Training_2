package main

import (
	"errors"
	"strings"
)

// Errors that correspond to Python exceptions escaping pyfiglet (empty stdout).
var (
	errCharNotPrinted = errors.New("Width is not enough to print this character")
	errPyCrash        = errors.New("pyfiglet would raise here (IndexError/TypeError)")
	errRunaway        = errors.New("pyfiglet would never finish")
)

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

type blankMarker struct {
	buffer   [][]rune
	iterator int
}

// builder is a port of pyfiglet.FigletBuilder.
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

	blankMarkers []blankMarker
	queue        [][][]rune
	buffer       [][]rune
}

// noChar marks Python's empty string '' where a single character is expected.
const noChar rune = -1

func newBuffer(h int) [][]rune {
	b := make([][]rune, h)
	for i := range b {
		b[i] = []rune{}
	}
	return b
}

func copyBuffer(b [][]rune) [][]rune {
	c := make([][]rune, len(b))
	copy(c, b)
	return c
}

func render(text string, font *Font, direction, justify string, width int) (string, error) {
	b := &builder{
		text:      []rune(text),
		direction: direction,
		width:     width,
		font:      font,
		justify:   justify,
		buffer:    newBuffer(font.height),
	}
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
		b.iterator++
		if len(b.queue) > 1_000_000 {
			return "", errRunaway
		}
	}
	return b.returnProduct(), nil
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

func (b *builder) addCharToProduct() error {
	curChar, curWidth, ok := b.charAt(b.iterator)

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{copyBuffer(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}
	if !ok {
		return nil
	}
	if b.width < curWidth {
		return errCharNotPrinted
	}
	b.curCharWidth = curWidth
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{copyBuffer(b.buffer), b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			if err := b.addCurCharRowToBufferRow(curChar, row); err != nil {
				return err
			}
		}
	}
	// Note: pyfiglet sets this even right after a line break.
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) returnProduct() string {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		buf = b.justifyString(buf)
		sb.WriteString(b.replaceHardblanks(buf))
	}
	return sb.String()
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune, error) {
	addLeft := append([]rune{}, b.buffer[row]...)
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft = append([]rune{}, curChar[row]...)
		addRight = b.buffer[row]
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := noChar
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		if i >= len(addRight) {
			return nil, nil, errPyCrash // IndexError: string index out of range
		}
		right := addRight[i]
		smushed, ok := b.smushChars(left, right)
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if !ok || smushed == noChar {
			return nil, nil, errPyCrash // TypeError joining None
		}
		addLeft[idx] = smushed
	}
	return addLeft, addRight, nil
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) error {
	addLeft, addRight, err := b.smushRow(curChar, row)
	if err != nil {
		return err
	}
	out := addLeft
	if b.maxSmush < len(addRight) {
		out = append(out, addRight[b.maxSmush:]...)
	}
	b.buffer[row] = out
	return nil
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = newBuffer(b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	// pyfiglet also recomputes maxSmush here; it is always recomputed before use.
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.queue = append(b.queue, m.buffer)
		b.iterator = m.iterator
		b.cutBufferCommon()
		return
	}
	b.queue = append(b.queue, b.buffer)
	b.iterator--
	b.cutBufferCommon()
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
	out := copyBuffer(buf)
	switch b.justify {
	case "right":
		for row := 0; row < b.font.height; row++ {
			out[row] = append(spaces(b.width-len(out[row])-1), out[row]...)
		}
	case "center":
		for row := 0; row < b.font.height; row++ {
			// int((width - len) / 2) truncates toward zero, like Go's integer division.
			out[row] = append(spaces((b.width-len(out[row]))/2), out[row]...)
		}
	}
	return out
}

func (b *builder) replaceHardblanks(buf [][]rune) string {
	s := string(joinLines(buf)) + "\n"
	return strings.ReplaceAll(s, b.font.hardBlank, " ")
}

func (b *builder) smushAmount(buffer [][]rune, curChar [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft, lineRight := buffer[row], curChar[row]
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

// pyIn mirrors Python's `c in s` for a one-character (or empty) string c.
func pyIn(c rune, s string) bool {
	if c == noChar {
		return true // '' in s is always True
	}
	return strings.ContainsRune(s, c)
}

// smushChars is a port of FigletBuilder.smushChars. ok=false means None.
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
	if mode&smHardblank != 0 && b.isHardBlank(left) && b.isHardBlank(right) {
		return left, true
	}
	if b.isHardBlank(left) || b.isHardBlank(right) {
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
		if pyIn(left, s.a) && pyIn(right, s.b) {
			return right, true
		}
		if pyIn(right, s.a) && pyIn(left, s.b) {
			return left, true
		}
	}

	if mode&smPair != 0 && left != noChar {
		lr, rl := string([]rune{left, right}), string([]rune{right, left})
		for _, p := range []string{lr, rl} {
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
	return 0, false
}
