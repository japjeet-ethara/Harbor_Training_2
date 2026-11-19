package main

// Port of pyfiglet's FigletBuilder / FigletRenderingEngine / FigletString.

import (
	"errors"
	"strings"
)

const (
	smEqual     = 1   // smush equal chars (not hardblanks)
	smLowline   = 2   // smush _ with any char in hierarchy
	smHierarchy = 4   // hierarchy: |, /\, [], {}, (), <>
	smPair      = 8   // [ + ] -> |, { + } -> |, ( + ) -> |
	smBigX      = 16  // / + \ -> |, \ + / -> Y, > + < -> X
	smHardblank = 32  // hardblank + hardblank -> hardblank
	smKern      = 64  // kerning
	smSmush     = 128 // smushing
)

const (
	leftToRight = "left-to-right"
	rightToLeft = "right-to-left"
)

// noChar stands for Python's empty string ("") where a character is expected.
const noChar rune = -1

var (
	errCharNotPrinted = errors.New("Width is not enough to print this character")
	errEndlessLoop    = errors.New("character can never fit on a line (pyfiglet would loop forever)")
	errNoneJoin       = errors.New("sequence item: expected str instance, NoneType found")
)

type figlet struct {
	font      *figletFont
	direction string // already resolved: left-to-right or right-to-left
	justify   string // already resolved: left, center or right
	width     int
}

func newFiglet(fontName, direction, justify string, width int) (*figlet, error) {
	font, err := newFigletFont(fontName)
	if err != nil {
		return nil, err
	}
	f := &figlet{font: font, width: width}
	f.direction = direction
	if direction == "auto" {
		f.direction = leftToRight
		if font.printDirection != nil && *font.printDirection == 1 {
			f.direction = rightToLeft
		}
	}
	f.justify = justify
	if justify == "auto" {
		if f.direction == leftToRight {
			f.justify = "left"
		} else {
			f.justify = "right"
		}
	}
	return f, nil
}

type blankMarker struct {
	buffer   [][]rune
	iterator int
}

type builder struct {
	text      []rune
	direction string
	width     int
	font      *figletFont
	justify   string
	hardBlank rune // noChar-like sentinel when the hardblank is not one character

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []blankMarker
	queue        [][][]rune
	buffer       [][]rune
}

// renderText mirrors Figlet.renderText.
func (f *figlet) renderText(text string) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	b := &builder{
		text:      []rune(text),
		direction: f.direction,
		width:     f.width,
		font:      f.font,
		justify:   f.justify,
		hardBlank: -2,
	}
	if hb := []rune(f.font.hardBlank); len(hb) == 1 {
		b.hardBlank = hb[0]
	}
	b.buffer = emptyBuffer(f.font.height)
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	return b.returnProduct(), nil
}

func emptyBuffer(height int) [][]rune {
	if height < 0 {
		height = 0
	}
	return make([][]rune, height)
}

func copyBuffer(buf [][]rune) [][]rune {
	return append([][]rune(nil), buf...)
}

func (b *builder) charAt(i int) ([][]rune, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	c, ok := b.font.chars[b.text[i]]
	return c, ok
}

func (b *builder) charWidthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	w, ok := b.font.width[b.text[i]]
	return w, ok
}

func (b *builder) addCharToProduct() {
	curChar, ok := b.charAt(b.iterator)

	// a newline flushes the buffer
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{copyBuffer(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if !ok {
		return
	}
	w, _ := b.charWidthAt(b.iterator)
	if b.width < w {
		panic(errCharNotPrinted)
	}
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, curChar)

	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{copyBuffer(b.buffer), b.iterator})
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

func (b *builder) returnProduct() string {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		buf = b.justifyBuffer(buf)
		sb.WriteString(b.replaceHardblanks(buf))
	}
	return sb.String()
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.direction == rightToLeft {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := noChar
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		right := addRight[i] // IndexError in Python if out of range
		smushed, ok := b.smushChars(left, right)
		// updateSmushedCharInLeftBuffer
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if idx == len(addLeft) {
			panic(errors.New("list assignment index out of range"))
		}
		if !ok {
			panic(errNoneJoin) // l[idx] = None; ''.join(l) -> TypeError
		}
		l := append([]rune(nil), addLeft...)
		if smushed == noChar { // l[idx] = '' removes the character
			l = append(l[:idx], l[idx+1:]...)
		} else {
			l[idx] = smushed
		}
		addLeft = l
	}
	return addLeft, addRight
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	tail := addRight[min(b.maxSmush, len(addRight)):]
	nr := make([]rune, 0, len(addLeft)+len(tail))
	nr = append(nr, addLeft...)
	nr = append(nr, tail...)
	b.buffer[row] = nr
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = emptyBuffer(b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, ok := b.charAt(b.iterator)
	if !ok {
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
	// With an entirely empty line the same character would be retried on the
	// same empty line forever; pyfiglet never finishes in that case.
	empty := true
	for _, r := range b.buffer {
		if len(r) != 0 {
			empty = false
			break
		}
	}
	if empty {
		panic(errEndlessLoop)
	}
	b.queue = append(b.queue, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.cutBufferAtLastBlank(m.buffer, m.iterator)
	} else {
		b.cutBufferAtLastChar()
	}
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

func (b *builder) justifyBuffer(buf [][]rune) [][]rune {
	out := copyBuffer(buf)
	for row := 0; row < b.font.height; row++ {
		var pad int
		switch b.justify {
		case "right":
			pad = b.width - len(out[row]) - 1
		case "center":
			pad = (b.width - len(out[row])) / 2 // truncates toward zero like int()
		default:
			continue
		}
		out[row] = append(spaces(pad), out[row]...)
	}
	return out
}

func (b *builder) replaceHardblanks(buf [][]rune) string {
	rows := make([]string, len(buf))
	for i, r := range buf {
		rows[i] = string(r)
	}
	s := strings.Join(rows, "\n") + "\n"
	return strings.ReplaceAll(s, b.font.hardBlank, " ")
}

// rstripSpaceLen is len(s.rstrip(' ')) (ASCII space only).
func rstripSpaceLen(r []rune) int {
	n := len(r)
	for n > 0 && r[n-1] == ' ' {
		n--
	}
	return n
}

func lstripSpaceCount(r []rune) int {
	n := 0
	for n < len(r) && r[n] == ' ' {
		n++
	}
	return n
}

// smushAmount mirrors FigletBuilder.smushAmount.
func (b *builder) smushAmount(buffer [][]rune, curChar [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.direction == rightToLeft {
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

// in mirrors Python's `ch in s` for a one-character (or empty) string.
func in(ch rune, s string) bool {
	return ch == noChar || strings.ContainsRune(s, ch)
}

var hierarchy = [][2]string{
	{"|", `/\[]{}()<>`},
	{`\/`, "[]{}()<>"},
	{"[]", "{}()<>"},
	{"{}", "()<>"},
	{"()", "<>"},
}

// smushChars mirrors FigletBuilder.smushChars. ok=false means Python's None.
func (b *builder) smushChars(left, right rune) (rune, bool) {
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
	hb := b.hardBlank
	// smushing by universal overlapping
	if mode&63 == 0 {
		if left == hb {
			return right, true
		}
		if right == hb {
			return left, true
		}
		if b.direction == rightToLeft {
			return left, true
		}
		return right, true
	}
	if mode&smHardblank != 0 && left == hb && right == hb {
		return left, true
	}
	if left == hb || right == hb {
		return 0, false
	}
	if mode&smEqual != 0 && left == right {
		return left, true
	}
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
	if mode&smPair != 0 && left != noChar {
		for _, pair := range [][2]rune{{left, right}, {right, left}} {
			s := string(pair[:])
			if s == "[]" || s == "{}" || s == "()" {
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

// ---- FigletString ----------------------------------------------------------

var reverseMap = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<',
	'[': ']', ']': '[', '{': '}', '}': '{',
}

var flipMap = map[rune]rune{
	'/': '\\', '\\': '/', 'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M',
	'P': 'b', 'R': 'b', 'b': 'P', '^': 'v', 'v': '^', '_': '-',
	'm': 'w', 'w': 'm',
}

func translate(s string, m map[rune]rune) []rune {
	r := []rune(s)
	for i, c := range r {
		if t, ok := m[c]; ok {
			r[i] = t
		}
	}
	return r
}

func newFromList(rows []string) string {
	return strings.Join(rows, "\n") + "\n"
}

func figletReverse(s string) string {
	var out []string
	for _, row := range pySplitlines(s) {
		r := translate(row, reverseMap)
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		out = append(out, string(r))
	}
	return newFromList(out)
}

func figletFlip(s string) string {
	lines := pySplitlines(s)
	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		out = append(out, string(translate(lines[i], flipMap)))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s string) string {
	var out []string
	seen := false
	for _, row := range pySplitlines(s) {
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
