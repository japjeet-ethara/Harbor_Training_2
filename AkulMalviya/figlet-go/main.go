package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed pyfiglet/fonts/*.flf
var embeddedFonts embed.FS

type Font struct {
	Height      int
	HardBlank   rune
	FullLayout  int
	PrintDir    int
	Glyphs      map[rune][]string
	GlyphWidths map[rune]int
}

type Builder struct {
	text              []rune
	font              *Font
	direction         string
	justify           string
	width             int
	iterator          int
	curCharWidth      int
	prevCharWidth     int
	maxSmush          int
	currentTotalWidth int
	buffer            []string
	blankMarkers      []savedBuffer
	product           [][]string
}

type savedBuffer struct {
	rows     []string
	iterator int
}

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

func readFontData(name string) ([]byte, error) {
	// Try embedded fonts first
	for _, ext := range []string{".flf", ".tlf"} {
		path := "pyfiglet/fonts/" + name + ext
		if data, err := embeddedFonts.ReadFile(path); err == nil {
			return data, nil
		}
	}

	// Try disk paths
	exe, err := os.Executable()
	if err != nil {
		exe = "."
	}
	exeDir := filepath.Dir(exe)

	candidates := []string{
		filepath.Join("/workspace/repo", "pyfiglet", "fonts", name+".flf"),
		filepath.Join("/workspace/repo", "fonts", name+".flf"),
		filepath.Join(exeDir, "pyfiglet", "fonts", name+".flf"),
		filepath.Join(exeDir, "fonts", name+".flf"),
		filepath.Join(".", "pyfiglet", "fonts", name+".flf"),
		filepath.Join(".", "fonts", name+".flf"),
	}

	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("font %q not found", name)
}

func loadFont(name string) (*Font, error) {
	data, err := readFontData(name)
	if err != nil {
		return nil, err
	}

	content := strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		"\u0085", " ",
		"\u2028", " ",
		"\u2029", " ",
	).Replace(string(data))

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font file")
	}

	header := strings.Fields(lines[0])
	if len(header) < 6 || len(header[0]) < 6 {
		return nil, fmt.Errorf("invalid font header")
	}

	hardBlank := rune(header[0][5])

	height, err := strconv.Atoi(header[1])
	if err != nil {
		return nil, err
	}

	oldLayout, _ := strconv.Atoi(header[4])
	commentLines, _ := strconv.Atoi(header[5])

	printDir := 0
	if len(header) > 6 {
		printDir, _ = strconv.Atoi(header[6])
	}

	fullLayout := 0
	if len(header) > 7 {
		fullLayout, _ = strconv.Atoi(header[7])
	} else if oldLayout == 0 {
		fullLayout = 64
	} else if oldLayout < 0 {
		fullLayout = 0
	} else {
		fullLayout = (oldLayout & 31) | 128
	}

	font := &Font{
		Height:      height,
		HardBlank:   hardBlank,
		FullLayout:  fullLayout,
		PrintDir:    printDir,
		Glyphs:      make(map[rune][]string),
		GlyphWidths: make(map[rune]int),
	}

	index := 1 + commentLines

	// 1. Load ASCII standard character set (32 - 126)
	for ch := rune(32); ch <= 126; ch++ {
		glyph, width, next := readOneGlyph(lines, index, height)
		if glyph != nil {
			if ch == 32 || hasNonEmptyRow(glyph) {
				font.Glyphs[ch] = glyph
				font.GlyphWidths[ch] = width
			}
		}
		index = next
	}

	// 2. Load German Umlaute (ÄÖÜäöüß)
	for _, ch := range "ÄÖÜäöüß" {
		if index+height <= len(lines) {
			glyph, width, next := readOneGlyph(lines, index, height)
			if glyph != nil && hasNonEmptyRow(glyph) {
				font.Glyphs[ch] = glyph
				font.GlyphWidths[ch] = width
			}
			index = next
		}
	}

	// 3. Load ASCII extended character set (0x hex markers)
	for index < len(lines) {
		line := strings.TrimSpace(lines[index])
		index++
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		codeStr := parts[0]
		if strings.HasPrefix(strings.ToLower(codeStr), "0x") {
			val, err := strconv.ParseInt(codeStr[2:], 16, 32)
			if err == nil {
				glyph, width, next := readOneGlyph(lines, index, height)
				if glyph != nil && hasNonEmptyRow(glyph) {
					font.Glyphs[rune(val)] = glyph
					font.GlyphWidths[rune(val)] = width
				}
				index = next
			}
		}
	}

	return font, nil
}

func readOneGlyph(lines []string, start, height int) ([]string, int, int) {
	if start+height > len(lines) {
		return nil, 0, start
	}

	firstLine := lines[start]
	trimmedFirst := strings.TrimRight(firstLine, " \t\r")
	if len(trimmedFirst) == 0 {
		return nil, 0, start + height
	}
	endmark := trimmedFirst[len(trimmedFirst)-1]

	rows := make([]string, height)
	maxWidth := 0

	for r := 0; r < height; r++ {
		line := lines[start+r]
		t := strings.TrimRight(line, " \t\r")
		count := 0
		for len(t) > 0 && t[len(t)-1] == endmark && count < 2 {
			t = t[:len(t)-1]
			count++
		}
		rows[r] = t
		w := utf8.RuneCountInString(t)
		if w > maxWidth {
			maxWidth = w
		}
	}

	return rows, maxWidth, start + height
}

func hasNonEmptyRow(glyph []string) bool {
	for _, row := range glyph {
		if row != "" {
			return true
		}
	}
	return false
}

func copyRows(rows []string) []string {
	result := make([]string, len(rows))
	copy(result, rows)
	return result
}

func newBuilder(text string, font *Font, direction, justify string, width int) *Builder {
	return &Builder{
		text:      []rune(text),
		font:      font,
		direction: direction,
		justify:   justify,
		width:     width,
		buffer:    make([]string, font.Height),
	}
}

func (b *Builder) getCharAt(i int) []string {
	if i < 0 || i >= len(b.text) {
		return nil
	}
	return b.font.Glyphs[b.text[i]]
}

func (b *Builder) getCharWidthAt(i int) int {
	if i < 0 || i >= len(b.text) {
		return 0
	}
	return b.font.GlyphWidths[b.text[i]]
}

func (b *Builder) getCurChar() []string {
	return b.getCharAt(b.iterator)
}

func (b *Builder) getCurWidth() int {
	return b.getCharWidthAt(b.iterator)
}

func (b *Builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}

	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}

	if b.font.FullLayout&smSmush == 0 {
		return 0, false
	}

	// Universal smushing
	if b.font.FullLayout&63 == 0 {
		if left == b.font.HardBlank {
			return right, true
		}
		if right == b.font.HardBlank {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}

	// Hardblank smushing
	if b.font.FullLayout&smHardblank != 0 &&
		left == b.font.HardBlank &&
		right == b.font.HardBlank {
		return left, true
	}

	if left == b.font.HardBlank || right == b.font.HardBlank {
		return 0, false
	}

	// Equal characters
	if b.font.FullLayout&smEqual != 0 && left == right {
		return left, true
	}

	rules := []struct {
		a string
		b string
	}{}

	if b.font.FullLayout&smLowline != 0 {
		rules = append(rules, struct {
			a string
			b string
		}{"_", "|/\\[]{}()<>"})
	}

	if b.font.FullLayout&smHierarchy != 0 {
		rules = append(rules,
			struct{ a, b string }{"|", "/\\[]{}()<>"},
			struct{ a, b string }{"\\/", "[]{}()<>"},
			struct{ a, b string }{"[]", "{}()<>"},
			struct{ a, b string }{"{}", "()<>"},
			struct{ a, b string }{"()", "<>"},
		)
	}

	for _, rule := range rules {
		if strings.ContainsRune(rule.a, left) && strings.ContainsRune(rule.b, right) {
			return right, true
		}
		if strings.ContainsRune(rule.a, right) && strings.ContainsRune(rule.b, left) {
			return left, true
		}
	}

	if b.font.FullLayout&smPair != 0 {
		pair := string([]rune{left, right})
		if pair == "[]" || pair == "][" || pair == "{}" || pair == "}{" || pair == "()" || pair == ")(" {
			return '|', true
		}
	}

	if b.font.FullLayout&smBigX != 0 {
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

func (b *Builder) smushAmount(curChar []string) int {
	if b.font.FullLayout&(smSmush|smKern) == 0 {
		return 0
	}

	maxSmush := b.curCharWidth

	for row := 0; row < b.font.Height; row++ {
		lineLeft := b.buffer[row]
		lineRight := curChar[row]

		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		lineLeftStripped := strings.TrimRight(lineLeft, " ")
		linebd := len(lineLeftStripped) - 1
		if linebd < 0 {
			linebd = 0
		}

		var ch1 rune
		if linebd < len(lineLeft) {
			ch1 = rune(lineLeft[linebd])
		} else {
			linebd = 0
		}

		lineRightStripped := strings.TrimLeft(lineRight, " ")
		charbd := len(lineRight) - len(lineRightStripped)

		var ch2 rune
		if charbd < len(lineRight) {
			ch2 = rune(lineRight[charbd])
		}

		amt := charbd + utf8.RuneCountInString(lineLeft) - 1 - linebd

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

func (b *Builder) currentSmushAmount(curChar []string) int {
	return b.smushAmount(curChar)
}

func (b *Builder) smushRow(curChar []string, row int) (string, string) {
	addLeft := []rune(b.buffer[row])
	addRight := []rune(curChar[row])

	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i

		var left rune
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}

		right := rune(0)
		if i < len(addRight) {
			right = addRight[i]
		}

		smushed, ok := b.smushChars(left, right)
		if ok && idx >= 0 && idx < len(addLeft) {
			addLeft[idx] = smushed
		}
	}

	return string(addLeft), string(addRight)
}

func (b *Builder) addCurCharRowToBufferRow(curChar []string, row int) {
	left, right := b.smushRow(curChar, row)
	rightRunes := []rune(right)

	smush := b.maxSmush
	if smush > len(rightRunes) {
		smush = len(rightRunes)
	}

	b.buffer[row] = left + string(rightRunes[smush:])
}

func (b *Builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.Height)
	b.blankMarkers = nil
	b.prevCharWidth = 0

	curChar := b.getCurChar()
	if curChar == nil {
		return
	}

	b.maxSmush = b.currentSmushAmount(curChar)
}

func (b *Builder) cutBufferAtLastBlank(saved savedBuffer) {
	b.product = append(b.product, copyRows(saved.rows))
	b.iterator = saved.iterator
	b.cutBufferCommon()
}

func (b *Builder) cutBufferAtLastChar() {
	b.product = append(b.product, copyRows(b.buffer))
	b.iterator--
	b.cutBufferCommon()
}

func (b *Builder) getLastBlank() (savedBuffer, bool) {
	if len(b.blankMarkers) == 0 {
		return savedBuffer{}, false
	}

	last := b.blankMarkers[len(b.blankMarkers)-1]
	b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
	return last, true
}

func (b *Builder) handleNewLine() {
	saved, ok := b.getLastBlank()
	if ok {
		b.cutBufferAtLastBlank(saved)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *Builder) addCharToProduct() error {
	curChar := b.getCurChar()

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, savedBuffer{
			rows:     copyRows(b.buffer),
			iterator: b.iterator,
		})
		b.handleNewLine()
		return nil
	}

	if curChar == nil {
		return nil
	}

	if b.width < b.getCurWidth() {
		return fmt.Errorf("width is not enough to print this character")
	}

	b.curCharWidth = b.getCurWidth()
	b.maxSmush = b.currentSmushAmount(curChar)

	b.currentTotalWidth = utf8.RuneCountInString(b.buffer[0]) +
		b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, savedBuffer{
			rows:     copyRows(b.buffer),
			iterator: b.iterator,
		})
	}

	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.Height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}

	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *Builder) justifyRows(rows []string) []string {
	result := copyRows(rows)

	for row := 0; row < b.font.Height; row++ {
		rowWidth := utf8.RuneCountInString(result[row])

		if b.justify == "right" {
			padding := b.width - rowWidth - 1
			if padding > 0 {
				result[row] = strings.Repeat(" ", padding) + result[row]
			}
		} else if b.justify == "center" {
			padding := (b.width - rowWidth) / 2
			if padding > 0 {
				result[row] = strings.Repeat(" ", padding) + result[row]
			}
		}
	}

	return result
}

func (b *Builder) result() string {
	if len(b.buffer) > 0 && b.buffer[0] != "" {
		b.product = append(b.product, copyRows(b.buffer))
	}

	var output strings.Builder

	for _, rows := range b.product {
		rows = b.justifyRows(rows)

		for _, row := range rows {
			row = strings.ReplaceAll(row, string(b.font.HardBlank), " ")
			output.WriteString(row)
			output.WriteByte('\n')
		}
	}

	return output.String()
}

func render(text string, font *Font, width int, justify, direction string) (string, error) {
	builder := newBuilder(text, font, direction, justify, width)

	for builder.iterator < len(builder.text) {
		if err := builder.addCharToProduct(); err != nil {
			return "", err
		}
		builder.iterator++
	}

	return builder.result(), nil
}

func reverseLines(input string) string {
	reverseMap := map[rune]rune{
		'(': ')', ')': '(',
		'/': '\\', '\\': '/',
		'<': '>', '>': '<',
		'[': ']', ']': '[',
		'{': '}', '}': '{',
	}

	lines := strings.Split(strings.TrimSuffix(input, "\n"), "\n")

	for i, line := range lines {
		runes := []rune(line)
		for a, b := 0, len(runes)-1; a < b; a, b = a+1, b-1 {
			runes[a], runes[b] = runes[b], runes[a]
		}
		for j, ch := range runes {
			if replacement, ok := reverseMap[ch]; ok {
				runes[j] = replacement
			}
		}
		lines[i] = string(runes)
	}

	return strings.Join(lines, "\n") + "\n"
}

func flipLines(input string) string {
	flipMap := map[rune]rune{
		'/': '\\', '\\': '/',
		'A': 'V', 'V': 'A',
		'M': 'W', 'W': 'M',
		'P': 'b', 'R': 'b', 'b': 'P',
		'v': '^', '^': 'v',
		'_': '-',
		'm': 'w', 'w': 'm',
	}

	lines := strings.Split(strings.TrimSuffix(input, "\n"), "\n")

	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	for i, line := range lines {
		runes := []rune(line)
		for j, ch := range runes {
			if replacement, ok := flipMap[ch]; ok {
				runes[j] = replacement
			}
		}
		lines[i] = string(runes)
	}

	return strings.Join(lines, "\n") + "\n"
}

func stripSurroundingNewlines(input string) string {
	lines := strings.Split(strings.TrimSuffix(input, "\n"), "\n")
	var out []string
	charsSeen := false
	for _, row := range lines {
		if strings.TrimSpace(row) != "" || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}
	if len(out) == 0 {
		return ""
	}
	joined := strings.Join(out, "\n") + "\n"
	return strings.TrimRight(joined, " \t\r\n")
}

func normalizeSurroundingNewlines(input string) string {
	return "\n" + stripSurroundingNewlines(input) + "\n"
}

func main() {
	fontName := flag.String("f", "standard", "font name")
	width := flag.Int("w", 80, "output width")
	justify := flag.String("j", "auto", "text justification")
	direction := flag.String("D", "auto", "text direction")
	reverse := flag.Bool("r", false, "reverse output")
	flip := flag.Bool("F", false, "flip output")
	strip := flag.Bool("s", false, "strip surrounding newlines")
	normalize := flag.Bool("n", false, "normalize surrounding newlines")

	flag.StringVar(fontName, "font", "standard", "font name")
	flag.IntVar(width, "width", 80, "output width")
	flag.StringVar(justify, "justify", "auto", "text justification")
	flag.StringVar(direction, "direction", "auto", "text direction")
	flag.BoolVar(reverse, "reverse", false, "reverse output")
	flag.BoolVar(flip, "flip", false, "flip output")
	flag.BoolVar(strip, "strip-surrounding-newlines", false, "strip surrounding newlines")
	flag.BoolVar(normalize, "normalize-surrounding-newlines", false, "normalize surrounding newlines")

	flag.Parse()

	text := strings.Join(flag.Args(), " ")

	font, err := loadFont(*fontName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: requested font %q not found.\n", *fontName)
		os.Exit(1)
	}

	if *direction == "auto" {
		*direction = "left-to-right"
		if font.PrintDir == 1 {
			*direction = "right-to-left"
		}
	}

	if *justify == "auto" {
		*justify = "left"
		if *direction == "right-to-left" {
			*justify = "right"
		}
	}

	result, err := render(text, font, *width, *justify, *direction)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *reverse {
		result = reverseLines(result)
	}

	if *flip {
		result = flipLines(result)
	}

	if *strip {
		result = stripSurroundingNewlines(result)
	} else if *normalize {
		result = normalizeSurroundingNewlines(result)
	}

	fmt.Print(result)
	fmt.Print("\n")
}
