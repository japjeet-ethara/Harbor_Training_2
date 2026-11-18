package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

//go:embed pyfiglet/fonts/*.flf
var fonts embed.FS

type rowExtent struct {
	length, first, last int
}

type canvas struct {
	columns [][]rune
	rows    []rowExtent
}

func newCanvas(height, width int) canvas {
	c := canvas{columns: make([][]rune, width), rows: make([]rowExtent, height)}
	for y := range c.rows {
		c.rows[y].last = -1
	}
	for x := range c.columns {
		c.columns[x] = make([]rune, height)
	}
	return c
}

func (c *canvas) setRow(y int, cells []rune) {
	edge := rowExtent{length: len(cells), first: len(cells), last: -1}
	for x, cell := range cells {
		c.columns[x][y] = cell
		if cell != ' ' {
			edge.first = min(edge.first, x)
			edge.last = x
		}
	}
	c.rows[y] = edge
}

type figFont struct {
	height, layout int
	hardblank      rune
	rtl            bool
	glyphs         map[rune]canvas
}

func loadFont(name string) (*figFont, error) {
	data, err := fonts.ReadFile("pyfiglet/fonts/" + name + ".flf")
	if err != nil {
		return nil, err
	}
	if archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil && len(archive.File) != 0 {
		entry, err := archive.File[0].Open()
		if err != nil {
			return nil, err
		}
		defer entry.Close()
		data, err = io.ReadAll(entry)
		if err != nil {
			return nil, err
		}
	}
	return decodeFont(string(data))
}

func pythonSpace(r rune) bool {
	return unicode.IsSpace(r) || r >= '\x1c' && r <= '\x1f'
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	skipLF := false
	for index, ch := range text {
		if skipLF && ch == '\n' {
			start = index + 1
			skipLF = false
			continue
		}
		skipLF = ch == '\r'
		switch ch {
		case '\r', '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\u0085', '\u2028', '\u2029':
			lines = append(lines, text[start:index])
			start = index + len(string(ch))
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

func decodeFont(content string) (*figFont, error) {
	content = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(content)
	lines := splitLines(content)
	if len(lines) == 0 || !(strings.HasPrefix(lines[0], "flf2") || strings.HasPrefix(lines[0], "tlf2")) {
		return nil, fmt.Errorf("invalid FIGlet header")
	}
	header := []rune(lines[0])
	if len(header) < 6 {
		return nil, fmt.Errorf("short FIGlet header")
	}
	fields := strings.FieldsFunc(string(header[5:]), pythonSpace)
	if len(fields) < 6 {
		return nil, fmt.Errorf("missing FIGlet header fields")
	}
	values := make([]int, min(len(fields)-1, 7))
	for i := range values {
		n, err := strconv.Atoi(fields[i+1])
		if err != nil {
			return nil, err
		}
		values[i] = n
	}
	height, oldLayout, comments := values[0], values[3], values[4]
	if height < 1 || comments < 0 || comments >= len(lines) {
		return nil, fmt.Errorf("invalid height or comment count")
	}
	font := &figFont{height: height, hardblank: []rune(fields[0])[0], glyphs: make(map[rune]canvas)}
	switch {
	case len(values) > 6:
		font.layout = values[6]
	case oldLayout == 0:
		font.layout = kern
	case oldLayout > 0:
		font.layout = oldLayout&31 | smush
	}
	font.rtl = len(values) > 5 && values[5] == 1
	reader := glyphReader{lines: lines[comments+1:], font: font}
	for ch := rune(32); ch < 127; ch++ {
		if err := reader.read(ch); err != nil {
			return nil, err
		}
	}
	if len(reader.lines) != 0 {
		for _, ch := range "ÄÖÜäöüß" {
			if err := reader.read(ch); err != nil {
				return nil, err
			}
		}
	}
	for len(reader.lines) != 0 {
		tag, _, _ := strings.Cut(strings.TrimFunc(reader.lines[0], pythonSpace), " ")
		reader.lines = reader.lines[1:]
		if strings.HasPrefix(strings.ToLower(tag), "0x") {
			code, err := strconv.ParseUint(tag[2:], 16, 32)
			if err != nil {
				return nil, err
			}
			if err := reader.read(rune(code)); err != nil {
				return nil, err
			}
		}
	}
	return font, nil
}

type glyphReader struct {
	lines []string
	font  *figFont
}

func (r *glyphReader) read(ch rune) error {
	h := r.font.height
	if len(r.lines) < h {
		return fmt.Errorf("truncated glyph U+%04X", ch)
	}
	first := []rune(strings.TrimRightFunc(r.lines[0], pythonSpace))
	if len(first) == 0 {
		first = []rune(r.lines[0])
	}
	if len(first) == 0 {
		return fmt.Errorf("missing glyph end marker for U+%04X", ch)
	}
	marker := first[len(first)-1]
	if pythonSpace(marker) {
		marker = first[0]
	}
	var width int
	rows := make([][]rune, h)
	for y, line := range r.lines[:h] {
		trimmed := []rune(strings.TrimRightFunc(line, pythonSpace))
		end := len(trimmed)
		if end != 0 && trimmed[end-1] == marker {
			end--
			if end != 0 && trimmed[end-1] == marker {
				end--
			}
			line = string(trimmed[:end])
		} else if pythonSpace(marker) {
			cells := []rune(line)
			for index := end; index < len(cells); index++ {
				if cells[index] == marker {
					line = string(cells[:index])
					break
				}
			}
		}
		rows[y] = []rune(line)
		width = max(width, len(rows[y]))
	}
	r.lines = r.lines[h:]
	if width != 0 || ch == ' ' {
		glyph := newCanvas(h, width)
		for y, row := range rows {
			glyph.setRow(y, row)
		}
		r.font.glyphs[ch] = glyph
	}
	return nil
}
