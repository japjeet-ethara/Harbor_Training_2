package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

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

type figletFont struct {
	hardblank      rune
	height         int
	printDirection int
	smushMode      int
	characters     map[rune][]string
	widths         map[rune]int
}

type figletBuilder struct {
	text              []rune
	font              *figletFont
	direction         string
	width             int
	justify           string
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []blankMarker
	buffer            []string
	product           [][]string
}

type blankMarker struct {
	buffer   []string
	iterator int
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, errHelpShown) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	opts, text, err := parseOptions(args)
	if err != nil {
		return err
	}
	if len(text) == 0 {
		return errors.New("Usage: figlet [options] [text..]")
	}

	font, err := loadFont(opts.font)
	if err != nil {
		return err
	}
	if opts.direction == "auto" {
		if font.printDirection == 1 {
			opts.direction = "right-to-left"
		} else {
			opts.direction = "left-to-right"
		}
	}
	if opts.justify == "auto" {
		if opts.direction == "right-to-left" {
			opts.justify = "right"
		} else {
			opts.justify = "left"
		}
	}

	builder := figletBuilder{
		text:      []rune(strings.Join(text, " ")),
		font:      font,
		direction: opts.direction,
		width:     opts.width,
		justify:   opts.justify,
		buffer:    make([]string, font.height),
	}
	rendered, err := builder.render()
	if err != nil {
		return err
	}
	if opts.reverse {
		rendered = reverseFiglet(rendered)
	}
	if opts.flip {
		rendered = flipFiglet(rendered)
	}
	if opts.strip {
		rendered = stripSurroundingNewlines(rendered)
	} else if opts.normalize {
		rendered = "\n" + stripSurroundingNewlines(rendered) + "\n"
	}
	fmt.Print(rendered)
	fmt.Print("\n")
	return nil
}

func parseOptions(args []string) (options, []string, error) {
	opts := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var text []string
	parseFlags := true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if parseFlags && arg == "--" {
			parseFlags = false
			continue
		}
		if !parseFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			text = append(text, arg)
			continue
		}

		name, value, hasValue := strings.Cut(arg, "=")
		if !hasValue {
			switch name {
			case "-r", "--reverse":
				opts.reverse = true
				continue
			case "-F", "--flip":
				opts.flip = true
				continue
			case "-s", "--strip-surrounding-newlines":
				opts.strip = true
				continue
			case "-n", "--normalize-surrounding-newlines":
				opts.normalize = true
				continue
			case "-h", "--help":
				fmt.Println("Usage: figlet [options] [text..]")
				return opts, nil, errHelpShown
			}
		}

		switch name {
		case "-f", "--font", "-D", "--direction", "-j", "--justify", "-w", "--width":
			if !hasValue {
				if i+1 >= len(args) {
					return opts, nil, fmt.Errorf("option %s requires a value", name)
				}
				i++
				value = args[i]
			}
			switch name {
			case "-f", "--font":
				opts.font = value
			case "-D", "--direction":
				opts.direction = value
			case "-j", "--justify":
				opts.justify = value
			case "-w", "--width":
				width, err := strconv.Atoi(value)
				if err != nil {
					return opts, nil, fmt.Errorf("invalid width %q", value)
				}
				opts.width = width
			}
		default:
			return opts, nil, fmt.Errorf("unknown option: %s", name)
		}
	}

	if opts.width <= 0 {
		return opts, nil, errors.New("width must be positive")
	}
	if opts.direction != "auto" && opts.direction != "left-to-right" && opts.direction != "right-to-left" {
		return opts, nil, fmt.Errorf("invalid direction: %s", opts.direction)
	}
	if opts.justify != "auto" && opts.justify != "left" && opts.justify != "center" && opts.justify != "right" {
		return opts, nil, fmt.Errorf("invalid justification: %s", opts.justify)
	}
	return opts, text, nil
}

var errHelpShown = errors.New("")

func locateFont(name string) (string, error) {
	direct := []string{name}
	if filepath.Ext(name) == "" {
		direct = []string{name + ".tlf", name + ".flf"}
	}
	for _, path := range direct {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	directories := []string{}
	if fontDir := os.Getenv("FIGLET_FONT_DIR"); fontDir != "" {
		directories = append(directories, fontDir)
	}
	if cwd, err := os.Getwd(); err == nil {
		directories = append(directories,
			filepath.Join(cwd, "fonts"),
			filepath.Join(cwd, "pyfiglet", "fonts"),
			filepath.Join(cwd, "pyfiglet", "fonts-standard"),
		)
	}
	if executable, err := os.Executable(); err == nil {
		directories = append(directories, filepath.Join(filepath.Dir(executable), "fonts"))
	}
	directories = append(directories,
		"/workspace/repo/fonts",
		"/workspace/repo/pyfiglet/fonts",
	)

	for _, directory := range directories {
		for _, path := range direct {
			candidate := filepath.Join(directory, filepath.Base(path))
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("font not found: %s", name)
}

func loadFont(name string) (*figletFont, error) {
	path, err := locateFont(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseFont(string(data), name)
}

func parseFont(data, name string) (*figletFont, error) {
	data = strings.NewReplacer("\r\n", "\n", "\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(data)
	lines := strings.Split(data, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 || len(lines[0]) < 5 {
		return nil, fmt.Errorf("invalid figlet font: %s", name)
	}
	magic := lines[0][:5]
	if magic[:4] != "flf2" && magic[:4] != "tlf2" {
		return nil, fmt.Errorf("invalid figlet font: %s", name)
	}
	fields := strings.Fields(lines[0][5:])
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed font header: %s", name)
	}
	hardblankRunes := []rune(fields[0])
	if len(hardblankRunes) == 0 {
		return nil, fmt.Errorf("malformed font header: %s", name)
	}
	height, err := strconv.Atoi(fields[1])
	if err != nil || height <= 0 {
		return nil, fmt.Errorf("invalid font height: %s", name)
	}
	oldLayout, err := strconv.Atoi(fields[4])
	if err != nil {
		return nil, fmt.Errorf("invalid font layout: %s", name)
	}
	commentLines, err := strconv.Atoi(fields[5])
	if err != nil || commentLines < 0 {
		return nil, fmt.Errorf("invalid font comments: %s", name)
	}

	printDirection := -1
	if len(fields) > 6 {
		printDirection, _ = strconv.Atoi(fields[6])
	}
	smushMode := 0
	if len(fields) > 7 {
		smushMode, _ = strconv.Atoi(fields[7])
	} else if oldLayout == 0 {
		smushMode = 64
	} else if oldLayout < 0 {
		smushMode = 0
	} else {
		smushMode = (oldLayout & 31) | 128
	}

	font := &figletFont{
		hardblank:      hardblankRunes[0],
		height:         height,
		printDirection: printDirection,
		smushMode:      smushMode,
		characters:     make(map[rune][]string),
		widths:         make(map[rune]int),
	}
	index := 1 + commentLines
	for code := rune(32); code <= 126; code++ {
		rows, next, width, err := parseGlyph(lines, index, height)
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s font: %w", name, err)
		}
		index = next
		if code == ' ' || hasVisibleGlyph(rows) {
			font.characters[code] = rows
			font.widths[code] = width
		}
	}
	if index < len(lines) {
		for _, code := range []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'} {
			rows, next, width, err := parseGlyph(lines, index, height)
			if err != nil {
				break
			}
			index = next
			if hasVisibleGlyph(rows) {
				font.characters[code] = rows
				font.widths[code] = width
			}
		}
	}
	for index < len(lines) {
		line := strings.TrimSpace(lines[index])
		index++
		if line == "" {
			continue
		}
		codeText, _, _ := strings.Cut(line, " ")
		if !strings.HasPrefix(strings.ToLower(codeText), "0x") {
			continue
		}
		code, err := strconv.ParseInt(codeText[2:], 16, 32)
		if err != nil {
			continue
		}
		rows, next, width, err := parseGlyph(lines, index, height)
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s font: %w", name, err)
		}
		index = next
		if hasVisibleGlyph(rows) {
			font.characters[rune(code)] = rows
			font.widths[rune(code)] = width
		}
	}
	return font, nil
}

func parseGlyph(lines []string, index, height int) ([]string, int, int, error) {
	if index+height > len(lines) {
		return nil, index, 0, errors.New("unexpected end of font")
	}
	rows := make([]string, 0, height)
	endMarker := rune(0)
	maxWidth := 0
	for rowIndex := 0; rowIndex < height; rowIndex++ {
		line := []rune(lines[index+rowIndex])
		end := len(line)
		for end > 0 && unicode.IsSpace(line[end-1]) {
			end--
		}
		if rowIndex == 0 {
			if end == 0 {
				return nil, index, 0, errors.New("missing glyph end marker")
			}
			endMarker = line[end-1]
		}
		for count := 0; count < 2 && end > 0 && line[end-1] == endMarker; count++ {
			end--
		}
		clean := string(line[:end])
		if width := utf8.RuneCountInString(clean); width > maxWidth {
			maxWidth = width
		}
		rows = append(rows, clean)
	}
	return rows, index + height, maxWidth, nil
}

func hasVisibleGlyph(rows []string) bool {
	for _, row := range rows {
		if row != "" {
			return true
		}
	}
	return false
}

func (b *figletBuilder) render() (string, error) {
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
		b.iterator++
	}
	if len(b.buffer) > 0 && b.buffer[0] != "" {
		b.product = append(b.product, cloneRows(b.buffer))
	}
	var output strings.Builder
	for _, buffer := range b.product {
		b.justifyBuffer(buffer)
		for _, row := range buffer {
			output.WriteString(strings.ReplaceAll(row, string(b.font.hardblank), " "))
			output.WriteByte('\n')
		}
	}
	return output.String(), nil
}

func (b *figletBuilder) addCharToProduct() error {
	code := b.text[b.iterator]
	if code == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneRows(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}
	curChar, exists := b.font.characters[code]
	if !exists {
		return nil
	}
	b.curCharWidth = b.font.widths[code]
	if b.width < b.curCharWidth {
		return fmt.Errorf("width is not enough to print character %q", code)
	}
	b.maxSmush = b.currentSmushAmount(curChar)
	b.currentTotalWidth = runeLen(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if code == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneRows(b.buffer), b.iterator})
	}
	if b.currentTotalWidth >= b.width {
		if b.buffer[0] == "" {
			return fmt.Errorf("width is not enough to print character %q", code)
		}
		b.handleNewLine()
		return nil
	}
	for row := 0; row < b.font.height; row++ {
		if err := b.addCurrentCharRow(curChar, row); err != nil {
			return err
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *figletBuilder) currentSmushAmount(curChar []string) int {
	if b.font.smushMode&(128|64) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft, lineRight := b.buffer[row], curChar[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}
		leftRunes := []rune(lineLeft)
		linebd := runeLen(strings.TrimRight(lineLeft, " ")) - 1
		if linebd < 0 {
			linebd = 0
		}
		leftChar := rune(0)
		if linebd < len(leftRunes) {
			leftChar = leftRunes[linebd]
		} else {
			linebd = 0
		}

		rightRunes := []rune(lineRight)
		charbd := 0
		for charbd < len(rightRunes) && rightRunes[charbd] == ' ' {
			charbd++
		}
		rightChar := rune(0)
		if charbd < len(rightRunes) {
			rightChar = rightRunes[charbd]
		}
		amount := charbd + runeLen(lineLeft) - 1 - linebd
		if leftChar == 0 || leftChar == ' ' {
			amount++
		} else if rightChar != 0 && b.smushChars(leftChar, rightChar) != 0 {
			amount++
		}
		if amount < maxSmush {
			maxSmush = amount
		}
	}
	return maxSmush
}

func (b *figletBuilder) smushChars(left, right rune) rune {
	if left == ' ' {
		return right
	}
	if right == ' ' {
		return left
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0
	}
	if b.font.smushMode&128 == 0 {
		return 0
	}
	if b.font.smushMode&63 == 0 {
		if left == b.font.hardblank {
			return right
		}
		if right == b.font.hardblank {
			return left
		}
		if b.direction == "right-to-left" {
			return left
		}
		return right
	}
	if b.font.smushMode&32 != 0 && left == b.font.hardblank && right == b.font.hardblank {
		return left
	}
	if left == b.font.hardblank || right == b.font.hardblank {
		return 0
	}
	if b.font.smushMode&1 != 0 && left == right {
		return left
	}
	if b.font.smushMode&2 != 0 {
		if left == '_' && strings.ContainsRune("|/\\[]{}()<>", right) {
			return right
		}
		if right == '_' && strings.ContainsRune("|/\\[]{}()<>", left) {
			return left
		}
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
				return right
			}
			if strings.ContainsRune(pair[0], right) && strings.ContainsRune(pair[1], left) {
				return left
			}
		}
	}
	if b.font.smushMode&8 != 0 {
		pair := string([]rune{left, right})
		reversePair := string([]rune{right, left})
		if pair == "[]" || pair == "{}" || pair == "()" || reversePair == "[]" || reversePair == "{}" || reversePair == "()" {
			return '|'
		}
	}
	if b.font.smushMode&16 != 0 {
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

func (b *figletBuilder) addCurrentCharRow(curChar []string, row int) error {
	left := b.buffer[row]
	right := curChar[row]
	if b.direction == "right-to-left" {
		left, right = right, left
	}
	leftRunes, rightRunes := []rune(left), []rune(right)
	for i := 0; i < b.maxSmush; i++ {
		index := len(leftRunes) - b.maxSmush + i
		leftChar := rune(0)
		if index >= 0 && index < len(leftRunes) {
			leftChar = leftRunes[index]
		}
		if i >= len(rightRunes) {
			return errors.New("malformed font glyph width")
		}
		smushed := b.smushChars(leftChar, rightRunes[i])
		if smushed != 0 && index >= 0 && index < len(leftRunes) {
			leftRunes[index] = smushed
		}
	}
	if b.maxSmush > len(rightRunes) {
		return errors.New("malformed font glyph width")
	}
	b.buffer[row] = string(leftRunes) + string(rightRunes[b.maxSmush:])
	return nil
}

func (b *figletBuilder) handleNewLine() {
	if len(b.blankMarkers) > 0 {
		marker := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.product = append(b.product, marker.buffer)
		b.iterator = marker.iterator
	} else {
		b.product = append(b.product, cloneRows(b.buffer))
		b.iterator--
	}
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
}

func (b *figletBuilder) justifyBuffer(buffer []string) {
	for i, row := range buffer {
		padding := 0
		switch b.justify {
		case "right":
			padding = b.width - runeLen(row) - 1
		case "center":
			padding = (b.width - runeLen(row)) / 2
		}
		if padding > 0 {
			buffer[i] = strings.Repeat(" ", padding) + row
		}
	}
}

func reverseFiglet(input string) string {
	rows := splitFigletRows(input)
	for i, row := range rows {
		runes := []rune(row)
		for j := range runes {
			runes[j] = reverseRune(runes[j])
		}
		for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
			runes[left], runes[right] = runes[right], runes[left]
		}
		rows[i] = string(runes)
	}
	return strings.Join(rows, "\n") + "\n"
}

func flipFiglet(input string) string {
	rows := splitFigletRows(input)
	for left, right := 0, len(rows)-1; left <= right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
	for i, row := range rows {
		runes := []rune(row)
		for j, r := range runes {
			runes[j] = flipRune(r)
		}
		rows[i] = string(runes)
	}
	return strings.Join(rows, "\n") + "\n"
}

func splitFigletRows(input string) []string {
	if strings.HasSuffix(input, "\n") {
		input = input[:len(input)-1]
	}
	return strings.Split(input, "\n")
}

func stripSurroundingNewlines(input string) string {
	rows := splitFigletRows(input)
	kept := make([]string, 0, len(rows))
	seen := false
	for _, row := range rows {
		if strings.TrimSpace(row) != "" || seen {
			seen = true
			kept = append(kept, row)
		}
	}
	return strings.TrimRightFunc(strings.Join(kept, "\n"), unicode.IsSpace)
}

func reverseRune(r rune) rune {
	switch r {
	case '(':
		return ')'
	case ')':
		return '('
	case '/':
		return '\\'
	case '\\':
		return '/'
	case '<':
		return '>'
	case '>':
		return '<'
	case '[':
		return ']'
	case ']':
		return '['
	case '{':
		return '}'
	case '}':
		return '{'
	case '^':
		return '^'
	default:
		return r
	}
}

func flipRune(r rune) rune {
	switch r {
	case '/':
		return '\\'
	case '\\':
		return '/'
	case 'A':
		return 'V'
	case 'M':
		return 'W'
	case 'P', 'R':
		return 'b'
	case 'V':
		return 'A'
	case 'W':
		return 'M'
	case '^':
		return 'v'
	case '_':
		return '-'
	case 'b':
		return 'P'
	case 'm':
		return 'w'
	case 'v':
		return '^'
	case 'w':
		return 'm'
	default:
		return r
	}
}

func cloneRows(rows []string) []string {
	return append([]string(nil), rows...)
}

func runeLen(value string) int {
	return utf8.RuneCountInString(value)
}
