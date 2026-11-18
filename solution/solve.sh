#!/usr/bin/env bash
set -euo pipefail

cd /workspace/repo

font_source=pyfiglet/fonts
if [[ ! -d "$font_source" ]]; then
    font_source=pyfiglet/fonts-standard
fi
if [[ ! -d "$font_source" ]]; then
    echo "FIGlet fonts are missing from the repository" >&2
    exit 1
fi

rm -rf fonts
mkdir -p fonts
cp "$font_source"/*.flf fonts/

cat > go.mod <<'EOF'
module figletgo

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

type glyph struct {
	rows  [][]rune
	width int
}

type font struct {
	hardblank      rune
	height         int
	printDirection int
	hasDirection   bool
	smushMode      int
	glyphs         map[rune]glyph
}

type blankMarker struct {
	buffer   []string
	iterator int
}

type builder struct {
	text              []rune
	direction         string
	width             int
	justify           string
	font              *font
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []blankMarker
	buffer            []string
	product           [][]string
}

func splitFontLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(text)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func trimEndMarker(line string, marker rune) []rune {
	line = strings.TrimRightFunc(line, unicode.IsSpace)
	row := []rune(line)
	for count := 0; count < 2 && len(row) > 0 && row[len(row)-1] == marker; count++ {
		row = row[:len(row)-1]
	}
	return row
}

func rowsAreEmpty(rows [][]rune) bool {
	for _, row := range rows {
		if len(row) != 0 {
			return false
		}
	}
	return true
}

func loadFont(name string) (*font, error) {
	data, err := fs.ReadFile(fontFiles, "fonts/"+name+".flf")
	if err != nil {
		return nil, fmt.Errorf("font %q not found: %w", name, err)
	}
	lines := splitFontLines(string(data))
	if len(lines) == 0 {
		return nil, fmt.Errorf("font %q is empty", name)
	}
	header := lines[0]
	if len(header) < 5 || (header[0] != 'f' && header[0] != 't') || header[1:4] != "lf2" {
		return nil, fmt.Errorf("font %q has an invalid FIGfont header", name)
	}
	fields := strings.Fields(header[5:])
	if len(fields) < 6 {
		return nil, fmt.Errorf("font %q has a malformed FIGfont header", name)
	}
	headerNumbers := make([]int, 5)
	for i := range headerNumbers {
		value, parseErr := strconv.Atoi(fields[i+1])
		if parseErr != nil {
			return nil, fmt.Errorf("font %q has an invalid header number: %w", name, parseErr)
		}
		headerNumbers[i] = value
	}
	hardblankRunes := []rune(fields[0])
	if len(hardblankRunes) != 1 {
		return nil, fmt.Errorf("font %q has an invalid hardblank", name)
	}
	height := headerNumbers[0]
	commentLines := headerNumbers[4]
	if height <= 0 || commentLines < 0 {
		return nil, fmt.Errorf("font %q has invalid height or comment count", name)
	}

	oldLayout := headerNumbers[3]
	printDirection, hasDirection := 0, false
	if len(fields) > 6 {
		printDirection, err = strconv.Atoi(fields[6])
		if err != nil {
			return nil, fmt.Errorf("font %q has an invalid print direction: %w", name, err)
		}
		hasDirection = true
	}
	fullLayout := 0
	if len(fields) > 7 {
		fullLayout, err = strconv.Atoi(fields[7])
		if err != nil {
			return nil, fmt.Errorf("font %q has an invalid full layout: %w", name, err)
		}
	} else if oldLayout == 0 {
		fullLayout = smKern
	} else if oldLayout < 0 {
		fullLayout = 0
	} else {
		fullLayout = (oldLayout & 31) | smSmush
	}

	result := &font{
		hardblank:      hardblankRunes[0],
		height:         height,
		printDirection: printDirection,
		hasDirection:   hasDirection,
		smushMode:      fullLayout,
		glyphs:         make(map[rune]glyph),
	}
	position := 1 + commentLines
	if position > len(lines) {
		return nil, fmt.Errorf("font %q has truncated comments", name)
	}

	readGlyph := func() (glyph, error) {
		rows := make([][]rune, height)
		width := 0
		var marker rune
		for rowIndex := 0; rowIndex < height; rowIndex++ {
			if position >= len(lines) {
				return glyph{}, errors.New("truncated FIGcharacter")
			}
			line := lines[position]
			position++
			if rowIndex == 0 {
				trimmed := []rune(strings.TrimRightFunc(line, unicode.IsSpace))
				if len(trimmed) == 0 {
					return glyph{}, errors.New("FIGcharacter has no end marker")
				}
				marker = trimmed[len(trimmed)-1]
			}
			rows[rowIndex] = trimEndMarker(line, marker)
			if len(rows[rowIndex]) > width {
				width = len(rows[rowIndex])
			}
		}
		return glyph{rows: rows, width: width}, nil
	}

	for code := rune(32); code <= 126; code++ {
		loaded, readErr := readGlyph()
		if readErr != nil {
			return nil, fmt.Errorf("font %q character %d: %w", name, code, readErr)
		}
		if code == 32 || !rowsAreEmpty(loaded.rows) {
			result.glyphs[code] = loaded
		}
	}

	if position < len(lines) {
		for _, code := range []rune("ÄÖÜäöüß") {
			loaded, readErr := readGlyph()
			if readErr != nil {
				return nil, fmt.Errorf("font %q character %U: %w", name, code, readErr)
			}
			if !rowsAreEmpty(loaded.rows) {
				result.glyphs[code] = loaded
			}
		}
	}

	for position < len(lines) {
		line := strings.TrimSpace(lines[position])
		position++
		if line == "" {
			continue
		}
		token := strings.SplitN(line, " ", 2)[0]
		if !strings.HasPrefix(strings.ToLower(token), "0x") {
			continue
		}
		code, parseErr := strconv.ParseInt(token, 0, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("font %q has invalid character code %q: %w", name, token, parseErr)
		}
		loaded, readErr := readGlyph()
		if readErr != nil {
			return nil, fmt.Errorf("font %q character %s: %w", name, token, readErr)
		}
		if !rowsAreEmpty(loaded.rows) {
			result.glyphs[rune(code)] = loaded
		}
	}
	return result, nil
}

func cloneRows(rows []string) []string {
	return append([]string(nil), rows...)
}

func runeLength(text string) int {
	return len([]rune(text))
}

func newBuilder(text string, loaded *font, direction string, width int, justify string) *builder {
	return &builder{
		text:      []rune(text),
		direction: direction,
		width:     width,
		justify:   justify,
		font:      loaded,
		buffer:    make([]string, loaded.height),
	}
}

func (b *builder) glyphAt(index int) (glyph, bool) {
	if index < 0 || index >= len(b.text) {
		return glyph{}, false
	}
	value, ok := b.font.glyphs[b.text[index]]
	return value, ok
}

func (b *builder) smushAmount(current glyph) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		left := []rune(b.buffer[row])
		right := current.rows[row]
		if b.direction == "right-to-left" {
			left, right = right, left
		}

		lineBoundary := len(left)
		for lineBoundary > 0 && left[lineBoundary-1] == ' ' {
			lineBoundary--
		}
		lineBoundary--
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
		} else {
			charBoundary = len(right)
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

	if b.font.smushMode&smHardblank != 0 &&
		left == b.font.hardblank && right == b.font.hardblank {
		return left, true
	}
	if left == b.font.hardblank || right == b.font.hardblank {
		return 0, false
	}
	if b.font.smushMode&smEqual != 0 && left == right {
		return left, true
	}

	if b.font.smushMode&smLowline != 0 {
		hierarchy := "|/\\[]{}()<>"
		if left == '_' && containsRune(hierarchy, right) {
			return right, true
		}
		if right == '_' && containsRune(hierarchy, left) {
			return left, true
		}
	}

	if b.font.smushMode&smHierarchy != 0 {
		hierarchies := [][2]string{
			{"|", "/\\[]{}()<>"},
			{"\\/", "[]{}()<>"},
			{"[]", "{}()<>"},
			{"{}", "()<>"},
			{"()", "<>"},
		}
		for _, pair := range hierarchies {
			if containsRune(pair[0], left) && containsRune(pair[1], right) {
				return right, true
			}
			if containsRune(pair[0], right) && containsRune(pair[1], left) {
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

func (b *builder) getLeftSmushedChar(index int, addLeft []rune) (rune, int) {
	position := len(addLeft) - b.maxSmush + index
	if position >= 0 && position < len(addLeft) {
		return addLeft[position], position
	}
	return 0, position
}

func (b *builder) smushRow(current glyph, row int) (string, string, error) {
	addLeft := []rune(b.buffer[row])
	addRight := append([]rune(nil), current.rows[row]...)
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for index := 0; index < b.maxSmush; index++ {
		if index >= len(addRight) {
			return "", "", errors.New("FIGcharacter row is shorter than its smush width")
		}
		left, position := b.getLeftSmushedChar(index, addLeft)
		smushed, ok := b.smushChars(left, addRight[index])
		if !ok {
			return "", "", fmt.Errorf("FIGcharacter edge cannot be smushed (%q, %q)", left, addRight[index])
		}
		if position >= 0 && position < len(addLeft) {
			addLeft[position] = smushed
		}
	}
	return string(addLeft), string(addRight), nil
}

func (b *builder) addCurrentRow(current glyph, row int) error {
	addLeft, addRight, err := b.smushRow(current, row)
	if err != nil {
		return err
	}
	right := []rune(addRight)
	if b.maxSmush < 0 || b.maxSmush > len(right) {
		return errors.New("invalid FIGcharacter smush width")
	}
	b.buffer[row] = addLeft + string(right[b.maxSmush:])
	return nil
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	current, ok := b.glyphAt(b.iterator)
	if !ok {
		return
	}
	b.maxSmush = b.smushAmount(current)
}

func (b *builder) handleNewLine() {
	if len(b.blankMarkers) > 0 {
		last := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.product = append(b.product, cloneRows(last.buffer))
		b.iterator = last.iterator
		b.cutBufferCommon()
		return
	}
	b.product = append(b.product, cloneRows(b.buffer))
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) addCharToProduct() error {
	value := b.text[b.iterator]
	current, exists := b.glyphAt(b.iterator)
	if value == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{
			buffer: cloneRows(b.buffer), iterator: b.iterator,
		})
		b.handleNewLine()
		return nil
	}
	if !exists {
		return nil
	}
	if b.width < current.width {
		return fmt.Errorf("width is not enough to print character %q", value)
	}

	b.curCharWidth = current.width
	b.maxSmush = b.smushAmount(current)
	b.currentTotalWidth = runeLength(b.buffer[0]) + b.curCharWidth - b.maxSmush
	if value == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{
			buffer: cloneRows(b.buffer), iterator: b.iterator,
		})
	}

	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			if err := b.addCurrentRow(current, row); err != nil {
				return err
			}
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) justifyRow(row string) string {
	length := runeLength(row)
	padding := 0
	switch b.justify {
	case "right":
		padding = b.width - length - 1
	case "center":
		padding = (b.width - length) / 2
	}
	if padding > 0 {
		return strings.Repeat(" ", padding) + row
	}
	return row
}

func (b *builder) formatProduct() string {
	var output strings.Builder
	for _, rows := range b.product {
		for index, row := range rows {
			if index > 0 {
				output.WriteByte('\n')
			}
			output.WriteString(strings.ReplaceAll(b.justifyRow(row), string(b.font.hardblank), " "))
		}
		output.WriteByte('\n')
	}
	return output.String()
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
	return b.formatProduct(), nil
}

func splitRenderedLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func translateASCII(row string, table string) string {
	mapping := []rune(table)
	result := []rune(row)
	for index, value := range result {
		if value >= 32 && value <= 127 {
			result[index] = mapping[int(value)-32]
		}
	}
	return string(result)
}

const reverseASCII = " !\"#$%&')(*+,-.\\" +
	"0123456789:;>=<?" +
	"@ABCDEFGHIJKLMNO" +
	"PQRSTUVWXYZ]/[^_" +
	"`abcdefghijklmno" +
	"pqrstuvwxyz}|{~\x7f"

const flipASCII = " !\"#$%&'()*+,-.\\" +
	"0123456789:;<=>?" +
	"@VBCDEFGHIJKLWNO" +
	"bQbSTUAMXYZ[/]v-" +
	"`aPcdefghijklwno" +
	"pqrstu^mxyz{|}~\x7f"

func reverseRendered(text string) string {
	lines := splitRenderedLines(text)
	for index, line := range lines {
		values := []rune(translateASCII(line, reverseASCII))
		for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
			values[left], values[right] = values[right], values[left]
		}
		lines[index] = string(values)
	}
	return strings.Join(lines, "\n") + "\n"
}

func flipRendered(text string) string {
	lines := splitRenderedLines(text)
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	for index, line := range lines {
		lines[index] = translateASCII(line, flipASCII)
	}
	return strings.Join(lines, "\n") + "\n"
}

func stripSurroundingNewlines(text string) string {
	lines := splitRenderedLines(text)
	output := make([]string, 0, len(lines))
	charactersSeen := false
	for _, line := range lines {
		if strings.TrimSpace(line) != "" || charactersSeen {
			charactersSeen = true
			output = append(output, line)
		}
	}
	return strings.TrimRightFunc(strings.Join(output, "\n")+"\n", unicode.IsSpace)
}

func renderText(text string, loaded *font, direction string, width int, justify string) (string, error) {
	if direction == "auto" {
		if loaded.hasDirection && loaded.printDirection == 1 {
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
	return newBuilder(text, loaded, direction, width, justify).render()
}

func run() int {
	flags := flag.NewFlagSet("pyfiglet", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	fontName := "standard"
	width := 80
	justify := "auto"
	direction := "auto"
	reverse := false
	flip := false
	strip := false
	normalize := false
	flags.StringVar(&fontName, "f", "standard", "font")
	flags.StringVar(&fontName, "font", "standard", "font")
	flags.IntVar(&width, "w", 80, "width")
	flags.IntVar(&width, "width", 80, "width")
	flags.StringVar(&justify, "j", "auto", "justification")
	flags.StringVar(&justify, "justify", "auto", "justification")
	flags.StringVar(&direction, "D", "auto", "direction")
	flags.StringVar(&direction, "direction", "auto", "direction")
	flags.BoolVar(&reverse, "r", false, "reverse output")
	flags.BoolVar(&reverse, "reverse", false, "reverse output")
	flags.BoolVar(&flip, "F", false, "flip output")
	flags.BoolVar(&flip, "flip", false, "flip output")
	flags.BoolVar(&strip, "s", false, "strip surrounding newlines")
	flags.BoolVar(&strip, "strip-surrounding-newlines", false, "strip surrounding newlines")
	flags.BoolVar(&normalize, "n", false, "normalize surrounding newlines")
	flags.BoolVar(&normalize, "normalize-surrounding-newlines", false, "normalize surrounding newlines")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if len(flags.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "text is required")
		return 1
	}
	text := strings.Join(flags.Args(), " ")

	loaded, err := loadFont(fontName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result, err := renderText(text, loaded, direction, width, justify)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if reverse {
		result = reverseRendered(result)
	}
	if flip {
		result = flipRendered(result)
	}
	if strip {
		result = stripSurroundingNewlines(result)
	} else if normalize {
		result = "\n" + stripSurroundingNewlines(result) + "\n"
	}
	if _, err := os.Stdout.Write([]byte(result + "\n")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
EOF

find . -type f -name '*.py' -delete
find . -type f -name '*.pyc' -delete
find . -type d -name '__pycache__' -prune -exec rm -rf {} +

gofmt -w main.go
go build -o /tmp/figlet .
