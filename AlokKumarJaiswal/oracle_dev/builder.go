package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	SM_EQUAL     = 1
	SM_LOWLINE   = 2
	SM_HIERARCHY = 4
	SM_PAIR      = 8
	SM_BIGX      = 16
	SM_HARDBLANK = 32
	SM_KERN      = 64
	SM_SMUSH     = 128
)

type blankMarker struct {
	buffer   []string
	iterator int
}

// FigletBuilder represents the internal FIGlet build/rendering process.
// Replaces Python class FigletBuilder.
type FigletBuilder struct {
	text              []rune
	direction         string
	width             int
	font              *FigletFont
	justify           string
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []blankMarker
	product           [][]string
	buffer            []string
}

// NewFigletBuilder creates a new FigletBuilder.
// Replaces FigletBuilder.__init__.
func NewFigletBuilder(text string, font *FigletFont, direction string, width int, justify string) *FigletBuilder {
	return &FigletBuilder{
		text:         []rune(text),
		direction:    direction,
		width:        width,
		font:         font,
		justify:      justify,
		buffer:       make([]string, font.Height),
		blankMarkers: make([]blankMarker, 0),
		product:      make([][]string, 0),
	}
}

// IsNotFinished returns true if there are more characters to process.
// Replaces FigletBuilder.isNotFinished.
func (b *FigletBuilder) IsNotFinished() bool {
	return b.iterator < len(b.text)
}

// GoToNextChar advances iterator to next character.
// Replaces FigletBuilder.goToNextChar.
func (b *FigletBuilder) GoToNextChar() {
	b.iterator++
}

// getCharAt gets the glyph rows for character at index i.
// Replaces FigletBuilder.getCharAt.
func (b *FigletBuilder) getCharAt(i int) []string {
	if i < 0 || i >= len(b.text) {
		return nil
	}
	c := b.text[i]
	return b.font.Chars[c]
}

// getCharWidthAt gets the width for character at index i.
// Replaces FigletBuilder.getCharWidthAt.
func (b *FigletBuilder) getCharWidthAt(i int) int {
	if i < 0 || i >= len(b.text) {
		return 0
	}
	c := b.text[i]
	return b.font.Width[c]
}

// getCurChar returns the current character glyph rows.
// Replaces FigletBuilder.getCurChar.
func (b *FigletBuilder) getCurChar() []string {
	return b.getCharAt(b.iterator)
}

// getCurWidth returns the current character width.
// Replaces FigletBuilder.getCurWidth.
func (b *FigletBuilder) getCurWidth() int {
	return b.getCharWidthAt(b.iterator)
}

// AddCharToProduct processes current character and updates buffer.
// Replaces FigletBuilder.addCharToProduct.
func (b *FigletBuilder) AddCharToProduct() error {
	curChar := b.getCurChar()

	// If character is newline, flush buffer
	if b.text[b.iterator] == '\n' {
		bufCopy := make([]string, len(b.buffer))
		copy(bufCopy, b.buffer)
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: bufCopy, iterator: b.iterator})
		b.handleNewLine()
		return nil
	}

	if curChar == nil {
		return nil
	}

	curWidth := b.getCurWidth()
	if b.width < curWidth {
		return fmt.Errorf("CharNotPrinted: Width is not enough to print this character")
	}
	b.curCharWidth = curWidth
	b.maxSmush = b.smushAmount(b.buffer, curChar)

	bufLen := 0
	if len(b.buffer) > 0 {
		bufLen = utf8.RuneCountInString(b.buffer[0])
	}
	b.currentTotalWidth = bufLen + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		bufCopy := make([]string, len(b.buffer))
		copy(bufCopy, b.buffer)
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: bufCopy, iterator: b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.Height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}

	b.prevCharWidth = b.curCharWidth
	return nil
}

// handleNewLine handles line wrapping or explicit newline.
// Replaces FigletBuilder.handleNewLine.
func (b *FigletBuilder) handleNewLine() {
	if len(b.blankMarkers) > 0 {
		last := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.cutBufferAtLastBlank(last.buffer, last.iterator)
	} else {
		b.cutBufferAtLastChar()
	}
}

// cutBufferAtLastBlank cuts buffer at last space/blank.
// Replaces FigletBuilder.cutBufferAtLastBlank.
func (b *FigletBuilder) cutBufferAtLastBlank(savedBuffer []string, savedIterator int) {
	b.product = append(b.product, savedBuffer)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

// cutBufferAtLastChar cuts buffer when no space/blank marker exists.
// Replaces FigletBuilder.cutBufferAtLastChar.
func (b *FigletBuilder) cutBufferAtLastChar() {
	bufCopy := make([]string, len(b.buffer))
	copy(bufCopy, b.buffer)
	b.product = append(b.product, bufCopy)
	b.iterator--
	b.cutBufferCommon()
}

// cutBufferCommon resets state after cutting buffer.
// Replaces FigletBuilder.cutBufferCommon.
func (b *FigletBuilder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.Height)
	b.blankMarkers = make([]blankMarker, 0)
	b.prevCharWidth = 0
	curChar := b.getCurChar()
	if curChar == nil {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

// smushRow smushes a single row.
// Replaces FigletBuilder.smushRow.
func (b *FigletBuilder) smushRow(curChar []string, row int) ([]rune, []rune) {
	addLeft := []rune(b.buffer[row])
	addRight := []rune(curChar[row])

	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		if idx >= 0 && idx < len(addLeft) && i < len(addRight) {
			left := addLeft[idx]
			right := addRight[i]
			smushed := b.smushChars(left, right)
			if smushed != nil {
				addLeft[idx] = *smushed
			}
		}
	}
	return addLeft, addRight
}

// addCurCharRowToBufferRow adds glyph row to buffer row.
// Replaces FigletBuilder.addCurCharRowToBufferRow.
func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	rightRem := []rune{}
	if b.maxSmush < len(addRight) {
		rightRem = addRight[b.maxSmush:]
	}
	b.buffer[row] = string(append(addLeft, rightRem...))
}

// smushAmount calculates how many columns the new glyph can slide into the buffer.
// Replaces FigletBuilder.smushAmount.
func (b *FigletBuilder) smushAmount(buffer []string, curChar []string) int {
	if (b.font.SmushMode & (SM_SMUSH | SM_KERN)) == 0 {
		return 0
	}

	maxSmush := b.curCharWidth
	for row := 0; row < b.font.Height; row++ {
		lineLeftStr := buffer[row]
		lineRightStr := curChar[row]
		if b.direction == "right-to-left" {
			lineLeftStr, lineRightStr = lineRightStr, lineLeftStr
		}

		lineLeft := []rune(lineLeftStr)
		lineRight := []rune(lineRightStr)

		// rstrip(' ')
		rightTrimLen := len(lineLeft)
		for rightTrimLen > 0 && lineLeft[rightTrimLen-1] == ' ' {
			rightTrimLen--
		}

		linebd := rightTrimLen - 1
		if linebd < 0 {
			linebd = 0
		}

		var ch1 rune
		hasCh1 := false
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
			hasCh1 = true
		} else {
			linebd = 0
		}

		// lstrip(' ')
		leftTrimIdx := 0
		for leftTrimIdx < len(lineRight) && lineRight[leftTrimIdx] == ' ' {
			leftTrimIdx++
		}
		charbd := leftTrimIdx

		var ch2 rune
		hasCh2 := false
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
			hasCh2 = true
		} else {
			charbd = len(lineRight)
		}

		amt := charbd + len(lineLeft) - 1 - linebd

		if !hasCh1 || ch1 == ' ' {
			amt++
		} else if hasCh2 && b.smushChars(ch1, ch2) != nil {
			amt++
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

// smushChars checks whether two touching characters can smush into one.
// Replaces FigletBuilder.smushChars.
func (b *FigletBuilder) smushChars(left, right rune) *rune {
	if left == ' ' {
		res := right
		return &res
	}
	if right == ' ' {
		res := left
		return &res
	}

	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return nil
	}

	if (b.font.SmushMode & SM_SMUSH) == 0 {
		return nil
	}

	var hardBlankRune rune
	if len(b.font.HardBlank) > 0 {
		hardBlankRune = []rune(b.font.HardBlank)[0]
	}

	// Universal smushing: when (smushMode & 63) == 0
	if (b.font.SmushMode & 63) == 0 {
		if left == hardBlankRune {
			res := right
			return &res
		}
		if right == hardBlankRune {
			res := left
			return &res
		}
		if b.direction == "right-to-left" {
			res := left
			return &res
		}
		res := right
		return &res
	}

	// Rule 1: Hardblank smushing (bit 32)
	if (b.font.SmushMode & SM_HARDBLANK) != 0 {
		if left == hardBlankRune && right == hardBlankRune {
			res := left
			return &res
		}
	}

	// Hardblank veto: if either is hardblank, reject overlap
	if left == hardBlankRune || right == hardBlankRune {
		return nil
	}

	// Rule 2: Equal character (bit 1)
	if (b.font.SmushMode & SM_EQUAL) != 0 {
		if left == right {
			res := left
			return &res
		}
	}

	// Rule 3: Lowline / underscore (bit 2)
	// Rule 4: Hierarchy (bit 4)
	type pair struct {
		a string
		b string
	}
	var smushes []pair

	if (b.font.SmushMode & SM_LOWLINE) != 0 {
		smushes = append(smushes, pair{a: "_", b: `|/\[]{}()<>`})
	}

	if (b.font.SmushMode & SM_HIERARCHY) != 0 {
		smushes = append(smushes,
			pair{a: "|", b: `/\[]{}()<>`},
			pair{a: `\/`, b: `[]{}()<>`},
			pair{a: `[]`, b: `{}()<>`},
			pair{a: `{}`, b: `()<>`},
			pair{a: `()`, b: `<>`},
		)
	}

	for _, p := range smushes {
		if strings.ContainsRune(p.a, left) && strings.ContainsRune(p.b, right) {
			res := right
			return &res
		}
		if strings.ContainsRune(p.a, right) && strings.ContainsRune(p.b, left) {
			res := left
			return &res
		}
	}

	// Rule 5: Opposite pairs (bit 8)
	if (b.font.SmushMode & SM_PAIR) != 0 {
		if (left == '[' && right == ']') || (left == ']' && right == '[') ||
			(left == '{' && right == '}') || (left == '}' && right == '{') ||
			(left == '(' && right == ')') || (left == ')' && right == '(') {
			res := '|'
			return &res
		}
	}

	// Rule 6: Big-X (bit 16)
	// Note: Preserve actual Big-X results from pyfiglet source over misleading comment:
	// '/' + '\' -> '|', '\' + '/' -> 'Y', '>' + '<' -> 'X'
	if (b.font.SmushMode & SM_BIGX) != 0 {
		if left == '/' && right == '\\' {
			res := '|'
			return &res
		}
		if left == '\\' && right == '/' {
			res := 'Y'
			return &res
		}
		if left == '>' && right == '<' {
			res := 'X'
			return &res
		}
	}

	return nil
}

// justifyString applies justification padding to each row of the buffer.
// Replaces FigletBuilder.justifyString.
func (b *FigletBuilder) justifyString(justify string, buffer []string) []string {
	if justify == "right" {
		for row := 0; row < b.font.Height; row++ {
			pad := b.width - utf8.RuneCountInString(buffer[row]) - 1
			if pad > 0 {
				buffer[row] = strings.Repeat(" ", pad) + buffer[row]
			}
		}
	} else if justify == "center" {
		for row := 0; row < b.font.Height; row++ {
			pad := (b.width - utf8.RuneCountInString(buffer[row])) / 2
			if pad > 0 {
				buffer[row] = strings.Repeat(" ", pad) + buffer[row]
			}
		}
	}
	return buffer
}

// replaceHardblanks joins rows with newline, appends final newline, and replaces hardblank with space.
// Replaces FigletBuilder.replaceHardblanks.
func (b *FigletBuilder) replaceHardblanks(buffer []string) string {
	str := strings.Join(buffer, "\n") + "\n"
	return strings.ReplaceAll(str, b.font.HardBlank, " ")
}

// ReturnProduct returns the rendered output string.
// Replaces FigletBuilder.returnProduct and FigletBuilder.formatProduct.
func (b *FigletBuilder) ReturnProduct() string {
	if len(b.buffer) > 0 && b.buffer[0] != "" {
		bufCopy := make([]string, len(b.buffer))
		copy(bufCopy, b.buffer)
		b.product = append(b.product, bufCopy)
	}

	var sb strings.Builder
	for _, buf := range b.product {
		bufCopy := make([]string, len(buf))
		copy(bufCopy, buf)
		bufCopy = b.justifyString(b.justify, bufCopy)
		sb.WriteString(b.replaceHardblanks(bufCopy))
	}
	return sb.String()
}
