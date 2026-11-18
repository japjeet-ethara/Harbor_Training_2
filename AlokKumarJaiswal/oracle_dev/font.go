package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

var (
	reMagicNumber = regexp.MustCompile(`^[tf]lf2.`)
	reEndMarker   = regexp.MustCompile(`(.)\s*$`)
)

// FigletFont represents a parsed FIGlet font.
// Replaces Python class FigletFont.
type FigletFont struct {
	Font           string
	Height         int
	HardBlank      string
	PrintDirection *int
	SmushMode      int
	Chars          map[rune][]string
	Width          map[rune]int
	Comment        string
}

// preloadFont locates and loads font data from embedded fonts or filesystem.
// Replaces FigletFont.preloadFont.
func preloadFont(fontName string) (string, error) {
	// 1. Try embedded fonts: fonts/<fontName>.flf
	embeddedPath := "fonts/" + fontName + ".flf"
	if data, err := embeddedFonts.ReadFile(embeddedPath); err == nil {
		return string(data), nil
	}
	if strings.HasSuffix(fontName, ".flf") {
		if data, err := embeddedFonts.ReadFile("fonts/" + fontName); err == nil {
			return string(data), nil
		}
	}

	// 2. Try filesystem
	candidates := []string{
		fontName + ".tlf",
		fontName + ".flf",
		fontName,
		filepath.Join(".", fontName+".tlf"),
		filepath.Join(".", fontName+".flf"),
		filepath.Join(".", fontName),
	}
	for _, cand := range candidates {
		if data, err := os.ReadFile(cand); err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("requested font '%s' not found", fontName)
}

// splitFontLines splits text into lines matching Python's splitlines() behavior.
func splitFontLines(s string) []string {
	if len(s) == 0 {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// parseChar parses a single FIGlet character glyph from data.
// Replaces FigletFont.loadFont.__char.
func parseChar(data *[]string, height int) (int, []string) {
	if len(*data) < height {
		return 0, nil
	}
	var endRegex *regexp.Regexp
	width := 0
	chars := make([]string, 0, height)

	for i := 0; i < height; i++ {
		line := (*data)[0]
		*data = (*data)[1:]

		if endRegex == nil {
			m := reEndMarker.FindStringSubmatch(line)
			if len(m) > 1 {
				endChar := m[1]
				endRegex = regexp.MustCompile(regexp.QuoteMeta(endChar) + `{1,2}\s*$`)
			}
		}

		if endRegex != nil {
			line = endRegex.ReplaceAllString(line, "")
		}

		lineWidth := utf8.RuneCountInString(line)
		if lineWidth > width {
			width = lineWidth
		}
		chars = append(chars, line)
	}
	return width, chars
}

// loadFont parses the raw font file data.
// Replaces FigletFont.loadFont.
func loadFont(fontName, rawData string) (*FigletFont, error) {
	// Remove unicode line splitting characters other than CRLF (U+0085, U+2028, U+2029)
	cleanedData := strings.Map(func(r rune) rune {
		if r == 0x0085 || r == 0x2028 || r == 0x2029 {
			return ' '
		}
		return r
	}, rawData)

	data := splitFontLines(cleanedData)
	if len(data) == 0 {
		return nil, fmt.Errorf("%s is not a valid figlet font", fontName)
	}

	header := data[0]
	data = data[1:]

	if !reMagicNumber.MatchString(header) {
		return nil, fmt.Errorf("%s is not a valid figlet font", fontName)
	}

	header = reMagicNumber.ReplaceAllString(header, "")
	fields := strings.Fields(header)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", fontName)
	}

	hardBlank := fields[0]
	height, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, fmt.Errorf("invalid height: %w", err)
	}
	_, err = strconv.Atoi(fields[2]) // baseLine
	if err != nil {
		return nil, fmt.Errorf("invalid baseLine: %w", err)
	}
	_, err = strconv.Atoi(fields[3]) // maxLength
	if err != nil {
		return nil, fmt.Errorf("invalid maxLength: %w", err)
	}
	oldLayout, err := strconv.Atoi(fields[4])
	if err != nil {
		return nil, fmt.Errorf("invalid oldLayout: %w", err)
	}
	commentLines, err := strconv.Atoi(fields[5])
	if err != nil {
		return nil, fmt.Errorf("invalid commentLines: %w", err)
	}

	var printDirection *int
	var fullLayout *int

	if len(fields) > 6 {
		v, err := strconv.Atoi(fields[6])
		if err == nil {
			printDirection = &v
		}
	}
	if len(fields) > 7 {
		v, err := strconv.Atoi(fields[7])
		if err == nil {
			fullLayout = &v
		}
	}

	var smushMode int
	if fullLayout == nil {
		if oldLayout == 0 {
			smushMode = 64
		} else if oldLayout < 0 {
			smushMode = 0
		} else {
			smushMode = (oldLayout & 31) | 128
		}
	} else {
		smushMode = *fullLayout
	}

	comment := ""
	for i := 0; i < commentLines && len(data) > 0; i++ {
		comment += data[0]
		data = data[1:]
	}

	font := &FigletFont{
		Font:           fontName,
		Height:         height,
		HardBlank:      hardBlank,
		PrintDirection: printDirection,
		SmushMode:      smushMode,
		Chars:          make(map[rune][]string),
		Width:          make(map[rune]int),
		Comment:        comment,
	}

	// ASCII standard character set (32 - 126)
	for i := 32; i < 127; i++ {
		if len(data) < height {
			break
		}
		width, letter := parseChar(&data, height)
		allEmpty := true
		for _, l := range letter {
			if l != "" {
				allEmpty = false
				break
			}
		}
		if i == 32 || !allEmpty {
			font.Chars[rune(i)] = letter
			font.Width[rune(i)] = width
		}
	}

	// German Umlaute (ÄÖÜäöüß)
	if len(data) > 0 {
		for _, r := range "ÄÖÜäöüß" {
			if len(data) < height {
				break
			}
			width, letter := parseChar(&data, height)
			allEmpty := true
			for _, l := range letter {
				if l != "" {
					allEmpty = false
					break
				}
			}
			if !allEmpty {
				font.Chars[r] = letter
				font.Width[r] = width
			}
		}
	}

	// Extended characters (tokens matching ^0x)
	for len(data) > 0 {
		line := strings.TrimSpace(data[0])
		data = data[1:]
		parts := strings.SplitN(line, " ", 2)
		tok := parts[0]
		if tok == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(tok), "0x") {
			code, err := strconv.ParseInt(tok[2:], 16, 64)
			if err == nil {
				if len(data) < height {
					break
				}
				width, letter := parseChar(&data, height)
				allEmpty := true
				for _, l := range letter {
					if l != "" {
						allEmpty = false
						break
					}
				}
				if !allEmpty {
					font.Chars[rune(code)] = letter
					font.Width[rune(code)] = width
				}
			}
		}
	}

	return font, nil
}

// loadFontByName preloads and parses a font.
// Replaces FigletFont.__init__.
func loadFontByName(fontName string) (*FigletFont, error) {
	data, err := preloadFont(fontName)
	if err != nil {
		return nil, err
	}
	return loadFont(fontName, data)
}
