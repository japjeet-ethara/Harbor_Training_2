#!/bin/sh
set -eu

cd /workspace/repo
mkdir -p fonts
for font in standard slant small big mini banner shadow script; do
    cp "pyfiglet/fonts/${font}.flf" "fonts/${font}.flf"
done

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
	"io/fs"
	"os"
	"path"
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

type font struct {
	height      int
	hardBlank   rune
	printDir    int
	smushMode   int
	glyphs      map[rune][]string
	glyphWidths map[rune]int
}

type blankMarker struct {
	rows     []string
	iterator int
}

type builder struct {
	text              []rune
	iterator          int
	direction         string
	width             int
	justify           string
	font              *font
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []blankMarker
	product           [][]string
	buffer            []string
}

func readFont(name string) (*font, error) {
	name = path.Base(name)
	if strings.HasSuffix(strings.ToLower(name), ".flf") {
		name = name[:len(name)-4]
	}
	data, err := fs.ReadFile(fontFiles, "fonts/"+name+".flf")
	if err != nil {
		return nil, fmt.Errorf("font %q not found", name)
	}
	return parseFont(name, string(data))
}

func parseFont(name, data string) (*font, error) {
	data = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(data)
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	lines := strings.Split(data, "\n")
	if strings.HasSuffix(data, "\n") {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 || len(lines[0]) < 5 ||
		!(strings.HasPrefix(lines[0], "flf2") || strings.HasPrefix(lines[0], "tlf2")) {
		return nil, fmt.Errorf("invalid FIGlet font %q", name)
	}
	header := strings.Fields(lines[0][5:])
	if len(header) < 6 {
		return nil, fmt.Errorf("malformed FIGlet header in %q", name)
	}
	headerInt := func(index int) (int, error) {
		value, err := strconv.Atoi(header[index])
		if err != nil {
			return 0, fmt.Errorf("malformed FIGlet header in %q: %w", name, err)
		}
		return value, nil
	}
	height, err := headerInt(1)
	if err != nil {
		return nil, err
	}
	oldLayout, err := headerInt(4)
	if err != nil {
		return nil, err
	}
	commentLines, err := headerInt(5)
	if err != nil {
		return nil, err
	}
	if height <= 0 || commentLines < 0 {
		return nil, fmt.Errorf("invalid FIGlet dimensions in %q", name)
	}

	printDir := -1
	if len(header) > 6 {
		printDir, err = headerInt(6)
		if err != nil {
			return nil, err
		}
	}
	smushMode := 0
	if len(header) > 7 {
		smushMode, err = headerInt(7)
		if err != nil {
			return nil, err
		}
	} else if oldLayout == 0 {
		smushMode = smKern
	} else if oldLayout > 0 {
		smushMode = (oldLayout & 31) | smSmush
	}
	hardBlanks := []rune(header[0])
	if len(hardBlanks) == 0 {
		return nil, fmt.Errorf("missing hardblank in %q", name)
	}
	result := &font{
		height:      height,
		hardBlank:   hardBlanks[0],
		printDir:    printDir,
		smushMode:   smushMode,
		glyphs:      make(map[rune][]string),
		glyphWidths: make(map[rune]int),
	}

	index := 1
	if index+commentLines > len(lines) {
		return nil, fmt.Errorf("truncated comments in %q", name)
	}
	index += commentLines
	parseGlyph := func() ([]string, int, error) {
		if index+height > len(lines) {
			return nil, 0, fmt.Errorf("truncated glyph data in %q", name)
		}
		rawRows := lines[index : index+height]
		index += height
		first := []rune(rawRows[0])
		last := len(first) - 1
		for last >= 0 && unicode.IsSpace(first[last]) {
			last--
		}
		if last < 0 {
			return nil, 0, fmt.Errorf("missing glyph endmark in %q", name)
		}
		endmark := first[last]
		rows := make([]string, height)
		maxWidth := 0
		for rowIndex, raw := range rawRows {
			runes := []rune(raw)
			end := len(runes) - 1
			for end >= 0 && unicode.IsSpace(runes[end]) {
				end--
			}
			if end >= 0 && runes[end] == endmark {
				start := end
				if start > 0 && runes[start-1] == endmark {
					start--
				}
				runes = runes[:start]
			}
			rows[rowIndex] = string(runes)
			if len(runes) > maxWidth {
				maxWidth = len(runes)
			}
		}
		return rows, maxWidth, nil
	}

	for code := rune(32); code < 127; code++ {
		rows, glyphWidth, err := parseGlyph()
		if err != nil {
			return nil, err
		}
		if code == 32 || strings.Join(rows, "") != "" {
			result.glyphs[code] = rows
			result.glyphWidths[code] = glyphWidth
		}
	}
	if index < len(lines) {
		for _, code := range []rune("ÄÖÜäöüß") {
			rows, glyphWidth, err := parseGlyph()
			if err != nil {
				return nil, err
			}
			if strings.Join(rows, "") != "" {
				result.glyphs[code] = rows
				result.glyphWidths[code] = glyphWidth
			}
		}
	}
	for index < len(lines) {
		line := strings.TrimSpace(lines[index])
		index++
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(strings.ToLower(fields[0]), "0x") {
			continue
		}
		codeValue, err := strconv.ParseInt(fields[0][2:], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid FIGlet code tag in %q: %w", name, err)
		}
		rows, glyphWidth, err := parseGlyph()
		if err != nil {
			return nil, err
		}
		if strings.Join(rows, "") != "" {
			result.glyphs[rune(codeValue)] = rows
			result.glyphWidths[rune(codeValue)] = glyphWidth
		}
	}
	return result, nil
}

func cloneRows(rows []string) []string {
	return append([]string(nil), rows...)
}

func (b *builder) charAt(index int) ([]string, bool) {
	if index < 0 || index >= len(b.text) {
		return nil, false
	}
	rows, ok := b.font.glyphs[b.text[index]]
	return rows, ok
}

func (b *builder) charWidthAt(index int) (int, bool) {
	if index < 0 || index >= len(b.text) {
		return 0, false
	}
	width, ok := b.font.glyphWidths[b.text[index]]
	return width, ok
}

func runeLen(text string) int {
	return len([]rune(text))
}

func (b *builder) smushAmount(curChar []string) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		left := []rune(b.buffer[row])
		right := []rune(curChar[row])
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
		leftChar := rune(0)
		if lineBoundary < len(left) {
			leftChar = left[lineBoundary]
		} else {
			lineBoundary = 0
		}
		charBoundary := 0
		for charBoundary < len(right) && right[charBoundary] == ' ' {
			charBoundary++
		}
		rightChar := rune(0)
		if charBoundary < len(right) {
			rightChar = right[charBoundary]
		} else {
			charBoundary = len(right)
		}
		amount := charBoundary + len(left) - 1 - lineBoundary
		if leftChar == 0 || leftChar == ' ' {
			amount++
		} else if rightChar != 0 {
			if _, ok := b.smushChar(leftChar, rightChar); ok {
				amount++
			}
		}
		if amount < maxSmush {
			maxSmush = amount
		}
	}
	return maxSmush
}

func runeInSet(char rune, set string) bool {
	return strings.ContainsRune(set, char)
}

func (b *builder) smushChar(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 || b.font.smushMode&smSmush == 0 {
		return 0, false
	}
	if b.font.smushMode&63 == 0 {
		if left == b.font.hardBlank {
			return right, true
		}
		if right == b.font.hardBlank {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if b.font.smushMode&smHardblank != 0 && left == b.font.hardBlank && right == b.font.hardBlank {
		return left, true
	}
	if left == b.font.hardBlank || right == b.font.hardBlank {
		return 0, false
	}
	if b.font.smushMode&smEqual != 0 && left == right {
		return left, true
	}
	if b.font.smushMode&smLowline != 0 {
		if left == '_' && runeInSet(right, "|/\\[]{}()<>") {
			return right, true
		}
		if right == '_' && runeInSet(left, "|/\\[]{}()<>") {
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
		for _, classes := range hierarchy {
			if runeInSet(left, classes[0]) && runeInSet(right, classes[1]) {
				return right, true
			}
			if runeInSet(right, classes[0]) && runeInSet(left, classes[1]) {
				return left, true
			}
		}
	}
	if b.font.smushMode&smPair != 0 {
		pair := string([]rune{left, right})
		reversePair := string([]rune{right, left})
		if pair == "[]" || pair == "{}" || pair == "()" ||
			reversePair == "[]" || reversePair == "{}" || reversePair == "()" {
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

func (b *builder) smushRow(curChar []string, row int) (string, string) {
	addLeft := []rune(b.buffer[row])
	addRight := []rune(curChar[row])
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	overlap := b.maxSmush
	for i := 0; i < overlap; i++ {
		index := len(addLeft) - overlap + i
		left := rune(0)
		if index >= 0 && index < len(addLeft) {
			left = addLeft[index]
		}
		var right rune
		if i < len(addRight) {
			right = addRight[i]
		}
		if smushed, ok := b.smushChar(left, right); ok && index >= 0 && index < len(addLeft) {
			addLeft[index] = smushed
		}
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap > len(addRight) {
		overlap = len(addRight)
	}
	return string(addLeft), string(addRight[overlap:])
}

func (b *builder) addCurCharRow(curChar []string, row int) {
	left, right := b.smushRow(curChar, row)
	b.buffer[row] = left + right
}

func (b *builder) lastBlank() (blankMarker, bool) {
	if len(b.blankMarkers) == 0 {
		return blankMarker{}, false
	}
	last := b.blankMarkers[len(b.blankMarkers)-1]
	b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
	return last, true
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	if curChar, ok := b.charAt(b.iterator); ok {
		b.maxSmush = b.smushAmount(curChar)
	}
}

func (b *builder) cutBufferAtLastBlank(marker blankMarker) {
	b.product = append(b.product, cloneRows(marker.rows))
	b.iterator = marker.iterator
	b.cutBufferCommon()
}

func (b *builder) cutBufferAtLastChar() {
	b.product = append(b.product, cloneRows(b.buffer))
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) handleNewLine() {
	if marker, ok := b.lastBlank(); ok {
		b.cutBufferAtLastBlank(marker)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *builder) addCharToProduct() error {
	current := b.text[b.iterator]
	if current == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneRows(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}
	curChar, ok := b.charAt(b.iterator)
	if !ok {
		return nil
	}
	charWidth, _ := b.charWidthAt(b.iterator)
	if b.width < charWidth {
		return fmt.Errorf("width is not enough to print character %q", current)
	}
	b.curCharWidth = charWidth
	b.maxSmush = b.smushAmount(curChar)
	b.currentTotalWidth = runeLen(b.buffer[0]) + b.curCharWidth - b.maxSmush
	if current == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneRows(b.buffer), b.iterator})
	}
	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addCurCharRow(curChar, row)
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func justifyRows(justify string, rows []string, width int) []string {
	result := cloneRows(rows)
	for row, text := range result {
		padding := 0
		switch justify {
		case "right":
			padding = width - runeLen(text) - 1
		case "center":
			padding = (width - runeLen(text)) / 2
		}
		if padding > 0 {
			result[row] = strings.Repeat(" ", padding) + text
		}
	}
	return result
}

func (b *builder) render() (string, error) {
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
		b.iterator++
	}
	if b.buffer[0] != "" {
		b.product = append(b.product, cloneRows(b.buffer))
	}
	var output strings.Builder
	for _, block := range b.product {
		rows := justifyRows(b.justify, block, b.width)
		for _, row := range rows {
			output.WriteString(strings.ReplaceAll(row, string(b.font.hardBlank), " "))
			output.WriteByte('\n')
		}
	}
	return output.String(), nil
}

func makeBuilder(text string, currentFont *font, direction, justify string, width int) *builder {
	if direction == "auto" {
		if currentFont.printDir == 1 {
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
		width:     width,
		justify:   justify,
		font:      currentFont,
		buffer:    make([]string, currentFont.height),
	}
}

func identityTranslation() []rune {
	table := make([]rune, 256)
	for i := range table {
		table[i] = rune(i)
	}
	return table
}

func translation(segments ...string) []rune {
	table := identityTranslation()
	index := 32
	for _, segment := range segments {
		for _, char := range segment {
			if index < len(table) {
				table[index] = char
				index++
			}
		}
	}
	return table
}

var reverseTable = translation(
	" !\"#$%&')(*+,-.\\",
	"0123456789:;>=<?",
	"@ABCDEFGHIJKLMNO",
	"PQRSTUVWXYZ]/[^_",
	"`abcdefghijklmno",
	"pqrstuvwxyz}|{~\x7f",
)

var flipTable = translation(
	" !\"#$%&'()*+,-.\\",
	"0123456789:;<=>?",
	"@VBCDEFGHIJKLWNO",
	"bQbSTUAMXYZ[/]v-",
	"`aPcdefghijklwno",
	"pqrstu^mxyz{|}~\x7f",
)

func translateChar(char rune, table []rune) rune {
	if char >= 0 && char < rune(len(table)) {
		return table[char]
	}
	return char
}

func pythonSplitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

func reverseOutput(text string) string {
	lines := pythonSplitLines(text)
	for row, line := range lines {
		runes := []rune(line)
		for i, char := range runes {
			runes[i] = translateChar(char, reverseTable)
		}
		for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
			runes[left], runes[right] = runes[right], runes[left]
		}
		lines[row] = string(runes)
	}
	return joinLines(lines)
}

func flipOutput(text string) string {
	lines := pythonSplitLines(text)
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	for row, line := range lines {
		runes := []rune(line)
		for i, char := range runes {
			runes[i] = translateChar(char, flipTable)
		}
		lines[row] = string(runes)
	}
	return joinLines(lines)
}

func stripSurroundingNewlines(text string) string {
	lines := pythonSplitLines(text)
	output := make([]string, 0, len(lines))
	charactersSeen := false
	for _, line := range lines {
		if strings.TrimSpace(line) != "" || charactersSeen {
			charactersSeen = true
			output = append(output, line)
		}
	}
	return strings.TrimRightFunc(joinLines(output), unicode.IsSpace)
}

func renderCLI(text, fontName, direction, justify string, width int, reverse, flip, strip, normalize bool) (string, error) {
	currentFont, err := readFont(fontName)
	if err != nil {
		return "", err
	}
	output, err := makeBuilder(text, currentFont, direction, justify, width).render()
	if err != nil {
		return "", err
	}
	if reverse {
		output = reverseOutput(output)
	}
	if flip {
		output = flipOutput(output)
	}
	if strip {
		output = stripSurroundingNewlines(output)
	} else if normalize {
		output = "\n" + stripSurroundingNewlines(output) + "\n"
	}
	return output, nil
}

func run() error {
	fontName := "standard"
	direction := "auto"
	justify := "auto"
	width := 80
	reverse := false
	flip := false
	strip := false
	normalize := false

	flags := flag.NewFlagSet("figlet", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&fontName, "f", "standard", "font")
	flags.StringVar(&fontName, "font", "standard", "font")
	flags.IntVar(&width, "w", 80, "width")
	flags.IntVar(&width, "width", 80, "width")
	flags.StringVar(&justify, "j", "auto", "justification")
	flags.StringVar(&justify, "justify", "auto", "justification")
	flags.StringVar(&direction, "D", "auto", "direction")
	flags.StringVar(&direction, "direction", "auto", "direction")
	flags.BoolVar(&reverse, "r", false, "reverse")
	flags.BoolVar(&reverse, "reverse", false, "reverse")
	flags.BoolVar(&flip, "F", false, "flip")
	flags.BoolVar(&flip, "flip", false, "flip")
	flags.BoolVar(&strip, "s", false, "strip surrounding newlines")
	flags.BoolVar(&strip, "strip-surrounding-newlines", false, "strip surrounding newlines")
	flags.BoolVar(&normalize, "n", false, "normalize surrounding newlines")
	flags.BoolVar(&normalize, "normalize-surrounding-newlines", false, "normalize surrounding newlines")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	switch direction {
	case "auto", "left-to-right", "right-to-left":
	default:
		return fmt.Errorf("invalid direction %q", direction)
	}
	switch justify {
	case "auto", "left", "center", "right":
	default:
		return fmt.Errorf("invalid justification %q", justify)
	}
	if width <= 0 {
		return errors.New("width must be positive")
	}
	text := strings.Join(flags.Args(), " ")
	output, err := renderCLI(text, fontName, direction, justify, width, reverse, flip, strip, normalize)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write([]byte(output)); err != nil {
		return err
	}
	_, err = os.Stdout.Write([]byte{'\n'})
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
EOF

find . -type d -name __pycache__ -prune -exec rm -rf {} +
find . -type f -name '*.py' -delete