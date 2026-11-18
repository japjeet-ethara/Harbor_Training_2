package main

import (
	"fmt"
	"slices"
	"strings"
)

type lineBreakState struct {
	index int
	rows  [][]rune
}

type lineLayout struct {
	glyphs    glyphBuilder
	width     int
	justify   string
	lastBreak lineBreakState
	output    strings.Builder
}

func render(font *figFont, opts options, input string) (string, error) {
	rtl := opts.direction == "right-to-left" || (opts.direction == "auto" && font.direction == 1)
	justify := opts.justify
	if justify == "auto" {
		justify = "left"
		if rtl {
			justify = "right"
		}
	}
	line := lineLayout{
		glyphs:    glyphBuilder{font: font, rows: make([][]rune, font.height), rtl: rtl},
		width:     opts.width,
		justify:   justify,
		lastBreak: lineBreakState{index: -1},
	}
	text := []rune(input)
	for cursor := 0; cursor < len(text); cursor++ {
		cell := text[cursor]
		if cell == '\n' {
			line.emit(line.glyphs.rows)
			line.reset()
			continue
		}
		next, found := font.glyphs[cell]
		if !found {
			continue
		}
		if opts.width < next.width {
			return "", fmt.Errorf("width is too small for %q", cell)
		}
		line.glyphs.currentSize = next.width
		overlap := line.glyphs.allowedOverlap(next)
		if cell == ' ' {
			line.lastBreak = lineBreakState{cursor, slices.Clone(line.glyphs.rows)}
		}
		if len(line.glyphs.rows[0])+next.width-overlap >= line.width {
			if line.lastBreak.index >= 0 {
				line.emit(line.lastBreak.rows)
				cursor = line.lastBreak.index
			} else {
				if next.width >= line.width {
					return "", fmt.Errorf("width leaves no room to wrap %q", cell)
				}
				line.emit(line.glyphs.rows)
				cursor--
			}
			line.reset()
		} else if err := line.glyphs.insert(next, overlap); err != nil {
			return "", err
		}
		line.glyphs.previousSize = next.width
	}
	if len(line.glyphs.rows[0]) > 0 {
		line.emit(line.glyphs.rows)
	}
	return line.output.String(), nil
}

func (line *lineLayout) reset() {
	clear(line.glyphs.rows)
	line.glyphs.previousSize = 0
	line.lastBreak = lineBreakState{index: -1}
}

func (line *lineLayout) emit(rows [][]rune) {
	for _, row := range rows {
		padding := 0
		switch line.justify {
		case "right":
			padding = line.width - len(row) - 1
		case "center":
			padding = (line.width - len(row)) / 2
		}
		line.output.WriteString(strings.Repeat(" ", max(0, padding)))
		for _, cell := range row {
			if cell == line.glyphs.font.hardblank {
				cell = ' '
			}
			line.output.WriteRune(cell)
		}
		line.output.WriteByte('\n')
	}
}
