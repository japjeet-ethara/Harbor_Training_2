package main

import (
	"fmt"
	"strings"
)

const (
	equal = 1 << iota
	lowline
	hierarchy
	pair
	bigX
	hardblank
	kern
	smush
)

type collisionRules struct {
	layout                  int
	hard                    rune
	rtl                     bool
	previousWidth, newWidth int
}

func (rules collisionRules) resolve(left, right rune) (rune, bool) {
	switch {
	case left == ' ':
		return right, true
	case right == ' ':
		return left, true
	case rules.previousWidth < 2 || rules.newWidth < 2 || rules.layout&smush == 0:
		return 0, false
	case rules.layout&63 == 0:
		switch {
		case left == rules.hard:
			return right, true
		case right == rules.hard || rules.rtl:
			return left, true
		default:
			return right, true
		}
	case left == rules.hard || right == rules.hard:
		return left, left == right && rules.layout&hardblank != 0
	case rules.layout&equal != 0 && left == right:
		return left, true
	}

	leftRank := hierarchyRank(left)
	rightRank := hierarchyRank(right)
	if rules.layout&lowline != 0 {
		switch {
		case left == '_' && rightRank != 0:
			return right, true
		case right == '_' && leftRank != 0:
			return left, true
		}
	}
	if rules.layout&hierarchy != 0 && leftRank != 0 && rightRank != 0 && leftRank != rightRank {
		if leftRank > rightRank {
			return left, true
		}
		return right, true
	}
	if rules.layout&pair != 0 {
		switch string([]rune{left, right}) {
		case "[]", "][", "{}", "}{", "()", ")(":
			return '|', true
		}
	}
	if rules.layout&bigX != 0 {
		switch string([]rune{left, right}) {
		case "/\\":
			return '|', true
		case "\\/":
			return 'Y', true
		case "><":
			return 'X', true
		}
	}
	return 0, false
}

func hierarchyRank(ch rune) int {
	switch ch {
	case '|':
		return 1
	case '/', '\\':
		return 2
	case '[', ']':
		return 3
	case '{', '}':
		return 4
	case '(', ')':
		return 5
	case '<', '>':
		return 6
	}
	return 0
}

func discoverOverlap(left, right canvas, rules collisionRules) int {
	if rules.layout&(smush|kern) == 0 {
		return 0
	}
	overlap := rules.newWidth
	for y, l := range left.rows {
		r := right.rows[y]
		available := max(0, l.last)
		amount := r.first + l.length - 1 - available
		if l.last < 0 {
			amount++
		} else if r.first < r.length {
			if _, ok := rules.resolve(left.columns[l.last][y], right.columns[r.first][y]); ok {
				amount++
			}
		}
		overlap = min(overlap, amount)
	}
	return overlap
}

func joinCanvases(left, right canvas, overlap int, rules collisionRules) (canvas, error) {
	width := 0
	for y, l := range left.rows {
		width = max(width, l.length+max(0, right.rows[y].length-overlap))
	}
	result := newCanvas(len(left.rows), width)
	for y, l := range left.rows {
		r := right.rows[y]
		row := make([]rune, l.length+max(0, r.length-overlap))
		for x := 0; x < l.length; x++ {
			row[x] = left.columns[x][y]
		}
		for x := max(0, overlap-l.length); x < min(overlap, r.length); x++ {
			index := l.length - overlap + x
			cell, ok := rules.resolve(row[index], right.columns[x][y])
			if !ok {
				return canvas{}, fmt.Errorf("invalid overlap")
			}
			row[index] = cell
		}
		for x := overlap; x < r.length; x++ {
			row[l.length+x-overlap] = right.columns[x][y]
		}
		result.setRow(y, row)
	}
	return result, nil
}

type checkpoint struct {
	position int
	content  canvas
}

type lineState struct {
	content       canvas
	lastBreak     checkpoint
	previousWidth int
}

func emptyLine(height int) lineState {
	return lineState{content: newCanvas(height, 0), lastBreak: checkpoint{position: -1}}
}

func resolvedDirection(font *figFont, option string) bool {
	return option == "right-to-left" || option == "auto" && font.rtl
}

func resolvedJustification(option string, rtl bool) string {
	if option != "auto" {
		return option
	}
	if rtl {
		return "right"
	}
	return "left"
}

func render(text string, font *figFont, opts options) (string, error) {
	rtl := resolvedDirection(font, opts.direction)
	justify := resolvedJustification(opts.justify, rtl)
	line := emptyLine(font.height)
	var output strings.Builder
	for index, chars := 0, []rune(text); index < len(chars); index++ {
		ch := chars[index]
		if ch == '\n' {
			writeLine(&output, line.content, opts.width, justify, font.hardblank)
			line = emptyLine(font.height)
			continue
		}
		glyph, exists := font.glyphs[ch]
		if !exists {
			continue
		}
		width := len(glyph.columns)
		if width > opts.width {
			return "", fmt.Errorf("glyph wider than rendering width")
		}
		rules := collisionRules{font.layout, font.hardblank, rtl, line.previousWidth, width}
		left, right := line.content, glyph
		if rtl {
			left, right = right, left
		}
		overlap := discoverOverlap(left, right, rules)
		if ch == ' ' {
			line.lastBreak = checkpoint{index, line.content}
		}
		if line.content.rows[0].length+width-overlap >= opts.width {
			if line.lastBreak.position >= 0 {
				writeLine(&output, line.lastBreak.content, opts.width, justify, font.hardblank)
				index = line.lastBreak.position
			} else {
				if len(line.content.columns) == 0 {
					return "", fmt.Errorf("glyph cannot fit on an empty line")
				}
				writeLine(&output, line.content, opts.width, justify, font.hardblank)
				index--
			}
			line = emptyLine(font.height)
			line.previousWidth = width
			continue
		}
		joined, err := joinCanvases(left, right, overlap, rules)
		if err != nil {
			return "", err
		}
		line.content = joined
		line.previousWidth = width
	}
	if line.content.rows[0].length != 0 {
		writeLine(&output, line.content, opts.width, justify, font.hardblank)
	}
	return output.String(), nil
}

func writeLine(output *strings.Builder, line canvas, width int, justify string, hard rune) {
	for y, edge := range line.rows {
		padding := 0
		switch justify {
		case "right":
			padding = width - edge.length - 1
		case "center":
			padding = (width - edge.length) / 2
		}
		output.WriteString(strings.Repeat(" ", max(0, padding)))
		for x := 0; x < edge.length; x++ {
			cell := line.columns[x][y]
			if cell == hard {
				cell = ' '
			}
			output.WriteRune(cell)
		}
		output.WriteByte('\n')
	}
}
