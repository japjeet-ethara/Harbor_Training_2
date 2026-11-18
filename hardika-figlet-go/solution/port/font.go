package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:embed fonts/*.flf
var fontFiles embed.FS

type layoutMode int

const (
	equal layoutMode = 1 << iota
	lowline
	hierarchy
	opposite
	bigX
	hardblankPair
	kerning
	smushing
	ruleBits = kerning - 1
)

type glyph struct {
	rows  []string
	width int
}

type fontLayout struct {
	height     int
	hardblank  rune
	rtl        bool
	horizontal layoutMode
}

type font struct {
	layout fontLayout
	glyphs map[rune]glyph
}

func loadFont(name string) (font, error) {
	if path.Base(name) != name {
		return font{}, fmt.Errorf("invalid font name %q", name)
	}
	raw, err := fontFiles.ReadFile("fonts/" + name + ".flf")
	if err != nil {
		return font{}, err
	}
	if archive, zipErr := zip.NewReader(bytes.NewReader(raw), int64(len(raw))); zipErr == nil && len(archive.File) != 0 {
		entry, err := archive.File[0].Open()
		if err != nil {
			return font{}, err
		}
		defer entry.Close()
		raw, err = io.ReadAll(entry)
		if err != nil {
			return font{}, err
		}
	}
	return parseFont(decodeFont(raw))
}

func decodeFont(raw []byte) string {
	raw = bytes.ReplaceAll(raw, []byte{0xc2, 0x85}, []byte(" "))
	raw = bytes.ReplaceAll(raw, []byte{0xe2, 0x80, 0xa8}, []byte(" "))
	raw = bytes.ReplaceAll(raw, []byte{0xe2, 0x80, 0xa9}, []byte(" "))
	return string(raw)
}

func isPythonSpace(ch rune) bool {
	return unicode.IsSpace(ch) || ch >= 0x1c && ch <= 0x1f
}

func isLineBreak(ch rune) bool {
	switch ch {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	default:
		return false
	}
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for index, ch := range text {
		if index < start || !isLineBreak(ch) {
			continue
		}
		lines = append(lines, text[start:index])
		start = index + utf8.RuneLen(ch)
		if ch == '\r' && start < len(text) && text[start] == '\n' {
			start++
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

func parseFont(data string) (font, error) {
	lines := splitLines(data)
	if len(lines) == 0 || !(strings.HasPrefix(lines[0], "flf2") || strings.HasPrefix(lines[0], "tlf2")) {
		return font{}, fmt.Errorf("invalid font signature")
	}
	_, magicSize := utf8.DecodeRuneInString(lines[0][4:])
	fields := strings.FieldsFunc(lines[0][4+magicSize:], isPythonSpace)
	if len(fields) < 6 {
		return font{}, fmt.Errorf("incomplete font header")
	}
	limit := min(len(fields), 8)
	values := make([]int, limit)
	for index := 1; index < limit; index++ {
		value, err := strconv.Atoi(fields[index])
		if err != nil {
			return font{}, fmt.Errorf("invalid font header field %d: %w", index, err)
		}
		values[index] = value
	}
	height, oldLayout, commentLines := values[1], values[4], values[5]
	if height <= 0 || commentLines < 0 || commentLines >= len(lines) {
		return font{}, fmt.Errorf("invalid font height or comment count")
	}

	layout := layoutMode(0)
	switch {
	case len(values) > 7:
		layout = layoutMode(values[7])
	case oldLayout == 0:
		layout = kerning
	case oldLayout > 0:
		layout = layoutMode(oldLayout&31) | smushing
	}
	blank, _ := utf8.DecodeRuneInString(fields[0])
	result := font{
		layout: fontLayout{
			height:     height,
			hardblank:  blank,
			rtl:        len(values) > 6 && values[6] == 1,
			horizontal: layout,
		},
		glyphs: make(map[rune]glyph),
	}
	index := 1 + commentLines
	for ch := ' '; ch <= '~'; ch++ {
		if err := result.readGlyph(lines, &index, ch); err != nil {
			return font{}, err
		}
	}
	if index < len(lines) {
		for _, ch := range "ÄÖÜäöüß" {
			if err := result.readGlyph(lines, &index, ch); err != nil {
				return font{}, err
			}
		}
	}
	for index < len(lines) {
		tag, _, _ := strings.Cut(strings.TrimFunc(lines[index], isPythonSpace), " ")
		index++
		if !strings.HasPrefix(strings.ToLower(tag), "0x") {
			continue
		}
		code, err := strconv.ParseUint(tag[2:], 16, 32)
		if err != nil {
			return font{}, fmt.Errorf("invalid glyph tag %q: %w", tag, err)
		}
		if err := result.readGlyph(lines, &index, rune(code)); err != nil {
			return font{}, err
		}
	}
	return result, nil
}

func (f *font) readGlyph(lines []string, index *int, ch rune) error {
	height := f.layout.height
	if height > len(lines)-*index {
		return fmt.Errorf("truncated glyph %U", ch)
	}
	first := strings.TrimRightFunc(lines[*index], isPythonSpace)
	if first == "" {
		return fmt.Errorf("missing endmark for %U", ch)
	}
	_, markerSize := utf8.DecodeLastRuneInString(first)
	marker := first[len(first)-markerSize:]
	g := glyph{rows: make([]string, height)}
	for row := range g.rows {
		line := lines[*index+row]
		trimmed := strings.TrimRightFunc(line, isPythonSpace)
		if strings.HasSuffix(trimmed, marker) {
			line = strings.TrimSuffix(strings.TrimSuffix(trimmed, marker), marker)
		}
		line = string([]rune(line))
		g.rows[row] = line
		g.width = max(g.width, utf8.RuneCountInString(line))
	}
	*index += height
	if ch == ' ' || g.width != 0 {
		f.glyphs[ch] = g
	}
	return nil
}
