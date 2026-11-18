package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:embed fonts/*
var fontFiles embed.FS

const (
	smushEqual     = 1
	smushLowline   = 2
	smushHierarchy = 4
	smushPair      = 8
	smushBigX      = 16
	smushHardblank = 32
	smushKern      = 64
	smushSmush     = 128
)

type figFont struct {
	hardblank         rune
	height            int
	printDirection    int
	hasPrintDirection bool
	smushMode         int
	chars             map[rune][]string
	widths            map[rune]int
}

type blankMarker struct {
	buffer   []string
	iterator int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	fontName := flag.String("f", "standard", "font")
	flag.StringVar(fontName, "font", "standard", "font")
	width := flag.Int("w", 80, "terminal width")
	flag.IntVar(width, "width", 80, "terminal width")
	justify := flag.String("j", "auto", "justification")
	flag.StringVar(justify, "justify", "auto", "justification")
	direction := flag.String("D", "auto", "text direction")
	flag.StringVar(direction, "direction", "auto", "text direction")
	reverse := flag.Bool("r", false, "mirror output")
	flag.BoolVar(reverse, "reverse", false, "mirror output")
	flip := flag.Bool("F", false, "flip output")
	flag.BoolVar(flip, "flip", false, "flip output")
	strip := flag.Bool("s", false, "strip surrounding newlines")
	flag.BoolVar(strip, "strip-surrounding-newlines", false, "strip surrounding newlines")
	normalize := flag.Bool("n", false, "normalize surrounding newlines")
	flag.BoolVar(normalize, "normalize-surrounding-newlines", false, "normalize surrounding newlines")
	flag.Parse()

	if flag.NArg() == 0 {
		return errors.New("text is required")
	}
	if *width < 1 {
		return fmt.Errorf("invalid width %d", *width)
	}
	if *justify != "auto" && *justify != "left" && *justify != "center" && *justify != "right" {
		return fmt.Errorf("invalid justification %q", *justify)
	}
	if *direction != "auto" && *direction != "left-to-right" && *direction != "right-to-left" {
		return fmt.Errorf("invalid direction %q", *direction)
	}

	font, err := loadFont(*fontName)
	if err != nil {
		return err
	}
	resolvedDirection := *direction
	if resolvedDirection == "auto" {
		resolvedDirection = "left-to-right"
		if font.hasPrintDirection && font.printDirection == 1 {
			resolvedDirection = "right-to-left"
		}
	}
	resolvedJustify := *justify
	if resolvedJustify == "auto" {
		resolvedJustify = "left"
		if resolvedDirection == "right-to-left" {
			resolvedJustify = "right"
		}
	}

	rendered, err := render(strings.Join(flag.Args(), " "), font, *width, resolvedDirection, resolvedJustify)
	if err != nil {
		return err
	}
	if *reverse {
		rendered = reverseArt(rendered)
	}
	if *flip {
		rendered = flipArt(rendered)
	}
	if *strip {
		rendered = stripSurroundingNewlines(rendered)
	} else if *normalize {
		rendered = "\n" + stripSurroundingNewlines(rendered) + "\n"
	}
	if _, err := os.Stdout.WriteString(rendered); err != nil {
		return err
	}
	_, err = os.Stdout.WriteString("\n")
	return err
}

func loadFont(name string) (*figFont, error) {
	stem := strings.TrimSuffix(strings.TrimSuffix(name, ".flf"), ".tlf")
	for _, extension := range []string{".tlf", ".flf"} {
		data, err := fontFiles.ReadFile("fonts/" + stem + extension)
		if err == nil {
			return parseFont(data)
		}
	}
	return nil, fmt.Errorf("font %q not found", name)
}

func parseFont(data []byte) (*figFont, error) {
	text := strings.ToValidUTF8(string(data), "\uFFFD")
	text = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(text)
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if len(lines) == 0 {
		return nil, errors.New("empty font")
	}
	header := lines[0]
	if len(header) < 5 || (header[:4] != "flf2" && header[:4] != "tlf2") {
		return nil, errors.New("invalid FIGlet font header")
	}
	fields := strings.Fields(header[5:])
	if len(fields) < 6 {
		return nil, errors.New("malformed FIGlet font header")
	}
	hardblanks := []rune(fields[0])
	if len(hardblanks) == 0 {
		return nil, errors.New("font hardblank is missing")
	}
	values := make([]int, 5)
	for i := range values {
		value, err := strconv.Atoi(fields[i+1])
		if err != nil {
			return nil, fmt.Errorf("invalid FIGlet header number %q: %w", fields[i+1], err)
		}
		values[i] = value
	}
	height, oldLayout, commentLines := values[0], values[3], values[4]
	if height < 1 || commentLines < 0 {
		return nil, errors.New("invalid FIGlet font dimensions")
	}

	font := &figFont{
		hardblank: hardblanks[0],
		height:    height,
		chars:     make(map[rune][]string),
		widths:    make(map[rune]int),
	}
	if len(fields) > 6 {
		font.printDirection, _ = strconv.Atoi(fields[6])
		font.hasPrintDirection = true
	}
	if len(fields) > 7 {
		font.smushMode, _ = strconv.Atoi(fields[7])
	} else if oldLayout == 0 {
		font.smushMode = 64
	} else if oldLayout < 0 {
		font.smushMode = 0
	} else {
		font.smushMode = (oldLayout & 31) | smushSmush
	}

	index := 1
	if index+commentLines > len(lines) {
		return nil, errors.New("font comments exceed available lines")
	}
	index += commentLines
	readGlyph := func() ([]string, int, error) {
		if index+height > len(lines) {
			return nil, 0, errors.New("font ended inside a glyph")
		}
		endMarker, ok := lastNonSpaceRune(lines[index])
		if !ok {
			return nil, 0, errors.New("glyph has no end marker")
		}
		rows := make([]string, height)
		maxWidth := 0
		for row := 0; row < height; row++ {
			line := []rune(lines[index])
			index++
			end := len(line)
			for end > 0 && unicode.IsSpace(line[end-1]) {
				end--
			}
			markerCount := 0
			for end > 0 && markerCount < 2 && line[end-1] == endMarker {
				end--
				markerCount++
			}
			if markerCount == 0 {
				return nil, 0, errors.New("glyph row is missing its end marker")
			}
			rows[row] = string(line[:end])
			if len(line[:end]) > maxWidth {
				maxWidth = len(line[:end])
			}
		}
		return rows, maxWidth, nil
	}
	addGlyph := func(code rune, rows []string, width int) {
		if code == ' ' || strings.Join(rows, "") != "" {
			font.chars[code] = rows
			font.widths[code] = width
		}
	}

	for code := rune(32); code < 127; code++ {
		rows, glyphWidth, err := readGlyph()
		if err != nil {
			return nil, err
		}
		addGlyph(code, rows, glyphWidth)
	}
	if index < len(lines) {
		for _, code := range []rune("ÄÖÜäöüß") {
			if index >= len(lines) {
				break
			}
			rows, glyphWidth, err := readGlyph()
			if err != nil {
				return nil, err
			}
			addGlyph(code, rows, glyphWidth)
		}
	}
	for index < len(lines) {
		line := strings.TrimSpace(lines[index])
		index++
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(strings.ToLower(fields[0]), "0x") {
			continue
		}
		code, err := strconv.ParseInt(fields[0], 0, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid extended character code %q: %w", fields[0], err)
		}
		rows, glyphWidth, err := readGlyph()
		if err != nil {
			return nil, err
		}
		addGlyph(rune(code), rows, glyphWidth)
	}
	return font, nil
}

func lastNonSpaceRune(text string) (rune, bool) {
	runes := []rune(text)
	for i := len(runes) - 1; i >= 0; i-- {
		if !unicode.IsSpace(runes[i]) {
			return runes[i], true
		}
	}
	return 0, false
}

func render(text string, font *figFont, width int, direction, justify string) (string, error) {
	input := []rune(text)
	buffer := make([]string, font.height)
	product := make([][]string, 0)
	blankMarkers := make([]blankMarker, 0)
	iterator, previousWidth, currentWidth := 0, 0, 0
	maxSmush := 0
	flush := func(rows []string) {
		product = append(product, append([]string(nil), rows...))
	}
	resetBuffer := func() {
		buffer = make([]string, font.height)
		blankMarkers = nil
		previousWidth = 0
	}
	smushAmount := func(current []string, curWidth int) int {
		if font.smushMode&(smushSmush|smushKern) == 0 {
			return 0
		}
		maximum := curWidth
		for row := 0; row < font.height; row++ {
			left := []rune(buffer[row])
			right := []rune(current[row])
			if direction == "right-to-left" {
				left, right = right, left
			}
			lineBoundary := len([]rune(strings.TrimRight(string(left), " "))) - 1
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
			}
			amount := charBoundary + len(left) - 1 - lineBoundary
			if leftChar == 0 || leftChar == ' ' {
				amount++
			} else if rightChar != 0 {
				if _, ok := smushChar(leftChar, rightChar, font, direction, previousWidth, curWidth); ok {
					amount++
				}
			}
			if amount < maximum {
				maximum = amount
			}
		}
		return maximum
	}
	smushRow := func(current []string, row, overlap int) (string, error) {
		left, right := []rune(buffer[row]), []rune(current[row])
		if direction == "right-to-left" {
			left, right = right, left
		}
		for i := 0; i < overlap; i++ {
			index := len(left) - overlap + i
			leftChar := rune(0)
			if index >= 0 && index < len(left) {
				leftChar = left[index]
			}
			if i >= len(right) {
				continue
			}
			combined, ok := smushChar(leftChar, right[i], font, direction, previousWidth, currentWidth)
			if !ok {
				return "", errors.New("FIGlet overlap is not smushable")
			}
			if index >= 0 && index < len(left) {
				left[index] = combined
			}
		}
		if overlap > len(right) {
			overlap = len(right)
		}
		return string(left) + string(right[overlap:]), nil
	}

	for iterator < len(input) {
		code := input[iterator]
		if code == '\n' {
			blankMarkers = append(blankMarkers, blankMarker{
				buffer: append([]string(nil), buffer...), iterator: iterator,
			})
			if len(blankMarkers) != 0 {
				marker := blankMarkers[len(blankMarkers)-1]
				blankMarkers = blankMarkers[:len(blankMarkers)-1]
				flush(marker.buffer)
				iterator = marker.iterator
				resetBuffer()
			}
			iterator++
			continue
		}
		current, found := font.chars[code]
		if !found {
			iterator++
			continue
		}
		currentWidth = font.widths[code]
		if width < currentWidth {
			return "", fmt.Errorf("width is not enough to print character %q", code)
		}
		maxSmush = smushAmount(current, currentWidth)
		totalWidth := runeCount(buffer[0]) + currentWidth - maxSmush
		if code == ' ' {
			blankMarkers = append(blankMarkers, blankMarker{
				buffer: append([]string(nil), buffer...), iterator: iterator,
			})
		}
		if totalWidth >= width {
			if len(blankMarkers) != 0 {
				marker := blankMarkers[len(blankMarkers)-1]
				blankMarkers = blankMarkers[:len(blankMarkers)-1]
				flush(marker.buffer)
				iterator = marker.iterator
				resetBuffer()
			} else {
				flush(buffer)
				iterator--
				resetBuffer()
			}
		} else {
			for row := 0; row < font.height; row++ {
				combined, err := smushRow(current, row, maxSmush)
				if err != nil {
					return "", err
				}
				buffer[row] = combined
			}
			previousWidth = currentWidth
		}
		iterator++
	}

	if buffer[0] != "" {
		flush(buffer)
	}
	var output strings.Builder
	for _, rows := range product {
		for row, line := range rows {
			switch justify {
			case "right":
				padding := width - runeCount(line) - 1
				if padding > 0 {
					line = strings.Repeat(" ", padding) + line
				}
			case "center":
				padding := (width - runeCount(line)) / 2
				if padding > 0 {
					line = strings.Repeat(" ", padding) + line
				}
			}
			line = strings.ReplaceAll(line, string(font.hardblank), " ")
			output.WriteString(line)
			if row < len(rows)-1 {
				output.WriteByte('\n')
			}
		}
		output.WriteByte('\n')
	}
	return output.String(), nil
}

func smushChar(left, right rune, font *figFont, direction string, previousWidth, currentWidth int) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if previousWidth < 2 || currentWidth < 2 {
		return 0, false
	}
	if font.smushMode&smushSmush == 0 {
		return 0, false
	}
	if font.smushMode&63 == 0 {
		if left == font.hardblank {
			return right, true
		}
		if right == font.hardblank {
			return left, true
		}
		if direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if font.smushMode&smushHardblank != 0 && left == font.hardblank && right == font.hardblank {
		return left, true
	}
	if left == font.hardblank || right == font.hardblank {
		return 0, false
	}
	if font.smushMode&smushEqual != 0 && left == right {
		return left, true
	}
	if font.smushMode&smushLowline != 0 {
		hierarchy := "|/\\[]{}()<>"
		if left == '_' && strings.ContainsRune(hierarchy, right) {
			return right, true
		}
		if right == '_' && strings.ContainsRune(hierarchy, left) {
			return left, true
		}
	}
	if font.smushMode&smushHierarchy != 0 {
		rank := func(char rune) int {
			for i, tier := range []string{"|", "/\\", "[]", "{}", "()", "<>"} {
				if strings.ContainsRune(tier, char) {
					return i
				}
			}
			return -1
		}
		leftRank, rightRank := rank(left), rank(right)
		if leftRank >= 0 && rightRank >= 0 && leftRank != rightRank {
			if leftRank < rightRank {
				return right, true
			}
			return left, true
		}
	}
	if font.smushMode&smushPair != 0 {
		if (left == '[' && right == ']') || (left == ']' && right == '[') ||
			(left == '{' && right == '}') || (left == '}' && right == '{') ||
			(left == '(' && right == ')') || (left == ')' && right == '(') {
			return '|', true
		}
	}
	if font.smushMode&smushBigX != 0 {
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

func runeCount(text string) int {
	return utf8.RuneCountInString(text)
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func reverseArt(text string) string {
	lines := splitLines(text)
	for i, line := range lines {
		runes := []rune(line)
		for j := range runes {
			switch runes[j] {
			case '(':
				runes[j] = ')'
			case ')':
				runes[j] = '('
			case '/':
				runes[j] = '\\'
			case '\\':
				runes[j] = '/'
			case '<':
				runes[j] = '>'
			case '>':
				runes[j] = '<'
			case '[':
				runes[j] = ']'
			case ']':
				runes[j] = '['
			case '{':
				runes[j] = '}'
			case '}':
				runes[j] = '{'
			}
		}
		for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
			runes[left], runes[right] = runes[right], runes[left]
		}
		lines[i] = string(runes)
	}
	return strings.Join(lines, "\n") + "\n"
}

func flipArt(text string) string {
	lines := splitLines(text)
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	flipTable := []rune("\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
		" !\"#$%&'()*+,-.\\" +
		"0123456789:;<=>?" +
		"@VBCDEFGHIJKLWNO" +
		"bQbSTUAMXYZ[/]v-" +
		"`aPcdefghijklwno" +
		"pqrstu^mxyz{|}~\x7f")
	for i, line := range lines {
		runes := []rune(line)
		for j, char := range runes {
			if char <= 0x7f {
				runes[j] = flipTable[char]
			}
		}
		lines[i] = string(runes)
	}
	return strings.Join(lines, "\n") + "\n"
}

func stripSurroundingNewlines(text string) string {
	lines := splitLines(text)
	output := make([]string, 0, len(lines))
	charactersSeen := false
	for _, line := range lines {
		if strings.TrimSpace(line) != "" || charactersSeen {
			charactersSeen = true
			output = append(output, line)
		}
	}
	return strings.TrimRightFunc(strings.Join(output, "\n"), unicode.IsSpace)
}
