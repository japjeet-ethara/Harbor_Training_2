#!/bin/bash
set -e

REPO_DIR="/workspace/repo"
cd "$REPO_DIR"

# 1. Prepare fonts directory and copy ONLY fonts-standard fonts (excluding fonts-contrib)
mkdir -p "$REPO_DIR/fonts"
find "$REPO_DIR" -path "*/fonts-standard/*.flf" -exec cp {} "$REPO_DIR/fonts/" \;
if [ -z "$(ls -A "$REPO_DIR/fonts" 2>/dev/null)" ]; then
    find "$REPO_DIR" -path "*/pyfiglet/fonts/*.flf" -exec cp {} "$REPO_DIR/fonts/" \;
fi

# 2. Delete every .py file anywhere in the repository
find "$REPO_DIR" -name "*.py" -type f -delete
find "$REPO_DIR" -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true

# 3. Create go.mod
cat << 'EOF' > "$REPO_DIR/go.mod"
module figlet

go 1.23
EOF

# 4. Create font.go
cat << 'EOF' > "$REPO_DIR/font.go"
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type Font struct {
	Name           string
	Height         int
	BaseLine       int
	MaxLength      int
	OldLayout      int
	CommentLines   int
	PrintDirection int
	SmushMode      int
	HardBlank      string
	HardBlankRune  rune
	Chars          map[rune][]string
	Width          map[rune]int
}

var endMarkerRegex = regexp.MustCompile(`(.)\s*$`)

func LoadFont(name string) (*Font, error) {
	lowerName := strings.ToLower(name)
	clean := name
	if strings.HasSuffix(lowerName, ".flf") || strings.HasSuffix(lowerName, ".tlf") {
		clean = name[:len(name)-4]
	}

	var data []byte
	var err error

	// 1. Direct path check
	for _, p := range []string{name, clean + ".flf", clean + ".tlf"} {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}

	// 2. Candidate directory search
	if err != nil {
		searchDirs := []string{
			"/workspace/repo/fonts",
			"/workspace/repo/pyfiglet/fonts",
			"/workspace/repo",
			"fonts",
			".",
		}
		if exePath, exeErr := os.Executable(); exeErr == nil {
			exeDir := filepath.Dir(exePath)
			searchDirs = append(searchDirs, filepath.Join(exeDir, "fonts"), exeDir)
		}
		for _, dir := range searchDirs {
			for _, ext := range []string{".flf", ".tlf"} {
				p := filepath.Join(dir, clean+ext)
				data, err = os.ReadFile(p)
				if err == nil {
					break
				}
			}
			if err == nil {
				break
			}
		}
	}

	// 3. Fallback: recursive walk inside repository if relocated
	if err != nil {
		origErr := err
		found := false
		_ = filepath.Walk("/workspace/repo", func(p string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info == nil || info.IsDir() {
				return nil
			}
			lower := strings.ToLower(info.Name())
			if strings.HasSuffix(lower, ".flf") || strings.HasSuffix(lower, ".tlf") {
				base := lower[:len(lower)-4]
				if strings.EqualFold(base, clean) {
					var readErr error
					data, readErr = os.ReadFile(p)
					if readErr == nil {
						found = true
						err = nil
						return filepath.SkipAll
					}
				}
			}
			return nil
		})
		if !found {
			err = origErr
		}
	}

	if err != nil {
		return nil, fmt.Errorf("font %q not found: %w", name, err)
	}

	content := string(data)
	content = strings.ReplaceAll(content, "\u0085", " ")
	content = strings.ReplaceAll(content, "\u2028", " ")
	content = strings.ReplaceAll(content, "\u2029", " ")

	lines := splitLines(content)
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font file")
	}

	header := lines[0]
	lines = lines[1:]

	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || header[1:4] != "lf2" {
		return nil, fmt.Errorf("not a figlet font: %s", header)
	}

	header = header[5:]
	fields := strings.Fields(header)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header")
	}

	hardBlank := fields[0]
	height, _ := strconv.Atoi(fields[1])
	baseLine, _ := strconv.Atoi(fields[2])
	maxLength, _ := strconv.Atoi(fields[3])
	oldLayout, _ := strconv.Atoi(fields[4])
	commentLines, _ := strconv.Atoi(fields[5])

	printDir := 0
	if len(fields) > 6 {
		printDir, _ = strconv.Atoi(fields[6])
	}

	fullLayout := -1
	if len(fields) > 7 {
		fullLayout, _ = strconv.Atoi(fields[7])
	}

	if fullLayout == -1 {
		if oldLayout == 0 {
			fullLayout = 64
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | 128
		}
	}

	if len(lines) < commentLines {
		return nil, fmt.Errorf("not enough lines for comments")
	}
	lines = lines[commentLines:]

	font := &Font{
		Name:           name,
		Height:         height,
		BaseLine:       baseLine,
		MaxLength:      maxLength,
		OldLayout:      oldLayout,
		CommentLines:   commentLines,
		PrintDirection: printDir,
		SmushMode:      fullLayout,
		HardBlank:      hardBlank,
		Chars:          make(map[rune][]string),
		Width:          make(map[rune]int),
	}
	if len(hardBlank) > 0 {
		font.HardBlankRune = []rune(hardBlank)[0]
	}

	readChar := func() (int, []string) {
		var endChar rune
		hasEnd := false
		width := 0
		var charLines []string
		for r := 0; r < font.Height; r++ {
			if len(lines) == 0 {
				break
			}
			line := lines[0]
			lines = lines[1:]

			if !hasEnd {
				m := endMarkerRegex.FindStringSubmatch(line)
				if len(m) > 1 {
					endChar = []rune(m[1])[0]
					hasEnd = true
				}
			}

			if hasEnd {
				line = stripEndMarker(line, endChar)
			}
			runes := []rune(line)
			if len(runes) > width {
				width = len(runes)
			}
			charLines = append(charLines, line)
		}
		return width, charLines
	}

	// Standard ASCII (32 - 126)
	for i := 32; i < 127; i++ {
		w, letter := readChar()
		if i == 32 || strings.Join(letter, "") != "" {
			font.Chars[rune(i)] = letter
			font.Width[rune(i)] = w
		}
	}

	// German umlaute: ÄÖÜäöüß
	for _, ch := range "ÄÖÜäöüß" {
		idx := 0
		for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
			idx++
		}
		if idx >= len(lines) {
			break
		}
		first := strings.Fields(strings.TrimSpace(lines[idx]))[0]
		if strings.HasPrefix(strings.ToLower(first), "0x") {
			break
		}
		lines = lines[idx:]
		w, letter := readChar()
		if strings.Join(letter, "") != "" {
			font.Chars[ch] = letter
			font.Width[ch] = w
		}
	}

	// Extended ASCII
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		first := parts[0]
		if strings.HasPrefix(strings.ToLower(first), "0x") {
			code64, err := strconv.ParseInt(first[2:], 16, 64)
			if err == nil {
				w, letter := readChar()
				if strings.Join(letter, "") != "" {
					font.Chars[rune(code64)] = letter
					font.Width[rune(code64)] = w
				}
			}
		}
	}

	return font, nil
}

func stripEndMarker(line string, endChar rune) string {
	runes := []rune(line)
	trailingSpace := 0
	for trailingSpace < len(runes) && unicode.IsSpace(runes[len(runes)-1-trailingSpace]) {
		trailingSpace++
	}
	idx := len(runes) - trailingSpace
	count := 0
	for count < 2 && idx > 0 && runes[idx-1] == endChar {
		idx--
		count++
	}
	if count > 0 {
		return string(runes[:idx])
	}
	return line
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			if i+1 < len(s) && s[i+1] == '\n' {
				lines = append(lines, s[start:i])
				i++
				start = i + 1
			} else {
				lines = append(lines, s[start:i])
				start = i + 1
			}
		} else if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
EOF

# 5. Create builder.go
cat << 'EOF' > "$REPO_DIR/builder.go"
package main

import (
	"strings"
)

const (
	SMEqual     = 1
	SMLowline   = 2
	SMHierarchy = 4
	SMPair      = 8
	SMBigX      = 16
	SMHardblank = 32
	SMKern      = 64
	SMSmush     = 128
)

type BlankMarker struct {
	Buffer   []string
	Iterator int
}

type Builder struct {
	Text          []rune
	Font          *Font
	Direction     string
	Width         int
	Justify       string
	Iterator      int
	MaxSmush      int
	CurCharWidth  int
	PrevCharWidth int
	BlankMarkers  []BlankMarker
	Buffer        []string
	Queue         [][]string
}

func NewBuilder(text string, font *Font, direction string, width int, justify string) *Builder {
	if direction == "auto" {
		if font.PrintDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}
	if justify == "auto" {
		if direction == "left-to-right" {
			justify = "left"
		} else {
			justify = "right"
		}
	}

	buf := make([]string, font.Height)
	for i := range buf {
		buf[i] = ""
	}

	return &Builder{
		Text:      []rune(text),
		Font:      font,
		Direction: direction,
		Width:     width,
		Justify:   justify,
		Buffer:    buf,
	}
}

func (b *Builder) SmushChars(left, right rune) rune {
	if left == ' ' {
		return right
	}
	if right == ' ' {
		return left
	}
	if b.PrevCharWidth < 2 || b.CurCharWidth < 2 {
		return 0
	}
	if (b.Font.SmushMode & SMSmush) == 0 {
		return 0
	}
	if (b.Font.SmushMode & 63) == 0 {
		if left == b.Font.HardBlankRune {
			return right
		}
		if right == b.Font.HardBlankRune {
			return left
		}
		if b.Direction == "right-to-left" {
			return left
		}
		return right
	}
	if (b.Font.SmushMode & SMHardblank) != 0 {
		if left == b.Font.HardBlankRune && right == b.Font.HardBlankRune {
			return left
		}
	}
	if left == b.Font.HardBlankRune || right == b.Font.HardBlankRune {
		return 0
	}
	if (b.Font.SmushMode & SMEqual) != 0 {
		if left == right {
			return left
		}
	}

	var smushes [][2]string
	if (b.Font.SmushMode & SMLowline) != 0 {
		smushes = append(smushes, [2]string{"_", `|/\[]{}()<>`})
	}
	if (b.Font.SmushMode & SMHierarchy) != 0 {
		smushes = append(smushes,
			[2]string{"|", `/\[]{}()<>`},
			[2]string{`\/`, `[]{}()<>`},
			[2]string{"[]", "{}()<>"},
			[2]string{"{}", "()<>"},
			[2]string{"()", "<>"},
		)
	}

	for _, pair := range smushes {
		a, bStr := pair[0], pair[1]
		if strings.ContainsRune(a, left) && strings.ContainsRune(bStr, right) {
			return right
		}
		if strings.ContainsRune(a, right) && strings.ContainsRune(bStr, left) {
			return left
		}
	}

	if (b.Font.SmushMode & SMPair) != 0 {
		if (left == '[' && right == ']') || (left == ']' && right == '[') ||
			(left == '{' && right == '}') || (left == '}' && right == '{') ||
			(left == '(' && right == ')') || (left == ')' && right == '(') {
			return '|'
		}
	}

	if (b.Font.SmushMode & SMBigX) != 0 {
		if left == '/' && right == '\\' {
			return '|'
		}
		if right == '/' && left == '\\' {
			return 'Y'
		}
		if left == '>' && right == '<' {
			return 'X'
		}
	}
	return 0
}

func (b *Builder) SmushAmount(curChar []string) int {
	if (b.Font.SmushMode & (SMSmush | SMKern)) == 0 {
		return 0
	}
	maxSmush := b.CurCharWidth
	for row := 0; row < b.Font.Height; row++ {
		lineLeft := b.Buffer[row]
		lineRight := curChar[row]
		if b.Direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		lineLeftRunes := []rune(lineLeft)
		lineRightRunes := []rune(lineRight)

		rTrimLen := len(lineLeftRunes)
		for rTrimLen > 0 && lineLeftRunes[rTrimLen-1] == ' ' {
			rTrimLen--
		}
		linebd := rTrimLen - 1
		if linebd < 0 {
			linebd = 0
		}
		var ch1 rune
		if linebd < len(lineLeftRunes) {
			ch1 = lineLeftRunes[linebd]
		} else {
			linebd = 0
			ch1 = 0
		}

		charbd := 0
		for charbd < len(lineRightRunes) && lineRightRunes[charbd] == ' ' {
			charbd++
		}
		var ch2 rune
		if charbd < len(lineRightRunes) {
			ch2 = lineRightRunes[charbd]
		} else {
			charbd = len(lineRightRunes)
			ch2 = 0
		}

		amt := charbd + len(lineLeftRunes) - 1 - linebd
		if ch1 == 0 || ch1 == ' ' {
			amt++
		} else if ch2 != 0 && b.SmushChars(ch1, ch2) != 0 {
			amt++
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

func (b *Builder) SmushRow(curChar []string, row int) (string, string) {
	addLeft := b.Buffer[row]
	addRight := curChar[row]
	if b.Direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	leftRunes := []rune(addLeft)
	rightRunes := []rune(addRight)

	for i := 0; i < b.MaxSmush; i++ {
		idx := len(leftRunes) - b.MaxSmush + i
		var left rune
		if idx >= 0 && idx < len(leftRunes) {
			left = leftRunes[idx]
		}
		var right rune
		if i >= 0 && i < len(rightRunes) {
			right = rightRunes[i]
		}
		smushed := b.SmushChars(left, right)
		if smushed != 0 && idx >= 0 && idx < len(leftRunes) {
			leftRunes[idx] = smushed
		}
	}
	return string(leftRunes), string(rightRunes)
}

func (b *Builder) CutBufferCommon() {
	b.Buffer = make([]string, b.Font.Height)
	for i := range b.Buffer {
		b.Buffer[i] = ""
	}
	b.BlankMarkers = nil
	b.PrevCharWidth = 0
	curChar := b.GetCurChar()
	if curChar != nil {
		b.MaxSmush = b.SmushAmount(curChar)
	}
}

func (b *Builder) HandleNewLine() {
	if len(b.BlankMarkers) > 0 {
		saved := b.BlankMarkers[len(b.BlankMarkers)-1]
		b.BlankMarkers = b.BlankMarkers[:len(b.BlankMarkers)-1]
		b.Queue = append(b.Queue, saved.Buffer)
		b.Iterator = saved.Iterator
		b.CutBufferCommon()
	} else {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.Queue = append(b.Queue, bufCopy)
		b.Iterator--
		b.CutBufferCommon()
	}
}

func (b *Builder) GetCurChar() []string {
	if b.Iterator < 0 || b.Iterator >= len(b.Text) {
		return nil
	}
	c := b.Text[b.Iterator]
	return b.Font.Chars[c]
}

func (b *Builder) Render() string {
	for b.Iterator < len(b.Text) {
		ch := b.Text[b.Iterator]
		if ch == '\n' {
			bufCopy := make([]string, len(b.Buffer))
			copy(bufCopy, b.Buffer)
			b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: bufCopy, Iterator: b.Iterator})
			b.HandleNewLine()
			b.Iterator++
			continue
		}

		curChar := b.GetCurChar()
		if curChar == nil {
			b.Iterator++
			continue
		}

		b.CurCharWidth = b.Font.Width[ch]
		b.MaxSmush = b.SmushAmount(curChar)
		currentTotalWidth := len([]rune(b.Buffer[0])) + b.CurCharWidth - b.MaxSmush

		if ch == ' ' {
			bufCopy := make([]string, len(b.Buffer))
			copy(bufCopy, b.Buffer)
			b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: bufCopy, Iterator: b.Iterator})
		}

		if currentTotalWidth >= b.Width {
			b.HandleNewLine()
		} else {
			for row := 0; row < b.Font.Height; row++ {
				al, ar := b.SmushRow(curChar, row)
				arRunes := []rune(ar)
				var arSub string
				if b.MaxSmush < len(arRunes) {
					arSub = string(arRunes[b.MaxSmush:])
				}
				b.Buffer[row] = al + arSub
			}
		}
		b.PrevCharWidth = b.CurCharWidth
		b.Iterator++
	}

	if b.Buffer[0] != "" {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.Queue = append(b.Queue, bufCopy)
	}

	var sb strings.Builder
	for _, buf := range b.Queue {
		rowStrs := make([]string, len(buf))
		copy(rowStrs, buf)

		if b.Justify == "right" {
			for r := 0; r < b.Font.Height; r++ {
				pad := b.Width - len([]rune(rowStrs[r])) - 1
				if pad < 0 {
					pad = 0
				}
				rowStrs[r] = strings.Repeat(" ", pad) + rowStrs[r]
			}
		} else if b.Justify == "center" {
			for r := 0; r < b.Font.Height; r++ {
				pad := (b.Width - len([]rune(rowStrs[r]))) / 2
				if pad < 0 {
					pad = 0
				}
				rowStrs[r] = strings.Repeat(" ", pad) + rowStrs[r]
			}
		}

		joined := strings.Join(rowStrs, "\n") + "\n"
		joined = strings.ReplaceAll(joined, b.Font.HardBlank, " ")
		sb.WriteString(joined)
	}

	return sb.String()
}
EOF

# 6. Create post.go
cat << 'EOF' > "$REPO_DIR/post.go"
package main

import (
	"strings"
)

var reverseMap = [256]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
	' ', '!', '"', '#', '$', '%', '&', '\'', ')', '(', '*', '+', ',', '-', '.', '\\',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ':', ';', '>', '=', '<', '?',
	'@', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O',
	'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', ']', '/', '[', '^', '_',
	'`', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', '}', '|', '{', '~', 0x7f,
	0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f,
	0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0x9b, 0x9c, 0x9d, 0x9e, 0x9f,
	0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf,
	0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xbb, 0xbc, 0xbd, 0xbe, 0xbf,
	0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf,
	0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf,
	0xe0, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef,
	0xf0, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xfb, 0xfc, 0xfd, 0xfe, 0xff,
}

var flipMap = [256]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
	' ', '!', '"', '#', '$', '%', '&', '\'', '(', ')', '*', '+', ',', '-', '.', '\\',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ':', ';', '<', '=', '>', '?',
	'@', 'V', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'W', 'N', 'O',
	'b', 'Q', 'b', 'S', 'T', 'U', 'A', 'M', 'X', 'Y', 'Z', '[', '/', ']', 'v', '-',
	'`', 'a', 'P', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'w', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', '^', 'm', 'x', 'y', 'z', '{', '|', '}', '~', 0x7f,
	0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f,
	0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0x9b, 0x9c, 0x9d, 0x9e, 0x9f,
	0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf,
	0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xbb, 0xbc, 0xbd, 0xbe, 0xbf,
	0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf,
	0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf,
	0xe0, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef,
	0xf0, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xfb, 0xfc, 0xfd, 0xfe, 0xff,
}

func translateRune(ch rune, table *[256]byte) rune {
	if ch >= 0 && ch < 256 {
		return rune(table[ch])
	}
	return ch
}

func PostProcess(s string, reverse, flip, stripNL, normalizeNL bool) string {
	lines := splitLines(s)

	if reverse {
		revLines := make([]string, len(lines))
		for i, line := range lines {
			runes := []rune(line)
			n := len(runes)
			res := make([]rune, n)
			for j := 0; j < n; j++ {
				res[j] = translateRune(runes[n-1-j], &reverseMap)
			}
			revLines[i] = string(res)
		}
		s = strings.Join(revLines, "\n") + "\n"
		lines = splitLines(s)
	}

	if flip {
		flipLines := make([]string, len(lines))
		n := len(lines)
		for i := 0; i < n; i++ {
			line := lines[n-1-i]
			runes := []rune(line)
			res := make([]rune, len(runes))
			for j, r := range runes {
				res[j] = translateRune(r, &flipMap)
			}
			flipLines[i] = string(res)
		}
		s = strings.Join(flipLines, "\n") + "\n"
		lines = splitLines(s)
	}

	if stripNL {
		var out []string
		seen := false
		for _, line := range lines {
			if strings.TrimSpace(line) != "" || seen {
				seen = true
				out = append(out, line)
			}
		}
		joined := strings.Join(out, "\n") + "\n"
		s = strings.TrimRight(joined, " \t\r\n\v\f")
	} else if normalizeNL {
		var out []string
		seen := false
		for _, line := range lines {
			if strings.TrimSpace(line) != "" || seen {
				seen = true
				out = append(out, line)
			}
		}
		joined := strings.Join(out, "\n") + "\n"
		s = "\n" + strings.TrimRight(joined, " \t\r\n\v\f") + "\n"
	}

	return s
}
EOF

# 7. Create main.go
cat << 'EOF' > "$REPO_DIR/main.go"
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func isBoolFlags(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, ch := range s {
		if ch != 'r' && ch != 'F' && ch != 's' && ch != 'n' {
			return false
		}
	}
	return true
}

func main() {
	fontName := "standard"
	width := 80
	justify := "auto"
	direction := "auto"
	reverse := false
	flip := false
	stripNL := false
	normalizeNL := false

	var textArgs []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--font":
			if i+1 < len(args) {
				fontName = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-f="):
			fontName = strings.TrimPrefix(arg, "-f=")
		case strings.HasPrefix(arg, "--font="):
			fontName = strings.TrimPrefix(arg, "--font=")
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			fontName = arg[2:]
		case arg == "-w" || arg == "--width":
			if i+1 < len(args) {
				if w, err := strconv.Atoi(args[i+1]); err == nil {
					width = w
				}
				i++
			}
		case strings.HasPrefix(arg, "-w="):
			if w, err := strconv.Atoi(strings.TrimPrefix(arg, "-w=")); err == nil {
				width = w
			}
		case strings.HasPrefix(arg, "--width="):
			if w, err := strconv.Atoi(strings.TrimPrefix(arg, "--width=")); err == nil {
				width = w
			}
		case strings.HasPrefix(arg, "-w") && len(arg) > 2:
			if w, err := strconv.Atoi(arg[2:]); err == nil {
				width = w
			}
		case arg == "-j" || arg == "--justify":
			if i+1 < len(args) {
				justify = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-j="):
			justify = strings.TrimPrefix(arg, "-j=")
		case strings.HasPrefix(arg, "--justify="):
			justify = strings.TrimPrefix(arg, "--justify=")
		case strings.HasPrefix(arg, "-j") && len(arg) > 2:
			justify = arg[2:]
		case arg == "-D" || arg == "--direction":
			if i+1 < len(args) {
				direction = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-D="):
			direction = strings.TrimPrefix(arg, "-D=")
		case strings.HasPrefix(arg, "--direction="):
			direction = strings.TrimPrefix(arg, "--direction=")
		case strings.HasPrefix(arg, "-D") && len(arg) > 2:
			direction = arg[2:]
		case arg == "-r" || arg == "--reverse":
			reverse = true
		case arg == "-F" || arg == "--flip":
			flip = true
		case arg == "-s" || arg == "--strip-surrounding-newlines":
			stripNL = true
		case arg == "-n" || arg == "--normalize-surrounding-newlines":
			normalizeNL = true
		case len(arg) > 1 && arg[0] == '-' && arg[1] != '-' && isBoolFlags(arg[1:]):
			for _, ch := range arg[1:] {
				switch ch {
				case 'r':
					reverse = true
				case 'F':
					flip = true
				case 's':
					stripNL = true
				case 'n':
					normalizeNL = true
				}
			}
		case arg == "--":
			textArgs = append(textArgs, args[i+1:]...)
			i = len(args)
		default:
			textArgs = append(textArgs, arg)
		}
	}

	if len(textArgs) == 0 {
		os.Exit(1)
	}

	text := strings.Join(textArgs, " ")

	font, err := LoadFont(fontName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	builder := NewBuilder(text, font, direction, width, justify)
	rendered := builder.Render()

	// -s takes priority over -n
	if stripNL {
		normalizeNL = false
	}

	final := PostProcess(rendered, reverse, flip, stripNL, normalizeNL)
	os.Stdout.WriteString(final)
	os.Stdout.WriteString("\n")
}
EOF

# 8. Build and verify
go build -o /tmp/figlet .
go build ./...
echo "FIGlet Go implementation successfully created and built!"
