package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode"
)

//go:embed pyfiglet/fonts/*
var bundledFonts embed.FS

type figFont struct {
	hardBlank      rune
	height         int
	printDirection int
	smushMode      int
	glyphs         map[rune][][]rune
	widths         map[rune]int
}

type blankMark struct {
	buffer   [][]rune
	iterator int
}

type builder struct {
	text          []rune
	font          *figFont
	direction     string
	justify       string
	width         int
	iterator      int
	maxSmush      int
	currentCharW  int
	previousCharW int
	currentTotalW int
	buffer        [][]rune
	blankMarkers  []blankMark
	product       [][][]rune
}

func main() {
	opts, text, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	font, err := loadFont(opts.font)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	direction := opts.direction
	if direction == "auto" {
		if font.printDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}
	justify := opts.justify
	if justify == "auto" {
		justify = "left"
		if direction == "right-to-left" {
			justify = "right"
		}
	}

	rendered, err := render(text, font, direction, justify, opts.width)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.reverse {
		rendered = transformRows(rendered, reverseMap, true)
	}
	if opts.flip {
		rendered = transformRows(rendered, flipMap, false)
	}
	if opts.strip {
		rendered = stripSurroundingNewlines(rendered)
	} else if opts.normalize {
		rendered = "\n" + stripSurroundingNewlines(rendered) + "\n"
	}
	_, _ = os.Stdout.WriteString(rendered + "\n")
}

type options struct {
	font      string
	direction string
	justify   string
	width     int
	reverse   bool
	flip      bool
	strip     bool
	normalize bool
}

func parseArgs(args []string) (options, string, error) {
	opts := options{
		font:      "standard",
		direction: "auto",
		justify:   "auto",
		width:     80,
	}
	var textArgs []string
	parsingOptions := true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if parsingOptions && arg == "--" {
			parsingOptions = false
			continue
		}
		if !parsingOptions || len(arg) < 2 || arg[0] != '-' {
			textArgs = append(textArgs, arg)
			continue
		}

		name, value, hasValue := strings.Cut(arg, "=")
		if name == "-f" || name == "-w" || name == "-j" || name == "-D" ||
			name == "--font" || name == "--width" || name == "--justify" ||
			name == "--direction" {
			if !hasValue {
				if len(name) == 2 && len(arg) > 2 {
					value = arg[2:]
					hasValue = true
				} else {
					i++
					if i >= len(args) {
						return opts, "", fmt.Errorf("option %s requires a value", name)
					}
					value = args[i]
				}
			}
			switch name {
			case "-f", "--font":
				opts.font = value
			case "-w", "--width":
				width, err := strconv.Atoi(value)
				if err != nil {
					return opts, "", fmt.Errorf("invalid width %q", value)
				}
				opts.width = width
			case "-j", "--justify":
				if value != "auto" && value != "left" && value != "center" && value != "right" {
					return opts, "", fmt.Errorf("invalid justification %q", value)
				}
				opts.justify = value
			case "-D", "--direction":
				if value != "auto" && value != "left-to-right" && value != "right-to-left" {
					return opts, "", fmt.Errorf("invalid direction %q", value)
				}
				opts.direction = value
			}
			continue
		}
		switch arg {
		case "-r", "--reverse":
			opts.reverse = true
		case "-F", "--flip":
			opts.flip = true
		case "-s", "--strip-surrounding-newlines":
			opts.strip = true
		case "-n", "--normalize-surrounding-newlines":
			opts.normalize = true
		default:
			if strings.HasPrefix(arg, "-f") && len(arg) > 2 {
				opts.font = arg[2:]
			} else if strings.HasPrefix(arg, "-w") && len(arg) > 2 {
				width, err := strconv.Atoi(arg[2:])
				if err != nil {
					return opts, "", fmt.Errorf("invalid width %q", arg[2:])
				}
				opts.width = width
			} else {
				return opts, "", fmt.Errorf("unknown option %q", arg)
			}
		}
	}
	if len(textArgs) == 0 {
		return opts, "", errors.New("text is required")
	}
	return opts, strings.Join(textArgs, " "), nil
}

func loadFont(name string) (*figFont, error) {
	for _, extension := range []string{"tlf", "flf"} {
		fontPath := path.Join("pyfiglet", "fonts", name+"."+extension)
		data, err := bundledFonts.ReadFile(fontPath)
		if err == nil {
			return parseFont(string(data))
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read font %q: %w", name, err)
		}
	}
	return nil, fmt.Errorf("font %q not found", name)
}

func parseFont(data string) (*figFont, error) {
	data = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(data)
	lines := strings.Split(data, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if len(lines) == 0 {
		return nil, errors.New("empty font")
	}
	header := lines[0]
	if !(strings.HasPrefix(header, "flf2") || strings.HasPrefix(header, "tlf2")) || len(header) < 5 {
		return nil, errors.New("invalid FIGlet font header")
	}
	fields := strings.Fields(header[5:])
	if len(fields) < 6 {
		return nil, errors.New("malformed FIGlet font header")
	}
	hardBlankRunes := []rune(fields[0])
	if len(hardBlankRunes) == 0 {
		return nil, errors.New("font has no hardblank")
	}
	height, err := strconv.Atoi(fields[1])
	if err != nil || height < 1 {
		return nil, errors.New("invalid font height")
	}
	oldLayout, err := strconv.Atoi(fields[4])
	if err != nil {
		return nil, errors.New("invalid old layout value")
	}
	commentLines, err := strconv.Atoi(fields[5])
	if err != nil || commentLines < 0 {
		return nil, errors.New("invalid comment line count")
	}

	font := &figFont{
		hardBlank:      hardBlankRunes[0],
		height:         height,
		printDirection: -1,
		glyphs:         make(map[rune][][]rune),
		widths:         make(map[rune]int),
	}
	if len(fields) > 6 {
		font.printDirection, err = strconv.Atoi(fields[6])
		if err != nil {
			return nil, errors.New("invalid print direction")
		}
	}
	if len(fields) > 7 {
		font.smushMode, err = strconv.Atoi(fields[7])
		if err != nil {
			return nil, errors.New("invalid full layout value")
		}
	} else if oldLayout == 0 {
		font.smushMode = 64
	} else if oldLayout < 0 {
		font.smushMode = 0
	} else {
		font.smushMode = (oldLayout & 31) | 128
	}

	position := 1
	if commentLines > len(lines)-position {
		return nil, errors.New("font has fewer comment lines than declared")
	}
	position += commentLines

	readGlyph := func() (int, [][]rune, error) {
		if position+height > len(lines) {
			return 0, nil, errors.New("font ends in a glyph")
		}
		endLine := []rune(lines[position])
		if len(endLine) == 0 {
			return 0, nil, errors.New("glyph has no end marker")
		}
		markerIndex := len(endLine) - 1
		for markerIndex > 0 && unicode.IsSpace(endLine[markerIndex]) {
			markerIndex--
		}
		marker := endLine[markerIndex]
		if markerIndex == 0 && unicode.IsSpace(marker) {
			marker = endLine[0]
		}
		rows := make([][]rune, height)
		width := 0
		for row := 0; row < height; row++ {
			line := []rune(strings.TrimRightFunc(lines[position], unicode.IsSpace))
			position++
			remove := 0
			for len(line) > 0 && line[len(line)-1] == marker && remove < 2 {
				line = line[:len(line)-1]
				remove++
			}
			rows[row] = line
			if len(line) > width {
				width = len(line)
			}
		}
		return width, rows, nil
	}

	for code := rune(32); code <= 126; code++ {
		width, rows, err := readGlyph()
		if err != nil {
			return nil, fmt.Errorf("load character %d: %w", code, err)
		}
		if code == 32 || !emptyGlyph(rows) {
			font.glyphs[code] = rows
			font.widths[code] = width
		}
	}

	if position < len(lines) {
		for _, code := range []rune("ÄÖÜäöüß") {
			if position >= len(lines) {
				return nil, errors.New("font ends in the umlaut glyphs")
			}
			width, rows, err := readGlyph()
			if err != nil {
				return nil, fmt.Errorf("load character %U: %w", code, err)
			}
			if !emptyGlyph(rows) {
				font.glyphs[code] = rows
				font.widths[code] = width
			}
		}
	}

	for position < len(lines) {
		line := strings.TrimSpace(lines[position])
		position++
		if line == "" {
			continue
		}
		codeText := strings.SplitN(line, " ", 2)[0]
		if !strings.HasPrefix(strings.ToLower(codeText), "0x") {
			continue
		}
		code, err := strconv.ParseInt(codeText[2:], 16, 32)
		if err != nil {
			continue
		}
		width, rows, err := readGlyph()
		if err != nil {
			return nil, fmt.Errorf("load extended character %s: %w", codeText, err)
		}
		if !emptyGlyph(rows) {
			font.glyphs[rune(code)] = rows
			font.widths[rune(code)] = width
		}
	}
	return font, nil
}

func emptyGlyph(rows [][]rune) bool {
	for _, row := range rows {
		if len(row) != 0 {
			return false
		}
	}
	return true
}

func render(text string, font *figFont, direction, justify string, width int) (string, error) {
	b := &builder{
		text:      []rune(text),
		font:      font,
		direction: direction,
		justify:   justify,
		width:     width,
		buffer:    make([][]rune, font.height),
	}
	for i := range b.buffer {
		b.buffer[i] = []rune{}
	}
	for b.iterator < len(b.text) {
		if err := b.addChar(); err != nil {
			return "", err
		}
		b.iterator++
	}
	if len(b.buffer) > 0 && len(b.buffer[0]) > 0 {
		b.product = append(b.product, cloneBlock(b.buffer))
	}

	var output strings.Builder
	for _, block := range b.product {
		for row := 0; row < font.height; row++ {
			line := block[row]
			switch justify {
			case "right":
				line = prependSpaces(line, width-len(line)-1)
			case "center":
				line = prependSpaces(line, (width-len(line))/2)
			}
			line = replaceRune(line, font.hardBlank, ' ')
			output.WriteString(string(line))
			output.WriteByte('\n')
		}
	}
	return output.String(), nil
}

func (b *builder) addChar() error {
	current := b.text[b.iterator]
	if current == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMark{
			buffer:   cloneBlock(b.buffer),
			iterator: b.iterator,
		})
		b.handleNewline()
		return nil
	}

	char, ok := b.font.glyphs[current]
	if !ok {
		return nil
	}
	b.currentCharW = b.font.widths[current]
	if b.width < b.currentCharW {
		return fmt.Errorf("width is not enough to print character %q", current)
	}
	b.maxSmush = b.smushAmount(b.buffer, char)
	b.currentTotalW = len(b.buffer[0]) + b.currentCharW - b.maxSmush

	if current == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMark{
			buffer:   cloneBlock(b.buffer),
			iterator: b.iterator,
		})
	}
	if b.currentTotalW >= b.width {
		b.handleNewline()
	} else {
		for row := 0; row < b.font.height; row++ {
			if err := b.addCharRow(char, row); err != nil {
				return err
			}
		}
	}
	b.previousCharW = b.currentCharW
	return nil
}

func (b *builder) addCharRow(char [][]rune, row int) error {
	left := b.buffer[row]
	right := char[row]
	if b.direction == "right-to-left" {
		left, right = append([]rune(nil), right...), left
	}
	for i := 0; i < b.maxSmush; i++ {
		index := len(left) - b.maxSmush + i
		var leftChar rune
		if index >= 0 && index < len(left) {
			leftChar = left[index]
		}
		if i >= len(right) {
			return errors.New("font row is shorter than its smush overlap")
		}
		smushed, ok := b.smushChar(leftChar, right[i])
		if index >= 0 && index < len(left) {
			if !ok {
				return errors.New("font requested a non-smushable overlap")
			}
			left[index] = smushed
		}
	}
	b.buffer[row] = append(left, right[min(b.maxSmush, len(right)):]...)
	return nil
}

func (b *builder) smushAmount(left, right [][]rune) int {
	if b.font.smushMode&(64|128) == 0 {
		return 0
	}
	maximum := b.currentCharW
	for row := 0; row < b.font.height; row++ {
		lineLeft, lineRight := left[row], right[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}
		lineBoundary := len(trimRightSpaces(lineLeft)) - 1
		if lineBoundary < 0 {
			lineBoundary = 0
		}
		var char1 rune
		if lineBoundary < len(lineLeft) {
			char1 = lineLeft[lineBoundary]
		} else {
			lineBoundary = 0
		}
		charBoundary := leadingSpaces(lineRight)
		var char2 rune
		if charBoundary < len(lineRight) {
			char2 = lineRight[charBoundary]
		} else {
			charBoundary = len(lineRight)
		}
		amount := charBoundary + len(lineLeft) - 1 - lineBoundary
		if char1 == 0 || char1 == ' ' {
			amount++
		} else if char2 != 0 {
			if _, ok := b.smushChar(char1, char2); ok {
				amount++
			}
		}
		if amount < maximum {
			maximum = amount
		}
	}
	return maximum
}

func (b *builder) smushChar(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.previousCharW < 2 || b.currentCharW < 2 {
		return 0, false
	}
	if b.font.smushMode&128 == 0 {
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
	if b.font.smushMode&32 != 0 && left == b.font.hardBlank && right == b.font.hardBlank {
		return left, true
	}
	if left == b.font.hardBlank || right == b.font.hardBlank {
		return 0, false
	}
	if b.font.smushMode&1 != 0 && left == right {
		return left, true
	}
	if b.font.smushMode&2 != 0 && ((left == '_' && strings.ContainsRune("|/\\[]{}()<>", right)) ||
		(right == '_' && strings.ContainsRune("|/\\[]{}()<>", left))) {
		if right == '_' {
			return left, true
		}
		return right, true
	}
	if b.font.smushMode&4 != 0 {
		hierarchy := [][2]string{
			{"|", `/\[]{}()<>`},
			{`\/`, "[]{}()<>"},
			{"[]", "{}()<>"},
			{"{}", "()<>"},
			{"()", "<>"},
		}
		for _, pair := range hierarchy {
			if strings.ContainsRune(pair[0], left) && strings.ContainsRune(pair[1], right) {
				return right, true
			}
			if strings.ContainsRune(pair[0], right) && strings.ContainsRune(pair[1], left) {
				return left, true
			}
		}
	}
	if b.font.smushMode&8 != 0 {
		for _, pair := range []string{string([]rune{left, right}), string([]rune{right, left})} {
			if pair == "[]" || pair == "{}" || pair == "()" {
				return '|', true
			}
		}
	}
	if b.font.smushMode&16 != 0 {
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

func (b *builder) handleNewline() {
	if len(b.blankMarkers) > 0 {
		last := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.product = append(b.product, last.buffer)
		b.iterator = last.iterator
	} else {
		b.product = append(b.product, cloneBlock(b.buffer))
		b.iterator--
	}
	b.currentTotalW = 0
	b.buffer = make([][]rune, b.font.height)
	for i := range b.buffer {
		b.buffer[i] = []rune{}
	}
	b.blankMarkers = nil
	b.previousCharW = 0
	if b.iterator >= 0 && b.iterator < len(b.text) {
		if char, ok := b.font.glyphs[b.text[b.iterator]]; ok {
			b.maxSmush = b.smushAmount(b.buffer, char)
		}
	}
}

func transformRows(text, mapping string, reverse bool) string {
	rows := splitOutputLines(text)
	if reverse {
		for i := range rows {
			rows[i] = reverseRunes(translate(rows[i], mapping))
		}
	} else {
		for i := range rows {
			rows[i] = translate(rows[i], mapping)
		}
		for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
			rows[left], rows[right] = rows[right], rows[left]
		}
	}
	if len(rows) == 0 {
		return "\n"
	}
	return strings.Join(rows, "\n") + "\n"
}

func splitOutputLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func translate(text, mapping string) string {
	table := []byte(mapping)
	var out strings.Builder
	for _, char := range text {
		if char >= 0 && char < rune(len(table)) {
			out.WriteRune(rune(table[char]))
		} else {
			out.WriteRune(char)
		}
	}
	return out.String()
}

func stripSurroundingNewlines(text string) string {
	var rows []string
	seen := false
	for _, row := range splitOutputLines(text) {
		if strings.TrimSpace(row) != "" || seen {
			seen = true
			rows = append(rows, row)
		}
	}
	return strings.TrimRightFunc(strings.Join(rows, "\n")+"\n", unicode.IsSpace)
}

func prependSpaces(row []rune, count int) []rune {
	if count <= 0 {
		return row
	}
	return append([]rune(strings.Repeat(" ", count)), row...)
}

func replaceRune(row []rune, from, to rune) []rune {
	out := make([]rune, len(row))
	for i, char := range row {
		if char == from {
			out[i] = to
		} else {
			out[i] = char
		}
	}
	return out
}

func cloneBlock(block [][]rune) [][]rune {
	copyBlock := make([][]rune, len(block))
	for i := range block {
		copyBlock[i] = append([]rune(nil), block[i]...)
	}
	return copyBlock
}

func trimRightSpaces(row []rune) []rune {
	end := len(row)
	for end > 0 && row[end-1] == ' ' {
		end--
	}
	return row[:end]
}

func leadingSpaces(row []rune) int {
	count := 0
	for count < len(row) && row[count] == ' ' {
		count++
	}
	return count
}

func reverseRunes(row string) string {
	runes := []rune(row)
	for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
		runes[left], runes[right] = runes[right], runes[left]
	}
	return string(runes)
}

const reverseMap = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
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
	"\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"

const flipMap = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
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
	"\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"
