package main

import (
	"fmt"
	"strings"
	"unicode"
)

var reverseMap = [256]rune{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 41, 40, 42, 43, 44, 45, 46, 92, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 62, 61, 60, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 93, 47, 91, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 125, 124, 123, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164, 165, 166, 167, 168, 169, 170, 171, 172, 173, 174, 175, 176, 177, 178, 179, 180, 181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191, 192, 193, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239, 240, 241, 242, 243, 244, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}
var flipMap = [256]rune{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 92, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 86, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 87, 78, 79, 98, 81, 98, 83, 84, 85, 65, 77, 88, 89, 90, 91, 47, 93, 118, 45, 96, 97, 80, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 119, 110, 111, 112, 113, 114, 115, 116, 117, 94, 109, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164, 165, 166, 167, 168, 169, 170, 171, 172, 173, 174, 175, 176, 177, 178, 179, 180, 181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191, 192, 193, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239, 240, 241, 242, 243, 244, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255}

type blankMarker struct {
	buffer   []string
	iterator int
}

type FigletBuilder struct {
	text          []rune
	font          *FigletFont
	direction     string
	width         int
	justify       string
	iterator      int
	maxSmush      int
	curCharWidth  int
	prevCharWidth int
	currentTotal  int
	blankMarkers  []blankMarker
	queue         [][]string
	buffer        []string
}

func copyBuffer(buf []string) []string {
	res := make([]string, len(buf))
	copy(res, buf)
	return res
}

func (b *FigletBuilder) getCurChar() ([]string, bool) {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return nil, false
	}
	ch, ok := b.font.Chars[b.text[b.iterator]]
	return ch, ok
}

func (b *FigletBuilder) getCurWidth() int {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return 0
	}
	return b.font.Width[b.text[b.iterator]]
}

func (b *FigletBuilder) currentSmushAmount(curChar []string) int {
	return smushAmount(b.font, b.buffer, curChar, b.prevCharWidth, b.curCharWidth, b.direction)
}

func (b *FigletBuilder) cutBufferCommon() {
	b.currentTotal = 0
	b.buffer = make([]string, b.font.Height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	if curChar, ok := b.getCurChar(); ok {
		b.maxSmush = b.currentSmushAmount(curChar)
	}
}

func (b *FigletBuilder) handleNewLine() {
	if len(b.blankMarkers) > 0 {
		last := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.queue = append(b.queue, last.buffer)
		b.iterator = last.iterator
		b.cutBufferCommon()
	} else {
		b.queue = append(b.queue, copyBuffer(b.buffer))
		b.iterator--
		b.cutBufferCommon()
	}
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
	b.buffer[row] = smushRow(b.font, b.buffer[row], curChar[row], b.maxSmush, b.prevCharWidth, b.curCharWidth, b.direction)
}

func (b *FigletBuilder) justifyBuffer(buffer []string) []string {
	res := make([]string, b.font.Height)
	if b.justify == "right" {
		for row := 0; row < b.font.Height; row++ {
			padLen := b.width - len(buffer[row]) - 1
			if padLen > 0 {
				res[row] = strings.Repeat(" ", padLen) + buffer[row]
			} else {
				res[row] = buffer[row]
			}
		}
	} else if b.justify == "center" {
		for row := 0; row < b.font.Height; row++ {
			padLen := (b.width - len(buffer[row])) / 2
			if padLen > 0 {
				res[row] = strings.Repeat(" ", padLen) + buffer[row]
			} else {
				res[row] = buffer[row]
			}
		}
	} else {
		copy(res, buffer)
	}
	return res
}

func (b *FigletBuilder) formatProduct() string {
	var sb strings.Builder
	for _, buf := range b.queue {
		justified := b.justifyBuffer(buf)
		lineStr := strings.Join(justified, "\n") + "\n"
		lineStr = strings.ReplaceAll(lineStr, string(b.font.HardBlank), " ")
		sb.WriteString(lineStr)
	}
	return sb.String()
}

func (b *FigletBuilder) Render() (string, error) {
	for b.iterator < len(b.text) {
		curCode := b.text[b.iterator]

		if curCode == '\n' {
			b.blankMarkers = append(b.blankMarkers, blankMarker{
				buffer:   copyBuffer(b.buffer),
				iterator: b.iterator,
			})
			b.handleNewLine()
			b.iterator++
			continue
		}

		curChar, ok := b.getCurChar()
		if !ok {
			// Missing character in font is ignored
			b.iterator++
			continue
		}

		curWidth := b.getCurWidth()
		if b.width < curWidth {
			return "", fmt.Errorf("width is not enough to print this character")
		}

		b.curCharWidth = curWidth
		b.maxSmush = b.currentSmushAmount(curChar)
		b.currentTotal = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

		if curCode == ' ' {
			b.blankMarkers = append(b.blankMarkers, blankMarker{
				buffer:   copyBuffer(b.buffer),
				iterator: b.iterator,
			})
		}

		if b.currentTotal >= b.width {
			b.handleNewLine()
		} else {
			for row := 0; row < b.font.Height; row++ {
				b.addCurCharRowToBufferRow(curChar, row)
			}
		}

		b.prevCharWidth = b.curCharWidth
		b.iterator++
	}

	if b.buffer[0] != "" {
		b.queue = append(b.queue, copyBuffer(b.buffer))
	}

	return b.formatProduct(), nil
}

func ReverseFiglet(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out []string
	for _, row := range lines {
		runes := []rune(row)
		for i, r := range runes {
			if int(r) < len(reverseMap) {
				runes[i] = reverseMap[r]
			}
		}
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

func FlipFiglet(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		row := lines[i]
		runes := []rune(row)
		for j, r := range runes {
			if int(r) < len(flipMap) {
				runes[j] = flipMap[r]
			}
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

func StripSurroundingNewlines(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var out []string
	charsSeen := false
	for _, row := range lines {
		if strings.TrimSpace(row) != "" || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}

	joined := strings.Join(out, "\n") + "\n"
	return strings.TrimRightFunc(joined, unicode.IsSpace)
}

func NormalizeSurroundingNewlines(s string) string {
	return "\n" + StripSurroundingNewlines(s) + "\n"
}

func Render(font *FigletFont, text string, direction, justify string, width int) (string, error) {
	// Resolve direction
	effectiveDir := direction
	if effectiveDir == "auto" {
		if font.PrintDirection == 1 {
			effectiveDir = "right-to-left"
		} else {
			effectiveDir = "left-to-right"
		}
	}

	// Resolve justification
	effectiveJust := justify
	if effectiveJust == "auto" {
		if effectiveDir == "right-to-left" {
			effectiveJust = "right"
		} else {
			effectiveJust = "left"
		}
	}

	builder := &FigletBuilder{
		text:      []rune(text),
		font:      font,
		direction: effectiveDir,
		width:     width,
		justify:   effectiveJust,
		buffer:    make([]string, font.Height),
	}

	return builder.Render()
}
