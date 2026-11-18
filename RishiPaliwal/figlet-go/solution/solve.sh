#!/bin/bash
set -euo pipefail

cd /workspace/repo
mkdir -p fonts
cp pyfiglet/fonts-standard/*.flf fonts/

cat > go.mod <<'EOF'
module figlet

go 1.23
EOF

cat > main.go <<'EOF'
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
)

//go:embed fonts/*.flf
var fontFiles embed.FS

const (
	smEqual     = 1
	smLowline   = 2
	smHierarchy = 4
	smPair      = 8
	smBigX      = 16
	smHardblank = 32
	smKern      = 64
	smSmush     = 128
)

type figFont struct {
	hardblank       rune
	height          int
	printDirection int
	smushMode       int
	chars           map[rune][]string
	widths          map[rune]int
}

type blankMarker struct {
	rows  []string
	index int
}

type builder struct {
	text             []rune
	direction        string
	justify          string
	width            int
	font             *figFont
	iterator         int
	maxSmush         int
	curCharWidth     int
	prevCharWidth    int
	currentTotalWidth int
	blankMarkers     []blankMarker
	product          [][]string
	buffer           []string
}

func parseInt(value, field string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s in font header: %w", field, err)
	}
	return n, nil
}

func loadFont(name string) (*figFont, error) {
	name = strings.TrimSuffix(name, ".flf")
	data, err := fontFiles.ReadFile("fonts/" + name + ".flf")
	if err != nil {
		return nil, fmt.Errorf("font %q not found", name)
	}

	content := strings.ReplaceAll(string(data), "\u0085", " ")
	content = strings.ReplaceAll(content, "\u2028", " ")
	content = strings.ReplaceAll(content, "\u2029", " ")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = strings.TrimSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || !(strings.HasPrefix(lines[0], "flf2") || strings.HasPrefix(lines[0], "tlf2")) {
		return nil, fmt.Errorf("%s is not a valid FIGlet font", name)
	}
	if len(lines[0]) < 5 {
		return nil, fmt.Errorf("malformed font header for %s", name)
	}

	fields := strings.Fields(lines[0][5:])
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed font header for %s", name)
	}
	hardblankRunes := []rune(fields[0])
	if len(hardblankRunes) == 0 {
		return nil, fmt.Errorf("missing hardblank in %s", name)
	}
	height, err := parseInt(fields[1], "height")
	if err != nil {
		return nil, err
	}
	_, err = parseInt(fields[2], "baseline")
	if err != nil {
		return nil, err
	}
	_, err = parseInt(fields[3], "max length")
	if err != nil {
		return nil, err
	}
	oldLayout, err := parseInt(fields[4], "old layout")
	if err != nil {
		return nil, err
	}
	commentLines, err := parseInt(fields[5], "comment line count")
	if err != nil || commentLines < 0 {
		if err == nil {
			err = errors.New("negative comment line count")
		}
		return nil, err
	}

	printDirection := -1
	if len(fields) > 6 {
		printDirection, err = parseInt(fields[6], "print direction")
		if err != nil {
			return nil, err
		}
	}
	smushMode := 0
	if len(fields) > 7 {
		smushMode, err = parseInt(fields[7], "full layout")
		if err != nil {
			return nil, err
		}
	} else if oldLayout == 0 {
		smushMode = smKern
	} else if oldLayout < 0 {
		smushMode = 0
	} else {
		smushMode = oldLayout&31 | smSmush
	}

	rows := lines[1:]
	if commentLines > len(rows) {
		return nil, fmt.Errorf("malformed comments in %s", name)
	}
	rows = rows[commentLines:]

	font := &figFont{
		hardblank:       hardblankRunes[0],
		height:          height,
		printDirection: printDirection,
		smushMode:       smushMode,
		chars:           make(map[rune][]string),
		widths:          make(map[rune]int),
	}
	position := 0
	readGlyph := func() ([]string, int, error) {
		if position >= len(rows) {
			return nil, 0, errors.New("unexpected end of font data")
		}
		marker, ok := lastNonSpaceRune(rows[position])
		if !ok {
			return nil, 0, errors.New("missing glyph end marker")
		}
		glyph := make([]string, 0, height)
		maxWidth := 0
		for row := 0; row < height; row++ {
			if position >= len(rows) {
				return nil, 0, errors.New("unexpected end of glyph")
			}
			line := removeEndMarker(rows[position], marker)
			position++
			if width := runeLen(line); width > maxWidth {
				maxWidth = width
			}
			glyph = append(glyph, line)
		}
		return glyph, maxWidth, nil
	}

	for code := rune(32); code <= 126; code++ {
		glyph, width, err := readGlyph()
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s: %w", name, err)
		}
		if code == 32 || !glyphIsEmpty(glyph) {
			font.chars[code] = glyph
			font.widths[code] = width
		}
	}
	if position < len(rows) {
		for _, code := range []rune("ÄÖÜäöüß") {
			glyph, width, err := readGlyph()
			if err != nil {
				return nil, fmt.Errorf("problem parsing %s: %w", name, err)
			}
			if !glyphIsEmpty(glyph) {
				font.chars[code] = glyph
				font.widths[code] = width
			}
		}
	}
	for position < len(rows) {
		line := strings.TrimSpace(rows[position])
		position++
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(strings.ToLower(fields[0]), "0x") {
			continue
		}
		value, err := strconv.ParseInt(fields[0][2:], 16, 32)
		if err != nil {
			continue
		}
		glyph, width, err := readGlyph()
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s: %w", name, err)
		}
		if !glyphIsEmpty(glyph) {
			font.chars[rune(value)] = glyph
			font.widths[rune(value)] = width
		}
	}
	return font, nil
}

func lastNonSpaceRune(line string) (rune, bool) {
	runes := []rune(line)
	for i := len(runes) - 1; i >= 0; i-- {
		if !unicode.IsSpace(runes[i]) {
			return runes[i], true
		}
	}
	return 0, false
}

func removeEndMarker(line string, marker rune) string {
	runes := []rune(line)
	end := len(runes)
	for end > 0 && unicode.IsSpace(runes[end-1]) {
		end--
	}
	removed := 0
	for end > 0 && removed < 2 && runes[end-1] == marker {
		end--
		removed++
	}
	if removed == 0 {
		return line
	}
	return string(runes[:end])
}

func runeLen(value string) int {
	return len([]rune(value))
}

func glyphIsEmpty(glyph []string) bool {
	for _, row := range glyph {
		if row != "" {
			return false
		}
	}
	return true
}

func newBuilder(text string, font *figFont, direction, justify string, width int) *builder {
	if direction == "auto" {
		if font.printDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}
	if justify == "auto" {
		if direction == "right-to-left" {
			justify = "right"
		} else {
			justify = "left"
		}
	}
	return &builder{
		text:      []rune(text),
		direction: direction,
		justify:   justify,
		width:     width,
		font:      font,
		buffer:    make([]string, font.height),
	}
}

func cloneRows(rows []string) []string {
	return append([]string(nil), rows...)
}

func (b *builder) currentChar() (rune, []string, int, bool) {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return 0, nil, 0, false
	}
	code := b.text[b.iterator]
	glyph, ok := b.font.chars[code]
	if !ok {
		return code, nil, 0, false
	}
	return code, glyph, b.font.widths[code], true
}

func (b *builder) addChar() error {
	code := b.text[b.iterator]
	if code == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneRows(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}
	_, glyph, charWidth, ok := b.currentChar()
	if !ok {
		return nil
	}
	if b.width < charWidth {
		return fmt.Errorf("width is not enough to print this character")
	}
	b.curCharWidth = charWidth
	b.maxSmush = b.currentSmushAmount(glyph)
	b.currentTotalWidth = runeLen(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if code == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneRows(b.buffer), b.iterator})
	}
	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addCurrentCharRow(glyph, row)
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) isNotFinished() bool {
	return b.iterator < len(b.text)
}

func (b *builder) returnProduct() string {
	if len(b.buffer) > 0 && b.buffer[0] != "" {
		b.product = append(b.product, b.buffer)
	}
	var result strings.Builder
	for _, rows := range b.product {
		b.justifyRows(rows)
		for _, row := range rows {
			result.WriteString(strings.ReplaceAll(row, string(b.font.hardblank), " "))
			result.WriteByte('\n')
		}
	}
	return result.String()
}

func (b *builder) leftSmushedChar(addLeft []rune, i int) (rune, int) {
	index := len(addLeft) - b.maxSmush + i
	if index >= 0 && index < len(addLeft) {
		return addLeft[index], index
	}
	return 0, index
}

func (b *builder) addCurrentCharRow(glyph []string, row int) {
	left := []rune(b.buffer[row])
	right := []rune(glyph[row])
	if b.direction == "right-to-left" {
		left, right = right, left
	}
	for i := 0; i < b.maxSmush; i++ {
		leftChar, index := b.leftSmushedChar(left, i)
		rightChar := right[i]
		if merged, ok := b.smushChars(leftChar, rightChar); ok && index >= 0 && index < len(left) {
			left[index] = merged
		}
	}
	b.buffer[row] = string(left) + string(right[b.maxSmush:])
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	_, glyph, _, ok := b.currentChar()
	if !ok {
		return
	}
	b.maxSmush = b.currentSmushAmount(glyph)
}

func (b *builder) cutBufferAtLastBlank(saved blankMarker) {
	b.product = append(b.product, saved.rows)
	b.iterator = saved.index
	b.cutBufferCommon()
}

func (b *builder) cutBufferAtLastChar() {
	b.product = append(b.product, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) handleNewLine() {
	if len(b.blankMarkers) > 0 {
		last := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.cutBufferAtLastBlank(last)
		return
	}
	b.cutBufferAtLastChar()
}

func (b *builder) justifyRows(rows []string) {
	for i, row := range rows {
		padding := 0
		switch b.justify {
		case "right":
			padding = b.width - runeLen(row) - 1
		case "center":
			padding = (b.width - runeLen(row)) / 2
		}
		if padding > 0 {
			rows[i] = strings.Repeat(" ", padding) + row
		}
	}
}

func (b *builder) currentSmushAmount(glyph []string) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		left := []rune(b.buffer[row])
		right := []rune(glyph[row])
		if b.direction == "right-to-left" {
			left, right = right, left
		}

		lineBoundary := len(left) - 1
		for lineBoundary >= 0 && left[lineBoundary] == ' ' {
			lineBoundary--
		}
		if lineBoundary < 0 {
			lineBoundary = 0
		}
		var leftChar rune
		if lineBoundary < len(left) {
			leftChar = left[lineBoundary]
		} else {
			lineBoundary = 0
		}

		charBoundary := 0
		for charBoundary < len(right) && right[charBoundary] == ' ' {
			charBoundary++
		}
		var rightChar rune
		if charBoundary < len(right) {
			rightChar = right[charBoundary]
		}

		amount := charBoundary + len(left) - 1 - lineBoundary
		if leftChar == 0 || leftChar == ' ' {
			amount++
		} else if rightChar != 0 {
			if _, ok := b.smushChars(leftChar, rightChar); ok {
				amount++
			}
		}
		if amount < maxSmush {
			maxSmush = amount
		}
	}
	return maxSmush
}

func containsRune(charset string, value rune) bool {
	return strings.ContainsRune(charset, value)
}

func (b *builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}
	if b.font.smushMode&smSmush == 0 {
		return 0, false
	}
	if b.font.smushMode&63 == 0 {
		if left == b.font.hardblank {
			return right, true
		}
		if right == b.font.hardblank {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if b.font.smushMode&smHardblank != 0 && left == b.font.hardblank && right == b.font.hardblank {
		return left, true
	}
	if left == b.font.hardblank || right == b.font.hardblank {
		return 0, false
	}
	if b.font.smushMode&smEqual != 0 && left == right {
		return left, true
	}

	if b.font.smushMode&smLowline != 0 {
		if left == '_' && containsRune("|/\\[]{}()<>", right) {
			return right, true
		}
		if right == '_' && containsRune("|/\\[]{}()<>", left) {
			return left, true
		}
	}
	if b.font.smushMode&smHierarchy != 0 {
		hierarchy := [][2]string{
			{"|", "/\\[]{}()<>"},
			{"/\\", "[]{}()<>"},
			{"[]", "{}()<>"},
			{"{}", "()<>"},
			{"()", "<>"},
		}
		for _, rule := range hierarchy {
			if containsRune(rule[0], left) && containsRune(rule[1], right) {
				return right, true
			}
			if containsRune(rule[0], right) && containsRune(rule[1], left) {
				return left, true
			}
		}
	}
	if b.font.smushMode&smPair != 0 {
		if (left == '[' && right == ']') || (left == ']' && right == '[') ||
			(left == '{' && right == '}') || (left == '}' && right == '{') ||
			(left == '(' && right == ')') || (left == ')' && right == '(') {
			return '|', true
		}
	}
	if b.font.smushMode&smBigX != 0 {
		if left == '/' && right == '\\' {
			return '|', true
		}
		if right == '/' && left == '\\' {
			return 'Y', true
		}
		if left == '>' && right == '<' {
			return 'X', true
		}
	}
	return 0, false
}

func splitLines(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(value, "\n"), "\n")
}

func translateRows(rows []string, mapping map[rune]rune, reverseColumns bool) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		runes := []rune(row)
		for i, char := range runes {
			if replacement, ok := mapping[char]; ok {
				runes[i] = replacement
			}
		}
		if reverseColumns {
			for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
				runes[left], runes[right] = runes[right], runes[left]
			}
		}
		out = append(out, string(runes))
	}
	return out
}

var reverseMap = map[rune]rune{
	'(': ')', ')': '(',
	'/': '\\', '\\': '/',
	'<': '>', '>': '<',
	'[': ']', ']': '[',
	'{': '}', '}': '{',
}

var flipMap = map[rune]rune{
	'/': '\\', '\\': '/',
	'A': 'V', 'V': 'A',
	'M': 'W', 'W': 'M',
	'P': 'b', 'b': 'P',
	'R': 'b',
	'^': 'v', 'v': '^',
	'_': '-',
	'm': 'w', 'w': 'm',
}

func stripSurroundingNewlines(value string) string {
	rows := splitLines(value)
	out := make([]string, 0, len(rows))
	charsSeen := false
	for _, row := range rows {
		if strings.TrimSpace(row) != "" || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}
	return strings.TrimRightFunc(strings.Join(out, "\n")+"\n", unicode.IsSpace)
}

func render(text, fontName, direction, justify string, width int) (string, error) {
	font, err := loadFont(fontName)
	if err != nil {
		return "", err
	}
	b := newBuilder(text, font, direction, justify, width)
	for b.isNotFinished() {
		if err := b.addChar(); err != nil {
			return "", err
		}
		b.iterator++
	}
	return b.returnProduct(), nil
}

func run() error {
	fs := flag.NewFlagSet("pyfiglet", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	font := "standard"
	fs.StringVar(&font, "f", "standard", "font")
	fs.StringVar(&font, "font", "standard", "font")
	width := 80
	fs.IntVar(&width, "w", 80, "width")
	fs.IntVar(&width, "width", 80, "width")
	justify := "auto"
	fs.StringVar(&justify, "j", "auto", "justification")
	fs.StringVar(&justify, "justify", "auto", "justification")
	direction := "auto"
	fs.StringVar(&direction, "D", "auto", "direction")
	fs.StringVar(&direction, "direction", "auto", "direction")
	reverse := fs.Bool("r", false, "reverse")
	fs.BoolVar(reverse, "reverse", false, "reverse")
	flip := fs.Bool("F", false, "flip")
	fs.BoolVar(flip, "flip", false, "flip")
	strip := fs.Bool("s", false, "strip surrounding newlines")
	fs.BoolVar(strip, "strip-surrounding-newlines", false, "strip surrounding newlines")
	normalize := fs.Bool("n", false, "normalize surrounding newlines")
	fs.BoolVar(normalize, "normalize-surrounding-newlines", false, "normalize surrounding newlines")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	text := strings.Join(fs.Args(), " ")
	if width <= 0 {
		return fmt.Errorf("width must be positive")
	}
	if justify != "auto" && justify != "left" && justify != "center" && justify != "right" {
		return fmt.Errorf("invalid justification")
	}
	if direction != "auto" && direction != "left-to-right" && direction != "right-to-left" {
		return fmt.Errorf("invalid direction")
	}

	output, err := render(text, font, direction, justify, width)
	if err != nil {
		return err
	}
	if *reverse {
		output = strings.Join(translateRows(splitLines(output), reverseMap, true), "\n") + "\n"
	}
	if *flip {
		rows := splitLines(output)
		for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
			rows[left], rows[right] = rows[right], rows[left]
		}
		output = strings.Join(translateRows(rows, flipMap, false), "\n") + "\n"
	}
	if *strip {
		output = stripSurroundingNewlines(output)
	} else if *normalize {
		output = "\n" + stripSurroundingNewlines(output) + "\n"
	}
	_, err = io.WriteString(os.Stdout, output+"\n")
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
EOF

gofmt -w main.go
find /workspace/repo -type f -name '*.py' -delete
