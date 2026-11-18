#!/bin/bash
set -euo pipefail

# figlet-go Oracle Solution
# Writes a complete Go port of pyfiglet to /workspace/repo,
# removes all .py files, and builds the binary to /tmp/figlet.

mkdir -p /workspace/repo
cd /workspace/repo

cat << 'EOF' > /workspace/repo/go.mod
module figlet

go 1.23
EOF

cat << 'EOF' > /workspace/repo/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

type Font struct {
	Height         int
	Baseline       int
	MaxLength      int
	OldLayout      int
	CommentLines   int
	PrintDirection int
	SmushMode      int
	Hardblank      byte
	Chars          map[rune][]string
	Widths         map[rune]int
}

func loadFont(name string) (*Font, error) {
	possiblePaths := []string{
		filepath.Join(".", "pyfiglet", "fonts", name+".flf"),
		filepath.Join("/workspace/repo", "pyfiglet", "fonts", name+".flf"),
		filepath.Join(".", name+".flf"),
		filepath.Join("/usr/share/figlet", name+".flf"),
	}

	var filePath string
	for _, p := range possiblePaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			filePath = p
			break
		}
	}
	if filePath == "" {
		return nil, fmt.Errorf("font %s not found", name)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Remove unicode line splitters to match pyfiglet
	text := string(content)
	reLineSplitters := regexp.MustCompile(`[\x{0085}\x{2028}\x{2029}]`)
	text = reLineSplitters.ReplaceAllString(text, " ")

	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font file")
	}

	headerLine := lines[0]
	lines = lines[1:]

	if !strings.HasPrefix(headerLine, "flf2a") {
		return nil, fmt.Errorf("invalid magic number in header")
	}

	hardblank := headerLine[5]
	headerRest := strings.TrimSpace(headerLine[6:])
	headerFields := strings.Fields(headerRest)
	if len(headerFields) < 5 {
		return nil, fmt.Errorf("malformed header")
	}

	height, _ := strconv.Atoi(headerFields[0])
	baseline, _ := strconv.Atoi(headerFields[1])
	maxLength, _ := strconv.Atoi(headerFields[2])
	oldLayout, _ := strconv.Atoi(headerFields[3])
	commentLines, _ := strconv.Atoi(headerFields[4])

	printDir := 0
	fullLayout := -1
	if len(headerFields) > 5 {
		printDir, _ = strconv.Atoi(headerFields[5])
	}
	if len(headerFields) > 6 {
		fullLayout, _ = strconv.Atoi(headerFields[6])
	}

	smushMode := fullLayout
	if smushMode == -1 {
		if oldLayout == 0 {
			smushMode = 64
		} else if oldLayout < 0 {
			smushMode = 0
		} else {
			smushMode = (oldLayout & 31) | 128
		}
	}

	if len(lines) < commentLines {
		return nil, fmt.Errorf("not enough lines for comments")
	}
	lines = lines[commentLines:]

	font := &Font{
		Height:         height,
		Baseline:       baseline,
		MaxLength:      maxLength,
		OldLayout:      oldLayout,
		CommentLines:   commentLines,
		PrintDirection: printDir,
		SmushMode:      smushMode,
		Hardblank:      hardblank,
		Chars:          make(map[rune][]string),
		Widths:         make(map[rune]int),
	}

	parseChar := func() (int, []string, bool) {
		if len(lines) < height {
			return 0, nil, false
		}
		var charLines []string
		width := 0
		var endMarker string
		for r := 0; r < height; r++ {
			raw := lines[0]
			lines = lines[1:]
			if r == 0 {
				trimmed := strings.TrimRight(raw, " \t\r")
				if len(trimmed) > 0 {
					endMarker = string(trimmed[len(trimmed)-1])
				}
			}
			reEnd := regexp.MustCompile(regexp.QuoteMeta(endMarker) + `{1,2}\s*$`)
			cleaned := reEnd.ReplaceAllString(raw, "")
			if len(cleaned) > width {
				width = len(cleaned)
			}
			charLines = append(charLines, cleaned)
		}
		return width, charLines, true
	}

	// ASCII 32 to 126
	for c := 32; c < 127; c++ {
		w, chLines, ok := parseChar()
		if !ok {
			break
		}
		if c == 32 || strings.Join(chLines, "") != "" {
			font.Chars[rune(c)] = chLines
			font.Widths[rune(c)] = w
		}
	}

	// German characters
	german := []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'}
	for _, gr := range german {
		w, chLines, ok := parseChar()
		if !ok {
			break
		}
		if strings.Join(chLines, "") != "" {
			font.Chars[gr] = chLines
			font.Widths[gr] = w
		}
	}

	// Extended characters (only 0x hex entries, matching pyfiglet)
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		numStr := parts[0]
		if strings.HasPrefix(strings.ToLower(numStr), "0x") {
			code, parseErr := strconv.ParseInt(numStr[2:], 16, 64)
			if parseErr != nil {
				continue
			}
			w, chLines, ok := parseChar()
			if !ok {
				break
			}
			if strings.Join(chLines, "") != "" {
				font.Chars[rune(code)] = chLines
				font.Widths[rune(code)] = w
			}
		}
	}

	return font, nil
}

type BlankMarker struct {
	Buffer   []string
	Iterator int
}

type FigletBuilder struct {
	Text          []rune
	Direction     string
	Width         int
	Font          *Font
	Justify       string
	Iterator      int
	MaxSmush      int
	CurCharWidth  int
	PrevCharWidth int
	BlankMarkers  []BlankMarker
	Product       [][]string
	Buffer        []string
}

func NewFigletBuilder(text string, font *Font, direction string, width int, justify string) *FigletBuilder {
	return &FigletBuilder{
		Text:      []rune(text),
		Direction: direction,
		Width:     width,
		Font:      font,
		Justify:   justify,
		Buffer:    make([]string, font.Height),
	}
}

func (b *FigletBuilder) smushChars(left, right byte) (byte, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.PrevCharWidth < 2 || b.CurCharWidth < 2 {
		return 0, false
	}
	if (b.Font.SmushMode & SMSmush) == 0 {
		return 0, false
	}
	// Universal smushing
	if (b.Font.SmushMode & 63) == 0 {
		if left == b.Font.Hardblank {
			return right, true
		}
		if right == b.Font.Hardblank {
			return left, true
		}
		if b.Direction == "right-to-left" {
			return left, true
		}
		return right, true
	}

	if (b.Font.SmushMode & SMHardblank) != 0 {
		if left == b.Font.Hardblank && right == b.Font.Hardblank {
			return left, true
		}
	}

	if left == b.Font.Hardblank || right == b.Font.Hardblank {
		return 0, false
	}

	if (b.Font.SmushMode & SMEqual) != 0 {
		if left == right {
			return left, true
		}
	}

	if (b.Font.SmushMode & SMLowline) != 0 {
		if left == '_' && strings.ContainsRune("|/\\[]{}()<>", rune(right)) {
			return right, true
		}
		if right == '_' && strings.ContainsRune("|/\\[]{}()<>", rune(left)) {
			return left, true
		}
	}

	if (b.Font.SmushMode & SMHierarchy) != 0 {
		classes := map[byte]int{
			'|': 1, '/': 2, '\\': 2, '[': 3, ']': 3, '{': 4, '}': 4, '(': 5, ')': 5, '<': 6, '>': 6,
		}
		cl1 := classes[left]
		cl2 := classes[right]
		if cl1 > 0 && cl2 > 0 && cl1 != cl2 {
			if cl1 > cl2 {
				return left, true
			}
			return right, true
		}
	}

	if (b.Font.SmushMode & SMPair) != 0 {
		pair := string([]byte{left, right})
		rpair := string([]byte{right, left})
		if pair == "[]" || pair == "{}" || pair == "()" ||
			rpair == "[]" || rpair == "{}" || rpair == "()" {
			return '|', true
		}
	}

	if (b.Font.SmushMode & SMBigX) != 0 {
		if left == '/' && right == '\\' {
			return '|', true
		}
		if left == '\\' && right == '/' {
			return 'Y', true
		}
		if left == '>' && right == '<' {
			return 'X', true
		}
	}

	return 0, false
}

func (b *FigletBuilder) currentSmushAmount(curChar []string) int {
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

		linebd := len(strings.TrimRight(lineLeft, " ")) - 1
		if linebd < 0 {
			linebd = 0
		}
		var ch1 byte
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		}

		charbd := len(lineRight) - len(strings.TrimLeft(lineRight, " "))
		var ch2 byte
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		}

		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == 0 || ch1 == ' ' {
			amt++
		} else if ch2 != 0 {
			if _, ok := b.smushChars(ch1, ch2); ok {
				amt++
			}
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

func (b *FigletBuilder) smushRow(curChar []string, row int) (string, string) {
	addLeft := b.Buffer[row]
	addRight := curChar[row]
	if b.Direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	leftBytes := []byte(addLeft)
	for i := 0; i < b.MaxSmush; i++ {
		idx := len(leftBytes) - b.MaxSmush + i
		var left byte
		if idx >= 0 && idx < len(leftBytes) {
			left = leftBytes[idx]
		}
		right := addRight[i]
		if smushed, ok := b.smushChars(left, right); ok && idx >= 0 && idx < len(leftBytes) {
			leftBytes[idx] = smushed
		}
	}
	return string(leftBytes), addRight
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	b.Buffer[row] = addLeft + addRight[b.MaxSmush:]
}

func (b *FigletBuilder) cutBufferCommon() {
	b.Buffer = make([]string, b.Font.Height)
	b.BlankMarkers = nil
	b.PrevCharWidth = 0
	if b.Iterator < len(b.Text) {
		r := b.Text[b.Iterator]
		if ch, ok := b.Font.Chars[r]; ok {
			b.MaxSmush = b.currentSmushAmount(ch)
		}
	}
}

func (b *FigletBuilder) handleNewLine() {
	if len(b.BlankMarkers) > 0 {
		last := b.BlankMarkers[len(b.BlankMarkers)-1]
		b.BlankMarkers = b.BlankMarkers[:len(b.BlankMarkers)-1]
		b.Product = append(b.Product, last.Buffer)
		b.Iterator = last.Iterator
		b.cutBufferCommon()
	} else {
		b.Product = append(b.Product, b.Buffer)
		b.Iterator--
		b.cutBufferCommon()
	}
}

func (b *FigletBuilder) AddCharToProduct() {
	if b.Iterator >= len(b.Text) {
		return
	}
	r := b.Text[b.Iterator]
	if r == '\n' {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: bufCopy, Iterator: b.Iterator})
		b.handleNewLine()
		return
	}

	curChar, ok := b.Font.Chars[r]
	if !ok {
		return
	}
	curWidth := b.Font.Widths[r]
	b.CurCharWidth = curWidth
	b.MaxSmush = b.currentSmushAmount(curChar)

	currentTotalWidth := len(b.Buffer[0]) + b.CurCharWidth - b.MaxSmush

	if r == ' ' {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: bufCopy, Iterator: b.Iterator})
	}

	if currentTotalWidth >= b.Width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.Font.Height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}
	b.PrevCharWidth = b.CurCharWidth
}

func (b *FigletBuilder) Render() string {
	for b.Iterator < len(b.Text) {
		b.AddCharToProduct()
		b.Iterator++
	}
	if len(b.Buffer) > 0 && b.Buffer[0] != "" {
		b.Product = append(b.Product, b.Buffer)
	}

	var sb strings.Builder
	for _, buf := range b.Product {
		bufCopy := make([]string, len(buf))
		copy(bufCopy, buf)

		// Justification
		if b.Justify == "right" {
			for row := 0; row < b.Font.Height; row++ {
				pad := b.Width - len(bufCopy[row]) - 1
				if pad > 0 {
					bufCopy[row] = strings.Repeat(" ", pad) + bufCopy[row]
				}
			}
		} else if b.Justify == "center" {
			for row := 0; row < b.Font.Height; row++ {
				pad := (b.Width - len(bufCopy[row])) / 2
				if pad > 0 {
					bufCopy[row] = strings.Repeat(" ", pad) + bufCopy[row]
				}
			}
		}

		lineStr := strings.Join(bufCopy, "\n") + "\n"
		lineStr = strings.ReplaceAll(lineStr, string(b.Font.Hardblank), " ")
		sb.WriteString(lineStr)
	}
	return sb.String()
}

func transformReverse(s string) string {
	revMap := map[byte]byte{
		'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<',
		'[': ']', ']': '[', '{': '}', '}': '{',
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out []string
	for _, line := range lines {
		runes := []rune(line)
		n := len(runes)
		reversed := make([]rune, n)
		for i := 0; i < n; i++ {
			b := byte(runes[n-1-i])
			if mapped, ok := revMap[b]; ok {
				reversed[i] = rune(mapped)
			} else {
				reversed[i] = runes[n-1-i]
			}
		}
		out = append(out, string(reversed))
	}
	return strings.Join(out, "\n") + "\n"
}

func transformFlip(s string) string {
	flipMap := map[byte]byte{
		'/': '\\', '\\': '/', 'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M',
		'P': 'b', 'R': 'b', 'b': 'P', '^': 'v', 'v': '^', '_': '-',
		'm': 'w', 'w': 'm',
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		runes := []rune(lines[i])
		flipped := make([]rune, len(runes))
		for j, r := range runes {
			b := byte(r)
			if mapped, ok := flipMap[b]; ok {
				flipped[j] = rune(mapped)
			} else {
				flipped[j] = r
			}
		}
		out = append(out, string(flipped))
	}
	return strings.Join(out, "\n") + "\n"
}

func transformStrip(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out []string
	seen := false
	for _, line := range lines {
		if strings.TrimSpace(line) != "" || seen {
			seen = true
			out = append(out, line)
		}
	}
	res := strings.Join(out, "\n") + "\n"
	return strings.TrimRight(res, " \t\r\n")
}

func transformNormalize(s string) string {
	return "\n" + transformStrip(s) + "\n"
}

func main() {
	fontName := flag.String("f", "standard", "font name")
	direction := flag.String("D", "auto", "direction")
	justify := flag.String("j", "auto", "justify")
	width := flag.Int("w", 80, "terminal width")
	reverse := flag.Bool("r", false, "reverse output")
	flip := flag.Bool("F", false, "flip output")
	strip := flag.Bool("s", false, "strip surrounding newlines")
	normalize := flag.Bool("n", false, "normalize surrounding newlines")

	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		return
	}
	text := strings.Join(args, " ")

	font, err := loadFont(*fontName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: requested font %q not found.\n", *fontName)
		os.Exit(1)
	}

	actualDirection := *direction
	if actualDirection == "auto" {
		if font.PrintDirection == 1 {
			actualDirection = "right-to-left"
		} else {
			actualDirection = "left-to-right"
		}
	}

	actualJustify := *justify
	if actualJustify == "auto" {
		if actualDirection == "right-to-left" {
			actualJustify = "right"
		} else {
			actualJustify = "left"
		}
	}

	builder := NewFigletBuilder(text, font, actualDirection, *width, actualJustify)
	rendered := builder.Render()

	if *reverse {
		rendered = transformReverse(rendered)
	}
	if *flip {
		rendered = transformFlip(rendered)
	}
	if *strip {
		rendered = transformStrip(rendered)
	} else if *normalize {
		rendered = transformNormalize(rendered)
	}

	fmt.Print(rendered + "\n")
}
EOF

# Clean up all .py files in repo
find /workspace/repo -name "*.py" -delete

# Build binary
cd /workspace/repo
go build -o /tmp/figlet .
