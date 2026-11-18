package main

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

var (
	reMagicNumber = regexp.MustCompile(`^[tf]lf2.`)
	reEndMarker   = regexp.MustCompile(`(.)\s*$`)
)

type FigletFont struct {
	Name           string
	Height         int
	BaseLine       int
	MaxLength      int
	OldLayout      int
	CommentLines   int
	PrintDirection int // 0 = LTR, 1 = RTL
	SmushMode      int
	HardBlank      rune
	Chars          map[rune][]string
	Width          map[rune]int
}

func loadFontData(fontName string) ([]byte, error) {
	// Try embedded font first
	for _, ext := range []string{"flf", "tlf"} {
		path := fmt.Sprintf("fonts/%s.%s", fontName, ext)
		if data, err := embeddedFonts.ReadFile(path); err == nil {
			return data, nil
		}
	}

	// Try filesystem locations
	candidateDirs := []string{
		".",
		"./fonts",
		"./pyfiglet/fonts",
		"./pyfiglet/fonts-standard",
		"/workspace/repo/pyfiglet/fonts",
		"/workspace/repo/pyfiglet/fonts-standard",
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidateDirs = append(candidateDirs,
			filepath.Join(exeDir, "fonts"),
			filepath.Join(exeDir, "pyfiglet", "fonts"),
			filepath.Join(exeDir, "pyfiglet", "fonts-standard"),
		)
	}

	for _, dir := range candidateDirs {
		for _, ext := range []string{"flf", "tlf"} {
			filePath := filepath.Join(dir, fmt.Sprintf("%s.%s", fontName, ext))
			if data, err := os.ReadFile(filePath); err == nil {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("pyfiglet error: requested font %q not found", fontName)
}

func parseChar(lines *[]string, height int) (int, []string, error) {
	if len(*lines) < height {
		return 0, nil, io.ErrUnexpectedEOF
	}

	var endRegex *regexp.Regexp
	charLines := make([]string, 0, height)
	maxWidth := 0

	for h := 0; h < height; h++ {
		line := (*lines)[0]
		*lines = (*lines)[1:]

		if endRegex == nil {
			match := reEndMarker.FindStringSubmatch(line)
			if match == nil {
				return 0, nil, fmt.Errorf("missing end marker in line: %q", line)
			}
			endChar := regexp.QuoteMeta(match[1])
			endRegex = regexp.MustCompile(endChar + `{1,2}\s*$`)
		}

		line = endRegex.ReplaceAllString(line, "")
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
		charLines = append(charLines, line)
	}

	return maxWidth, charLines, nil
}

func LoadFont(fontName string) (*FigletFont, error) {
	raw, err := loadFontData(fontName)
	if err != nil {
		return nil, err
	}

	content := string(raw)
	// Replace non-CRLF unicode line splitting characters (\u0085, \u2028, \u2029) with space
	content = strings.Map(func(r rune) rune {
		if r == 0x0085 || r == 0x2028 || r == 0x2029 {
			return ' '
		}
		return r
	}, content)

	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")

	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font file: %s", fontName)
	}

	headerLine := lines[0]
	lines = lines[1:]

	if !reMagicNumber.MatchString(headerLine) {
		return nil, fmt.Errorf("%s is not a valid figlet font", fontName)
	}

	headerClean := reMagicNumber.ReplaceAllString(headerLine, "")
	headerFields := strings.Fields(headerClean)
	if len(headerFields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", fontName)
	}

	hardBlankRunes := []rune(headerFields[0])
	if len(hardBlankRunes) == 0 {
		return nil, fmt.Errorf("missing hardblank in font %s", fontName)
	}
	hardBlank := hardBlankRunes[0]

	height, err := strconv.Atoi(headerFields[1])
	if err != nil {
		return nil, err
	}
	baseLine, err := strconv.Atoi(headerFields[2])
	if err != nil {
		return nil, err
	}
	maxLength, err := strconv.Atoi(headerFields[3])
	if err != nil {
		return nil, err
	}
	oldLayout, err := strconv.Atoi(headerFields[4])
	if err != nil {
		return nil, err
	}
	commentLines, err := strconv.Atoi(headerFields[5])
	if err != nil {
		return nil, err
	}

	printDirection := 0
	hasPrintDir := false
	if len(headerFields) > 6 {
		if val, err := strconv.Atoi(headerFields[6]); err == nil {
			printDirection = val
			hasPrintDir = true
		}
	}

	fullLayout := 0
	hasFullLayout := false
	if len(headerFields) > 7 {
		if val, err := strconv.Atoi(headerFields[7]); err == nil {
			fullLayout = val
			hasFullLayout = true
		}
	}

	if !hasFullLayout {
		if oldLayout == 0 {
			fullLayout = 64
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | 128
		}
	}

	if !hasPrintDir {
		printDirection = 0
	}

	// Skip comments
	if len(lines) < commentLines {
		return nil, fmt.Errorf("not enough comment lines in font %s", fontName)
	}
	lines = lines[commentLines:]

	font := &FigletFont{
		Name:           fontName,
		Height:         height,
		BaseLine:       baseLine,
		MaxLength:      maxLength,
		OldLayout:      oldLayout,
		CommentLines:   commentLines,
		PrintDirection: printDirection,
		SmushMode:      fullLayout,
		HardBlank:      hardBlank,
		Chars:          make(map[rune][]string),
		Width:          make(map[rune]int),
	}

	// Standard ASCII 32 - 126
	for i := 32; i <= 126; i++ {
		width, letter, err := parseChar(&lines, height)
		if err != nil {
			break
		}
		if i == 32 || strings.Join(letter, "") != "" {
			font.Chars[rune(i)] = letter
			font.Width[rune(i)] = width
		}
	}

	// German Umlaute: Ä, Ö, Ü, ä, ö, ü, ß
	germanUmlaute := []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'}
	for _, u := range germanUmlaute {
		if len(lines) < height {
			break
		}
		width, letter, err := parseChar(&lines, height)
		if err != nil {
			break
		}
		if strings.Join(letter, "") != "" {
			font.Chars[u] = letter
			font.Width[u] = width
		}
	}

	// Extended characters with 0x prefix
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		firstToken := parts[0]
		if strings.HasPrefix(strings.ToLower(firstToken), "0x") {
			code64, err := strconv.ParseInt(firstToken[2:], 16, 32)
			if err != nil {
				continue
			}
			width, letter, err := parseChar(&lines, height)
			if err != nil {
				break
			}
			if strings.Join(letter, "") != "" {
				font.Chars[rune(code64)] = letter
				font.Width[rune(code64)] = width
			}
		}
	}

	return font, nil
}
