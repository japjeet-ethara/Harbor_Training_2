package main

// The layout engine: a line-by-line port of pyfiglet's FigletBuilder.
//
// Algorithm overview
//
// The output is built one "line" (a block of Font.Height rows) at a time in
// `buffer`. For every input character:
//
//  1. smushAmount() works out how many columns the glyph can slide left into
//     the buffer. Per row it measures the gap between the buffer's last
//     non-space character and the glyph's first non-space character, adds one
//     column if the two edge characters may be smushed, and takes the
//     minimum over all rows. With neither kerning nor smushing enabled the
//     amount is 0 (full width).
//  2. If the line would then reach the width limit, the line is broken:
//     at the last space seen on this line (the buffer as it was before that
//     space is emitted and input resumes after the space), or, with no
//     space, just before the current character, which is then retried on a
//     fresh line.
//  3. Otherwise each row is appended, overlapping `maxSmush` columns; the
//     overlapping cells are resolved by smushChars() (the six FIGfont rules,
//     or universal smushing).
//
// In right-to-left mode the roles of buffer and glyph are swapped, so each
// glyph is placed to the left of the buffer. Finished lines are justified
// and their hardblanks turned into spaces.
//
// The many small quirks of the original (for example, smushing at the start
// of a line trims a glyph's leading blanks, and a newline in the text flushes
// even an empty line) are kept on purpose: output must match byte for byte.

import (
	"errors"
	"strings"
)

// errCharNotPrinted corresponds to pyfiglet.CharNotPrinted.
var errCharNotPrinted = errors.New("Width is not enough to print this character")

// errNoProgress is raised where the original would loop forever: a glyph
// exactly as wide as the line limit can never be placed, and the original
// keeps retrying it on an empty line.
var errNoProgress = errors.New("width is not enough to print this character (the original pyfiglet never terminates here)")

// pyCrash marks a point where the original raises an uncaught Python
// exception (IndexError or TypeError) during rendering.
type pyCrash struct{ msg string }

func (c pyCrash) Error() string { return c.msg }

type marker struct {
	buffer   [][]rune
	iterator int
}

type builder struct {
	text    []rune
	font    *Font
	rtl     bool
	width   int
	justify string
	hb      rune // single-rune hardblank, or -1

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []marker
	product      [][][]rune
	buffer       [][]rune
}

// Render lays out text and returns the figure, rows separated by "\n".
func Render(font *Font, text string, direction, justify string, width int) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			if c, ok := r.(pyCrash); ok {
				out, err = "", c
				return
			}
			panic(r)
		}
	}()
	b := &builder{
		text:    []rune(text),
		font:    font,
		rtl:     direction == "right-to-left",
		width:   width,
		justify: justify,
		hb:      font.hardBlankRune(),
		buffer:  emptyBuffer(font.Height),
	}
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
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
	return append([][]rune(nil), buf...) // rows are never modified in place
}

func (b *builder) row0() []rune {
	if len(b.buffer) == 0 {
		panic(pyCrash{"IndexError: list index out of range"})
	}
	return b.buffer[0]
}

func (b *builder) charAt(i int) ([][]rune, int, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, 0, false
	}
	c := int64(b.text[i])
	rows, ok := b.font.Chars[c]
	return rows, b.font.Width[c], ok
}

func (b *builder) addCharToProduct() error {
	cur, curWidth, ok := b.charAt(b.iterator)

	// A newline flushes the current line.
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
		return b.handleNewLine()
	}
	if !ok { // characters missing from the font are skipped
		return nil
	}
	if b.width < curWidth {
		return errCharNotPrinted
	}
	b.curCharWidth = curWidth
	b.maxSmush = b.smushAmount(b.buffer, cur)
	b.currentTotalWidth = len(b.row0()) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		if err := b.handleNewLine(); err != nil {
			return err
		}
	} else {
		for row := 0; row < b.font.Height; row++ {
			b.addCurCharRowToBufferRow(cur, row)
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) handleNewLine() error {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		// Cut at the last blank: emit the line as it was before it and
		// resume after it.
		b.product = append(b.product, m.buffer)
		b.iterator = m.iterator
		b.cutBufferCommon()
		return nil
	}
	// No blank on this line: emit it and retry the current character.
	if allEmpty(b.buffer) {
		return errNoProgress
	}
	b.product = append(b.product, b.buffer)
	b.iterator--
	b.cutBufferCommon()
	return nil
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = emptyBuffer(b.font.Height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	// The original also recomputes maxSmush here; the value is always
	// recomputed before it is next used, so that step is omitted.
}

// smushAmount is FigletBuilder.smushAmount.
func (b *builder) smushAmount(buffer, cur [][]rune) int {
	if b.font.SmushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.Height; row++ {
		lineLeft, lineRight := buffer[row], cur[row]
		if b.rtl {
			lineLeft, lineRight = lineRight, lineLeft
		}

		// Only ASCII spaces are stripped, to match figlet exactly.
		linebd := len(trimRightSpace(lineLeft)) - 1
		if linebd < 0 {
			linebd = 0
		}
		var ch1 rune = -1 // -1 stands for Python's ''
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}

		charbd := len(lineRight) - len(trimLeftSpace(lineRight))
		var ch2 rune = -1
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
		}

		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == -1 || ch1 == ' ' {
			amt++
		} else if ch2 != -1 {
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

func trimRightSpace(s []rune) []rune {
	j := len(s)
	for j > 0 && s[j-1] == ' ' {
		j--
	}
	return s[:j]
}

func trimLeftSpace(s []rune) []rune {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return s[i:]
}

// addCurCharRowToBufferRow is smushRow plus the final concatenation.
func (b *builder) addCurCharRowToBufferRow(cur [][]rune, row int) {
	addLeft, addRight := b.buffer[row], cur[row]
	if b.rtl {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		if i >= len(addRight) {
			panic(pyCrash{"IndexError: string index out of range"})
		}
		if idx < 0 || idx >= len(addLeft) {
			continue // the original computes a value here and discards it
		}
		smushed, ok := b.smushChars(addLeft[idx], addRight[i])
		if !ok {
			panic(pyCrash{"TypeError: sequence item: expected str instance, NoneType found"})
		}
		next := append([]rune(nil), addLeft...)
		next[idx] = smushed
		addLeft = next
	}
	tail := addRight[min(b.maxSmush, len(addRight)):]
	joined := make([]rune, 0, len(addLeft)+len(tail))
	b.buffer[row] = append(append(joined, addLeft...), tail...)
}

// smushChars decides which character to show where two glyph edges
// overlap. ok is false when the pair cannot be smushed (Python's None).
func (b *builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	// No overlapping if the previous or current character is narrower
	// than 2 columns.
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}
	mode := b.font.SmushMode
	if mode&smSmush == 0 { // kerning only
		return 0, false
	}

	// Universal smushing: no rules selected.
	if mode&63 == 0 {
		if left == b.hb {
			return right, true
		}
		if right == b.hb {
			return left, true
		}
		// The later character in the text wins.
		if b.rtl {
			return left, true
		}
		return right, true
	}

	if mode&smHardblank != 0 && left == b.hb && right == b.hb { // rule 6
		return left, true
	}
	if left == b.hb || right == b.hb {
		return 0, false
	}
	if mode&smEqual != 0 && left == right { // rule 1
		return left, true
	}

	type class struct{ a, b string }
	var smushes []class
	if mode&smLowline != 0 { // rule 2
		smushes = append(smushes, class{"_", `|/\[]{}()<>`})
	}
	if mode&smHierarchy != 0 { // rule 3
		smushes = append(smushes,
			class{"|", `/\[]{}()<>`},
			class{`\/`, "[]{}()<>"},
			class{"[]", "{}()<>"},
			class{"{}", "()<>"},
			class{"()", "<>"},
		)
	}
	for _, c := range smushes {
		if strings.ContainsRune(c.a, left) && strings.ContainsRune(c.b, right) {
			return right, true
		}
		if strings.ContainsRune(c.a, right) && strings.ContainsRune(c.b, left) {
			return left, true
		}
	}

	if mode&smPair != 0 { // rule 4
		for _, p := range []string{string([]rune{left, right}), string([]rune{right, left})} {
			if p == "[]" || p == "{}" || p == "()" {
				return '|', true
			}
		}
	}
	if mode&smBigX != 0 { // rule 5
		switch {
		case left == '/' && right == '\\':
			return '|', true
		case right == '/' && left == '\\':
			return 'Y', true
		case left == '>' && right == '<':
			return 'X', true
		}
	}
	return 0, false
}

// returnProduct flushes the last line, justifies every line and replaces
// hardblanks with spaces.
func (b *builder) returnProduct() string {
	if len(b.row0()) != 0 {
		b.product = append(b.product, b.buffer)
	}
	var out strings.Builder
	for _, buf := range b.product {
		buf = b.justifyRows(buf)
		rows := make([]string, len(buf))
		for i, r := range buf {
			rows[i] = string(r)
		}
		s := strings.Join(rows, "\n") + "\n"
		out.WriteString(strings.ReplaceAll(s, string(b.font.HardBlank), " "))
	}
	return out.String()
}

// justifyRows is justifyString; each row is padded on its own.
func (b *builder) justifyRows(buf [][]rune) [][]rune {
	out := copyBuffer(buf)
	for row := 0; row < b.font.Height && row < len(out); row++ {
		pad := 0
		switch b.justify {
		case "right":
			pad = b.width - len(out[row]) - 1
		case "center":
			pad = (b.width - len(out[row])) / 2 // int() truncates toward zero
		}
		if pad > 0 {
			out[row] = append([]rune(strings.Repeat(" ", pad)), out[row]...)
		}
	}
	return out
}
