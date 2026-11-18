#!/bin/sh
set -eu

cd /workspace/repo
mkdir -p fonts
for font in standard slant small big mini banner shadow script; do
    cp "pyfiglet/fonts-standard/${font}.flf" fonts/
done

cat > go.mod <<'GO_MOD'
module figlet

go 1.23
GO_MOD

cat > main.go <<'GO_SOURCE'
package main

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

//go:embed fonts/*.flf
var fontFiles embed.FS

type options struct {
	font      string
	width     int
	justify   string
	direction string
	reverse   bool
	flip      bool
	strip     bool
	normalize bool
	text      string
}

type glyph struct {
	rows  [][]rune
	width int
}

type figFont struct {
	height         int
	hardBlank      rune
	smushMode      int
	printDirection *int
	glyphs         map[rune]glyph
}

type blankMarker struct {
	rows     [][]rune
	iterator int
}

type figletBuilder struct {
	text              []rune
	font              *figFont
	direction         string
	width             int
	justify           string
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []blankMarker
	buffer            [][]rune
	product           [][][]rune
}

var magicNumber = regexp.MustCompile(`^[tf]lf2.`)

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if opts.text == "" {
		fmt.Fprintln(os.Stderr, "text is required")
		os.Exit(2)
	}

	font, err := loadFont(opts.font)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	direction := opts.direction
	if direction == "auto" {
		if font.printDirection != nil && *font.printDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}
	justify := opts.justify
	if justify == "auto" {
		if direction == "right-to-left" {
			justify = "right"
		} else {
			justify = "left"
		}
	}

	rendered, err := render(opts.text, font, direction, justify, opts.width)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.reverse {
		rendered = reverseRows(rendered)
	}
	if opts.flip {
		rendered = flipRows(rendered)
	}
	if opts.strip {
		rendered = stripSurroundingNewlines(rendered)
	} else if opts.normalize {
		rendered = "\n" + stripSurroundingNewlines(rendered) + "\n"
	}
	_, _ = os.Stdout.WriteString(rendered)
	_, _ = os.Stdout.Write([]byte{'\n'})
}

func parseOptions(args []string) (options, error) {
	opts := options{
		font:      "standard",
		width:     80,
		justify:   "auto",
		direction: "auto",
	}
	text := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			text = append(text, args[i+1:]...)
			break
		}
		switch arg {
		case "-f", "--font":
			value, next, err := optionValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			opts.font, i = value, next
		case "-w", "--width":
			value, next, err := optionValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			width, err := strconv.Atoi(value)
			if err != nil {
				return opts, fmt.Errorf("invalid width %q", value)
			}
			opts.width, i = width, next
		case "-j", "--justify":
			value, next, err := optionValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			if value != "auto" && value != "left" && value != "center" && value != "right" {
				return opts, fmt.Errorf("invalid justification %q", value)
			}
			opts.justify, i = value, next
		case "-D", "--direction":
			value, next, err := optionValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			if value != "auto" && value != "left-to-right" && value != "right-to-left" {
				return opts, fmt.Errorf("invalid direction %q", value)
			}
			opts.direction, i = value, next
		case "-r", "--reverse":
			opts.reverse = true
		case "-F", "--flip":
			opts.flip = true
		case "-s", "--strip-surrounding-newlines":
			opts.strip = true
		case "-n", "--normalize-surrounding-newlines":
			opts.normalize = true
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown option %q", arg)
			}
			text = append(text, arg)
		}
	}
	opts.text = strings.Join(text, " ")
	return opts, nil
}

func optionValue(args []string, index int, name string) (string, int, error) {
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("option %s requires a value", name)
	}
	return args[index+1], index + 1, nil
}

func loadFont(name string) (*figFont, error) {
	data, err := fontFiles.ReadFile("fonts/" + name + ".flf")
	if err != nil {
		return nil, fmt.Errorf("font %q not found", name)
	}
	lines := splitFontLines(string(data))
	if len(lines) == 0 {
		return nil, errors.New("empty font file")
	}
	header := lines[0]
	match := magicNumber.FindStringIndex(header)
	if match == nil {
		return nil, fmt.Errorf("%s is not a valid FIGlet font", name)
	}
	fields := strings.Fields(header[match[1]:])
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	hardBlankRunes := []rune(fields[0])
	if len(hardBlankRunes) == 0 {
		return nil, fmt.Errorf("missing hardblank in %s", name)
	}
	values := make([]int, 5)
	for i := range values {
		value, err := strconv.Atoi(fields[i+1])
		if err != nil {
			return nil, fmt.Errorf("malformed header for %s: %w", name, err)
		}
		values[i] = value
	}
	height, commentLines := values[0], values[4]
	if height <= 0 || commentLines < 0 || commentLines+1 > len(lines) {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	var printDirection *int
	if len(fields) > 6 {
		value, err := strconv.Atoi(fields[6])
		if err != nil {
			return nil, fmt.Errorf("malformed print direction in %s", name)
		}
		printDirection = &value
	}
	smushMode := 0
	if len(fields) > 7 {
		value, err := strconv.Atoi(fields[7])
		if err != nil {
			return nil, fmt.Errorf("malformed layout mode in %s", name)
		}
		smushMode = value
	} else {
		oldLayout := values[3]
		switch {
		case oldLayout == 0:
			smushMode = 64
		case oldLayout < 0:
			smushMode = 0
		default:
			smushMode = (oldLayout & 31) | 128
		}
	}

	font := &figFont{
		height:         height,
		hardBlank:      hardBlankRunes[0],
		smushMode:      smushMode,
		printDirection: printDirection,
		glyphs:         make(map[rune]glyph),
	}
	lines = lines[1+commentLines:]
	position := 0
	readGlyph := func() (glyph, error) {
		return readFontGlyph(lines, &position, height)
	}
	for code := rune(32); code <= 126; code++ {
		item, err := readGlyph()
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s font: %w", name, err)
		}
		if code == ' ' || glyphHasContent(item) {
			font.glyphs[code] = item
		}
	}
	for _, code := range "ÄÖÜäöüß" {
		if position >= len(lines) {
			break
		}
		item, err := readGlyph()
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s font: %w", name, err)
		}
		if glyphHasContent(item) {
			font.glyphs[code] = item
		}
	}
	for position < len(lines) {
		line := strings.TrimSpace(lines[position])
		position++
		if line == "" {
			continue
		}
		token := strings.Fields(line)[0]
		if len(token) < 2 || !(strings.HasPrefix(token, "0x") || strings.HasPrefix(token, "0X")) {
			continue
		}
		code, err := strconv.ParseInt(token[2:], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s font: invalid codepoint %q", name, token)
		}
		item, err := readGlyph()
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s font: %w", name, err)
		}
		if glyphHasContent(item) {
			font.glyphs[rune(code)] = item
		}
	}
	return font, nil
}

func splitFontLines(data string) []string {
	data = strings.NewReplacer(
		"\u0085", " ",
		"\u2028", " ",
		"\u2029", " ",
		"\r\n", "\n",
		"\r", "\n",
		"\v", "\n",
		"\f", "\n",
		"\u001c", "\n",
		"\u001d", "\n",
		"\u001e", "\n",
	).Replace(data)
	if data == "" {
		return nil
	}
	lines := strings.Split(data, "\n")
	if strings.HasSuffix(data, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func readFontGlyph(lines []string, position *int, height int) (glyph, error) {
	if *position+height > len(lines) {
		return glyph{}, errors.New("unexpected end of font data")
	}
	first := []rune(lines[*position])
	end := rune(0)
	for i := len(first) - 1; i >= 0; i-- {
		if !unicode.IsSpace(first[i]) {
			end = first[i]
			break
		}
	}
	if end == 0 {
		return glyph{}, errors.New("missing glyph end marker")
	}
	rows := make([][]rune, height)
	width := 0
	for row := 0; row < height; row++ {
		line := []rune(lines[*position])
		*position++
		line = trimGlyphEnd(line, end)
		rows[row] = line
		if len(line) > width {
			width = len(line)
		}
	}
	return glyph{rows: rows, width: width}, nil
}

func trimGlyphEnd(line []rune, end rune) []rune {
	last := len(line)
	for last > 0 && unicode.IsSpace(line[last-1]) {
		last--
	}
	for count := 0; count < 2 && last > 0 && line[last-1] == end; count++ {
		last--
	}
	return line[:last]
}

func glyphHasContent(item glyph) bool {
	for _, row := range item.rows {
		if len(row) > 0 {
			return true
		}
	}
	return false
}

func render(text string, font *figFont, direction, justify string, width int) (string, error) {
	builder := &figletBuilder{
		text:      []rune(text),
		font:      font,
		direction: direction,
		width:     width,
		justify:   justify,
		buffer:    makeRows(font.height),
	}
	for builder.iterator < len(builder.text) {
		if err := builder.addCharToProduct(); err != nil {
			return "", err
		}
		builder.iterator++
	}
	if len(builder.buffer) > 0 && len(builder.buffer[0]) > 0 {
		builder.product = append(builder.product, builder.buffer)
	}
	var output strings.Builder
	for _, block := range builder.product {
		for rowIndex := range block {
			row := block[rowIndex]
			if justify == "right" {
				pad := width - len(row) - 1
				if pad > 0 {
					row = append(spaces(pad), row...)
				}
			} else if justify == "center" {
				pad := (width - len(row)) / 2
				if pad > 0 {
					row = append(spaces(pad), row...)
				}
			}
			for _, char := range row {
				if char == font.hardBlank {
					char = ' '
				}
				output.WriteRune(char)
			}
			output.WriteByte('\n')
		}
	}
	return output.String(), nil
}

func makeRows(height int) [][]rune {
	rows := make([][]rune, height)
	for i := range rows {
		rows[i] = []rune{}
	}
	return rows
}

func spaces(count int) []rune {
	result := make([]rune, count)
	for i := range result {
		result[i] = ' '
	}
	return result
}

func (builder *figletBuilder) addCharToProduct() error {
	currentCode := builder.text[builder.iterator]
	if currentCode == '\n' {
		builder.blankMarkers = append(builder.blankMarkers, blankMarker{
			rows:     cloneRows(builder.buffer),
			iterator: builder.iterator,
		})
		builder.handleNewLine()
		return nil
	}
	current, ok := builder.font.glyphs[currentCode]
	if !ok {
		return nil
	}
	if builder.width < current.width {
		return fmt.Errorf("width is not enough to print this character")
	}
	builder.curCharWidth = current.width
	builder.maxSmush = builder.currentSmushAmount(current)
	builder.currentTotalWidth = len(builder.buffer[0]) + builder.curCharWidth - builder.maxSmush
	if currentCode == ' ' {
		builder.blankMarkers = append(builder.blankMarkers, blankMarker{
			rows:     cloneRows(builder.buffer),
			iterator: builder.iterator,
		})
	}
	if builder.currentTotalWidth >= builder.width {
		builder.handleNewLine()
	} else {
		for row := 0; row < builder.font.height; row++ {
			builder.addCurCharRowToBufferRow(current, row)
		}
	}
	builder.prevCharWidth = builder.curCharWidth
	return nil
}

func (builder *figletBuilder) currentSmushAmount(current glyph) int {
	return builder.smushAmount(current)
}

func (builder *figletBuilder) smushAmount(current glyph) int {
	if builder.font.smushMode&(128|64) == 0 {
		return 0
	}
	maxSmush := builder.curCharWidth
	for row := 0; row < builder.font.height; row++ {
		lineLeft := builder.buffer[row]
		lineRight := current.rows[row]
		if builder.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}
		lineBD := len(trimRightSpaces(lineLeft)) - 1
		if lineBD < 0 {
			lineBD = 0
		}
		leftChar := rune(0)
		if lineBD < len(lineLeft) {
			leftChar = lineLeft[lineBD]
		} else {
			lineBD = 0
		}
		charBD := len(lineRight) - len(trimLeftSpaces(lineRight))
		rightChar := rune(0)
		if charBD < len(lineRight) {
			rightChar = lineRight[charBD]
		} else {
			charBD = len(lineRight)
		}
		amount := charBD + len(lineLeft) - 1 - lineBD
		if leftChar == 0 || leftChar == ' ' {
			amount++
		} else if rightChar != 0 {
			if _, ok := builder.smushChars(leftChar, rightChar); ok {
				amount++
			}
		}
		if amount < maxSmush {
			maxSmush = amount
		}
	}
	return maxSmush
}

func trimRightSpaces(line []rune) []rune {
	end := len(line)
	for end > 0 && line[end-1] == ' ' {
		end--
	}
	return line[:end]
}

func trimLeftSpaces(line []rune) []rune {
	start := 0
	for start < len(line) && line[start] == ' ' {
		start++
	}
	return line[start:]
}

func (builder *figletBuilder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if builder.prevCharWidth < 2 || builder.curCharWidth < 2 {
		return 0, false
	}
	if builder.font.smushMode&128 == 0 {
		return 0, false
	}
	if builder.font.smushMode&63 == 0 {
		if left == builder.font.hardBlank {
			return right, true
		}
		if right == builder.font.hardBlank {
			return left, true
		}
		if builder.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if builder.font.smushMode&32 != 0 && left == builder.font.hardBlank && right == builder.font.hardBlank {
		return left, true
	}
	if left == builder.font.hardBlank || right == builder.font.hardBlank {
		return 0, false
	}
	if builder.font.smushMode&1 != 0 && left == right {
		return left, true
	}
	if builder.font.smushMode&2 != 0 {
		if left == '_' && strings.ContainsRune("|/\\[]{}()<>", right) {
			return right, true
		}
		if right == '_' && strings.ContainsRune("|/\\[]{}()<>", left) {
			return left, true
		}
	}
	if builder.font.smushMode&4 != 0 {
		hierarchies := []struct {
			earlier string
			later   string
		}{
			{"|", "/\\[]{}()<>"},
			{"/\\", "[]{}()<>"},
			{"[]", "{}()<>"},
			{"{}", "()<>"},
			{"()", "<>"},
		}
		for _, hierarchy := range hierarchies {
			if strings.ContainsRune(hierarchy.earlier, left) && strings.ContainsRune(hierarchy.later, right) {
				return right, true
			}
			if strings.ContainsRune(hierarchy.earlier, right) && strings.ContainsRune(hierarchy.later, left) {
				return left, true
			}
		}
	}
	if builder.font.smushMode&8 != 0 {
		for _, pair := range [][2]rune{{'[', ']'}, {'{', '}'}, {'(', ')'}} {
			if left == pair[0] && right == pair[1] || left == pair[1] && right == pair[0] {
				return '|', true
			}
		}
	}
	if builder.font.smushMode&16 != 0 {
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

func (builder *figletBuilder) addCurCharRowToBufferRow(current glyph, row int) {
	left := builder.buffer[row]
	right := current.rows[row]
	if builder.direction == "right-to-left" {
		left, right = right, left
	}
	left = append([]rune(nil), left...)
	for i := 0; i < builder.maxSmush; i++ {
		index := len(left) - builder.maxSmush + i
		leftChar := rune(0)
		if index >= 0 && index < len(left) {
			leftChar = left[index]
		}
		rightChar := right[i]
		if merged, ok := builder.smushChars(leftChar, rightChar); ok && index >= 0 && index < len(left) {
			left[index] = merged
		}
	}
	builder.buffer[row] = append(left, right[builder.maxSmush:]...)
}

func (builder *figletBuilder) handleNewLine() {
	last := len(builder.blankMarkers) - 1
	if last >= 0 {
		marker := builder.blankMarkers[last]
		builder.blankMarkers = builder.blankMarkers[:last]
		builder.cutBufferAtLastBlank(marker.rows, marker.iterator)
		return
	}
	builder.cutBufferAtLastChar()
}

func (builder *figletBuilder) cutBufferAtLastBlank(rows [][]rune, iterator int) {
	builder.product = append(builder.product, rows)
	builder.iterator = iterator
	builder.cutBufferCommon()
}

func (builder *figletBuilder) cutBufferAtLastChar() {
	builder.product = append(builder.product, builder.buffer)
	builder.iterator--
	builder.cutBufferCommon()
}

func (builder *figletBuilder) cutBufferCommon() {
	builder.currentTotalWidth = 0
	builder.buffer = makeRows(builder.font.height)
	builder.blankMarkers = nil
	builder.prevCharWidth = 0
	if current, ok := builder.font.glyphs[builder.text[builder.iterator]]; ok {
		builder.maxSmush = builder.currentSmushAmount(current)
	}
}

func cloneRows(rows [][]rune) [][]rune {
	copyRows := make([][]rune, len(rows))
	for i, row := range rows {
		copyRows[i] = append([]rune(nil), row...)
	}
	return copyRows
}

func reverseRows(value string) string {
	rows := outputRows(value)
	for i := range rows {
		row := []rune(rows[i])
		for left, right := 0, len(row)-1; left < right; left, right = left+1, right-1 {
			row[left], row[right] = row[right], row[left]
		}
		for i, char := range row {
			row[i] = translate(char, reverseTable)
		}
		rows[i] = string(row)
	}
	return strings.Join(rows, "\n") + "\n"
}

func flipRows(value string) string {
	rows := outputRows(value)
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
	for rowIndex := range rows {
		row := []rune(rows[rowIndex])
		for i, char := range row {
			row[i] = translate(char, flipTable)
		}
		rows[rowIndex] = string(row)
	}
	return strings.Join(rows, "\n") + "\n"
}

func outputRows(value string) []string {
	if value == "" {
		return nil
	}
	rows := strings.Split(value, "\n")
	if strings.HasSuffix(value, "\n") {
		rows = rows[:len(rows)-1]
	}
	return rows
}

func translate(char rune, table []byte) rune {
	if char >= 0 && char < rune(len(table)) {
		return rune(table[char])
	}
	return char
}

func stripSurroundingNewlines(value string) string {
	var rows []string
	seen := false
	for _, row := range outputRows(value) {
		if strings.TrimSpace(row) != "" || seen {
			seen = true
			rows = append(rows, row)
		}
	}
	return strings.TrimRightFunc(strings.Join(rows, "\n")+"\n", unicode.IsSpace)
}

var reverseTable = []byte(
	"\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
		" !\"#$%&')(*+,-.\\" +
		"0123456789:;>=<?" +
		"@ABCDEFGHIJKLMNO" +
		"PQRSTUVWXYZ]/[^_" +
		"`abcdefghijklmno" +
		"pqrstuvwxyz}|{~\x7f" +
		"\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f" +
		"\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f" +
		"\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf" +
		"\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf" +
		"\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf" +
		"\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf" +
		"\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef" +
		"\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff",
)

var flipTable = []byte(
	"\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
		" !\"#$%&'()*+,-.\\" +
		"0123456789:;<=>?" +
		"@VBCDEFGHIJKLWNO" +
		"bQbSTUAMXYZ[/]v-" +
		"`aPcdefghijklwno" +
		"pqrstu^mxyz{|}~\x7f" +
		"\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f" +
		"\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f" +
		"\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf" +
		"\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf" +
		"\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf" +
		"\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf" +
		"\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef" +
		"\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff",
)
GO_SOURCE

find /workspace/repo -type f -name '*.py' -delete
gofmt -w main.go
go build -o /tmp/figlet .
