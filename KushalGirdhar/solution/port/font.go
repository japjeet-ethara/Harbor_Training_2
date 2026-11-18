package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed pyfiglet/fonts/*.flf
var standardFonts embed.FS

type glyph struct {
	rows  [][]rune
	width int
}

type figFont struct {
	glyphs    map[rune]glyph
	height    int
	hardblank rune
	direction int
	layout    int
}

func loadFont(name string) (*figFont, error) {
	for _, suffix := range []string{".tlf", ".flf"} {
		filename := name + suffix
		data, err := standardFonts.ReadFile("pyfiglet/fonts/" + filename)
		if err == nil {
			return parseFontData(data)
		}
		for _, directory := range []string{".", "/usr/local/share/pyfiglet", "/usr/local/share/figlet", "/usr/share/figlet"} {
			path := filepath.Join(directory, filename)
			if filepath.IsAbs(filename) {
				path = filename
			}
			data, err = os.ReadFile(path)
			if err == nil {
				return parseFontData(data)
			}
		}
	}
	return nil, fmt.Errorf("font %q unavailable", name)
}

func parseFontData(data []byte) (*figFont, error) {
	if archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil && len(archive.File) > 0 {
		file, err := archive.File[0].Open()
		if err != nil {
			return nil, err
		}
		defer file.Close()
		data, err = io.ReadAll(file)
		if err != nil {
			return nil, err
		}
	}
	decoded := decodeUTF8(string(data))
	decoded = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(decoded)
	return parseFont(splitLines(decoded))
}

func decodeUTF8(data string) string {
	if utf8.ValidString(data) {
		return data
	}
	var decoded strings.Builder
	for len(data) > 0 {
		cell, size := utf8.DecodeRuneInString(data)
		if cell == utf8.RuneError && size == 1 {
			for size < len(data) && !utf8.FullRuneInString(data[:size+1]) {
				size++
			}
		}
		decoded.WriteRune(cell)
		data = data[size:]
	}
	return decoded.String()
}

func parseFont(lines []string) (*figFont, error) {
	if len(lines) == 0 || (!strings.HasPrefix(lines[0], "flf2") && !strings.HasPrefix(lines[0], "tlf2")) {
		return nil, fmt.Errorf("invalid font signature")
	}
	_, signatureSize := utf8.DecodeRuneInString(lines[0][4:])
	fields := strings.FieldsFunc(lines[0][4+signatureSize:], pythonSpace)
	if len(fields) < 6 || utf8.RuneCountInString(fields[0]) != 1 {
		return nil, fmt.Errorf("incomplete font header")
	}
	values := make([]int, 7)
	for i, field := range fields[1:min(len(fields), 8)] {
		value, err := strconv.Atoi(field)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	height, oldLayout, comments := values[0], values[3], values[4]
	if height <= 0 || comments < 0 || comments >= len(lines) {
		return nil, fmt.Errorf("invalid font height or comment count")
	}
	hardblank, _ := utf8.DecodeRuneInString(fields[0])
	layout := values[6]
	if len(fields) < 8 {
		switch {
		case oldLayout == 0:
			layout = kern
		case oldLayout > 0:
			layout = oldLayout&31 | smush
		}
	}
	font := &figFont{make(map[rune]glyph), height, hardblank, values[5], layout}
	reader := glyphReader{lines: lines[1+comments:], font: font}
	for cell := ' '; cell <= '~'; cell++ {
		if err := reader.read(cell); err != nil {
			return nil, err
		}
	}
	if len(reader.lines) > 0 {
		for _, cell := range "ÄÖÜäöüß" {
			if err := reader.read(cell); err != nil {
				return nil, err
			}
		}
	}
	for len(reader.lines) > 0 {
		tag, _, _ := strings.Cut(strings.TrimFunc(reader.lines[0], pythonSpace), " ")
		reader.lines = reader.lines[1:]
		if !strings.HasPrefix(strings.ToLower(tag), "0x") {
			continue
		}
		code, err := strconv.ParseInt(tag[2:], 16, 32)
		if err != nil {
			return nil, err
		}
		if err := reader.read(rune(code)); err != nil {
			return nil, err
		}
	}
	return font, nil
}

type glyphReader struct {
	lines []string
	font  *figFont
}

func (reader *glyphReader) read(cell rune) error {
	if len(reader.lines) < reader.font.height {
		return fmt.Errorf("incomplete glyph %q", cell)
	}
	firstRow := strings.TrimRightFunc(reader.lines[0], pythonSpace)
	if firstRow == "" {
		return fmt.Errorf("no endmark for glyph %q", cell)
	}
	endmark, _ := utf8.DecodeLastRuneInString(firstRow)
	letter := glyph{rows: make([][]rune, reader.font.height)}
	for i, line := range reader.lines[:reader.font.height] {
		cells := []rune(line)
		end := len(cells)
		for end > 0 && pythonSpace(cells[end-1]) {
			end--
		}
		for removed := 0; removed < 2 && end > 0 && cells[end-1] == endmark; removed++ {
			end--
			line = string(cells[:end])
		}
		letter.rows[i] = []rune(line)
		letter.width = max(letter.width, len(letter.rows[i]))
	}
	reader.lines = reader.lines[reader.font.height:]
	if cell == ' ' || letter.width > 0 {
		reader.font.glyphs[cell] = letter
	}
	return nil
}
