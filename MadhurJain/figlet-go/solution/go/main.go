package main

import (
	"embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

//go:embed fonts/*.flf
var fontFS embed.FS

// ---------------------------------------------------------------------------
// Translation maps for reverse and flip (256 entries each)
// ---------------------------------------------------------------------------

var reverseMap = [256]rune{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
	32, 33, 34, 35, 36, 37, 38, 39, 41, 40, 42, 43, 44, 45, 46, 92,
	48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 62, 61, 60, 63,
	64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
	80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 93, 47, 91, 94, 95,
	96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111,
	112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 125, 124, 123, 126, 127,
	128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143,
	144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159,
	160, 161, 162, 163, 164, 165, 166, 167, 168, 169, 170, 171, 172, 173, 174, 175,
	176, 177, 178, 179, 180, 181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191,
	192, 193, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207,
	208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223,
	224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239,
	240, 241, 242, 243, 244, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255,
}

var flipMap = [256]rune{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
	32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 92,
	48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63,
	64, 86, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 87, 78, 79,
	98, 81, 98, 83, 84, 85, 65, 77, 88, 89, 90, 91, 47, 93, 118, 45,
	96, 97, 80, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 119, 110, 111,
	112, 113, 114, 115, 116, 117, 94, 109, 120, 121, 122, 123, 124, 125, 126, 127,
	128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143,
	144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159,
	160, 161, 162, 163, 164, 165, 166, 167, 168, 169, 170, 171, 172, 173, 174, 175,
	176, 177, 178, 179, 180, 181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191,
	192, 193, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207,
	208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223,
	224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239,
	240, 241, 242, 243, 244, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255,
}

// ---------------------------------------------------------------------------
// FigletFont
// ---------------------------------------------------------------------------

type FigletFont struct {
	font           string
	comment        string
	chars          map[rune][]string
	width          map[rune]int
	height         int
	hardBlank      rune
	printDirection int
	smushMode      int
}

func parseChar(lines *[]string, height int) (int, []string) {
	var endRune rune
	var hasEnd bool
	width := 0
	chars := make([]string, 0, height)

	for r := 0; r < height; r++ {
		line := (*lines)[0]
		*lines = (*lines)[1:]
		runes := []rune(line)

		if !hasEnd {
			// Find the last non-space character
			idx := len(runes) - 1
			for idx >= 0 && runes[idx] == ' ' {
				idx--
			}
			if idx >= 0 {
				endRune = runes[idx]
				hasEnd = true
			}
		}

		if hasEnd {
			// Remove endRune once or twice, plus trailing spaces, from end of line
			// Matching Python: re.compile(re.escape(end) + r'{1,2}\s*$')
			endIdx := len(runes)
			for endIdx > 0 && runes[endIdx-1] == ' ' {
				endIdx--
			}
			count := 0
			for count < 2 && endIdx > 0 && runes[endIdx-1] == endRune {
				count++
				endIdx--
			}
			if count > 0 {
				runes = runes[:endIdx]
			}
		}

		if len(runes) > width {
			width = len(runes)
		}
		chars = append(chars, string(runes))
	}

	return width, chars
}

func loadFont(fontName string) (*FigletFont, error) {
	rawBytes, err := fontFS.ReadFile("fonts/" + fontName + ".flf")
	if err != nil {
		return nil, fmt.Errorf("requested font %q not found", fontName)
	}

	rawStr := string(rawBytes)
	rawStr = strings.ReplaceAll(rawStr, "\u0085", " ")
	rawStr = strings.ReplaceAll(rawStr, "\u2028", " ")
	rawStr = strings.ReplaceAll(rawStr, "\u2029", " ")

	lines := splitLinesPy(rawStr)
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font file")
	}

	header := lines[0]
	lines = lines[1:]

	if !strings.HasPrefix(header, "flf2") && !strings.HasPrefix(header, "tlf2") {
		return nil, fmt.Errorf("%s is not a valid figlet font", fontName)
	}
	// Skip magic number: [tf]lf2. (5 characters)
	if len(header) < 5 {
		return nil, fmt.Errorf("malformed header for %s", fontName)
	}
	headerRest := header[5:]
	headerFields := strings.Fields(headerRest)
	if len(headerFields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", fontName)
	}

	hardBlankRunes := []rune(headerFields[0])
	if len(hardBlankRunes) == 0 {
		return nil, fmt.Errorf("missing hardblank in %s", fontName)
	}
	hardBlank := hardBlankRunes[0]

	height, _ := strconv.Atoi(headerFields[1])
	baseLine, _ := strconv.Atoi(headerFields[2])
	_ = baseLine
	maxLength, _ := strconv.Atoi(headerFields[3])
	_ = maxLength
	oldLayout, _ := strconv.Atoi(headerFields[4])
	commentLines, _ := strconv.Atoi(headerFields[5])

	printDirection := 0
	if len(headerFields) > 6 {
		printDirection, _ = strconv.Atoi(headerFields[6])
	}

	fullLayout := -1
	if len(headerFields) > 7 {
		fullLayout, _ = strconv.Atoi(headerFields[7])
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

	ff := &FigletFont{
		font:           fontName,
		chars:          make(map[rune][]string),
		width:          make(map[rune]int),
		height:         height,
		hardBlank:      hardBlank,
		printDirection: printDirection,
		smushMode:      fullLayout,
	}

	// Strip comment lines
	var comment strings.Builder
	for i := 0; i < commentLines && len(lines) > 0; i++ {
		comment.WriteString(lines[0])
		lines = lines[1:]
	}
	ff.comment = comment.String()

	// Load ASCII standard character set (32 - 127)
	for i := 32; i < 127; i++ {
		if len(lines) < height {
			break
		}
		w, letter := parseChar(&lines, height)
		if i == 32 || strings.Join(letter, "") != "" {
			ff.chars[rune(i)] = letter
			ff.width[rune(i)] = w
		}
	}

	// Load German Umlaute - directly after character 127
	germanChars := []rune("ÄÖÜäöüß")
	for _, c := range germanChars {
		if len(lines) < height {
			break
		}
		w, letter := parseChar(&lines, height)
		if strings.Join(letter, "") != "" {
			ff.chars[c] = letter
			ff.width[c] = w
		}
	}

	// Load ASCII extended character set (hex-tagged only)
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		tag := parts[0]
		if strings.HasPrefix(strings.ToLower(tag), "0x") {
			code, err := strconv.ParseInt(tag[2:], 16, 64)
			if err == nil && len(lines) >= height {
				w, letter := parseChar(&lines, height)
				if strings.Join(letter, "") != "" {
					ff.chars[rune(code)] = letter
					ff.width[rune(code)] = w
				}
			}
		}
	}

	return ff, nil
}

// ---------------------------------------------------------------------------
// FigletBuilder
// ---------------------------------------------------------------------------

const (
	SM_EQUAL     = 1
	SM_LOWLINE   = 2
	SM_HIERARCHY = 4
	SM_PAIR      = 8
	SM_BIGX      = 16
	SM_HARDBLANK = 32
	SM_KERN      = 64
	SM_SMUSH     = 128
)

type blankMarker struct {
	buffer   []string
	iterator int
}

type FigletBuilder struct {
	text              []rune
	direction         string
	width             int
	font              *FigletFont
	justify           string
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []blankMarker
	productQueue      [][]string
	buffer            []string
}

func copyBuffer(buf []string) []string {
	c := make([]string, len(buf))
	copy(c, buf)
	return c
}

func runeLen(s string) int {
	return len([]rune(s))
}

func (b *FigletBuilder) getCurChar() ([]string, bool) {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return nil, false
	}
	c := b.text[b.iterator]
	ch, ok := b.font.chars[c]
	return ch, ok
}

func (b *FigletBuilder) getCurWidth() int {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return 0
	}
	c := b.text[b.iterator]
	return b.font.width[c]
}

func (b *FigletBuilder) isNotFinished() bool {
	return b.iterator < len(b.text)
}

func (b *FigletBuilder) goToNextChar() {
	b.iterator++
}

func (b *FigletBuilder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, ok := b.getCurChar()
	if !ok {
		return
	}
	b.maxSmush = b.smushAmount(curChar)
}

func (b *FigletBuilder) cutBufferAtLastBlank(savedBuffer []string, savedIterator int) {
	b.productQueue = append(b.productQueue, savedBuffer)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *FigletBuilder) cutBufferAtLastChar() {
	b.productQueue = append(b.productQueue, copyBuffer(b.buffer))
	b.iterator--
	b.cutBufferCommon()
}

func (b *FigletBuilder) getLastBlank() (blankMarker, bool) {
	if len(b.blankMarkers) == 0 {
		return blankMarker{}, false
	}
	last := b.blankMarkers[len(b.blankMarkers)-1]
	b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
	return last, true
}

func (b *FigletBuilder) handleNewLine() {
	saved, ok := b.getLastBlank()
	if ok {
		b.cutBufferAtLastBlank(saved.buffer, saved.iterator)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *FigletBuilder) addCharToProduct() {
	curChar, ok := b.getCurChar()

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{
			buffer:   copyBuffer(b.buffer),
			iterator: b.iterator,
		})
		b.handleNewLine()
		return
	}

	if !ok {
		return
	}

	curWidth := b.getCurWidth()
	if b.width < curWidth {
		// "If a character is wider than -w, print nothing to stdout and exit 1."
		os.Exit(1)
	}

	b.curCharWidth = curWidth
	b.maxSmush = b.smushAmount(curChar)

	b.currentTotalWidth = runeLen(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{
			buffer:   copyBuffer(b.buffer),
			iterator: b.iterator,
		})
	}

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{
			buffer:   copyBuffer(b.buffer),
			iterator: b.iterator,
		})
		b.handleNewLine()
	}

	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}

	b.prevCharWidth = b.curCharWidth
}

func (b *FigletBuilder) smushChars(left, right rune) rune {
	if left == ' ' {
		return right
	}
	if right == ' ' {
		return left
	}

	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0
	}

	if (b.font.smushMode & SM_SMUSH) == 0 {
		return 0
	}

	// Universal overlapping
	if (b.font.smushMode & 63) == 0 {
		if left == b.font.hardBlank {
			return right
		}
		if right == b.font.hardBlank {
			return left
		}
		if b.direction == "right-to-left" {
			return left
		}
		return right
	}

	if (b.font.smushMode & SM_HARDBLANK) != 0 {
		if left == b.font.hardBlank && right == b.font.hardBlank {
			return left
		}
	}

	if left == b.font.hardBlank || right == b.font.hardBlank {
		return 0
	}

	if (b.font.smushMode & SM_EQUAL) != 0 {
		if left == right {
			return left
		}
	}

	type smushPair struct {
		a, b string
	}
	var smushes []smushPair

	if (b.font.smushMode & SM_LOWLINE) != 0 {
		smushes = append(smushes, smushPair{"_", `|/\[]{}()<>`})
	}

	if (b.font.smushMode & SM_HIERARCHY) != 0 {
		smushes = append(smushes,
			smushPair{"|", `/\[]{}()<>`},
			smushPair{`/\\`, `[]{}()<>`},
			smushPair{`[]`, `{}()<>`},
			smushPair{`{}`, `()<>`},
			smushPair{`()`, `<>`},
		)
	}

	for _, sp := range smushes {
		if strings.ContainsRune(sp.a, left) && strings.ContainsRune(sp.b, right) {
			return right
		}
		if strings.ContainsRune(sp.a, right) && strings.ContainsRune(sp.b, left) {
			return left
		}
	}

	if (b.font.smushMode & SM_PAIR) != 0 {
		p1 := string([]rune{left, right})
		p2 := string([]rune{right, left})
		for _, pair := range []string{p1, p2} {
			if pair == "[]" || pair == "{}" || pair == "()" {
				return '|'
			}
		}
	}

	if (b.font.smushMode & SM_BIGX) != 0 {
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

func (b *FigletBuilder) smushAmount(curChar []string) int {
	if (b.font.smushMode & (SM_SMUSH | SM_KERN)) == 0 {
		return 0
	}

	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := []rune(b.buffer[row])
		lineRight := []rune(curChar[row])
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		rEnd := len(lineLeft)
		for rEnd > 0 && lineLeft[rEnd-1] == ' ' {
			rEnd--
		}
		linebd := rEnd - 1
		if linebd < 0 {
			linebd = 0
		}

		var ch1 rune
		var hasCh1 bool
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
			hasCh1 = true
		} else {
			linebd = 0
			hasCh1 = false
		}

		charbd := 0
		for charbd < len(lineRight) && lineRight[charbd] == ' ' {
			charbd++
		}
		var ch2 rune
		var hasCh2 bool
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
			hasCh2 = true
		} else {
			charbd = len(lineRight)
			hasCh2 = false
		}

		amt := charbd + len(lineLeft) - 1 - linebd

		if !hasCh1 || ch1 == ' ' {
			amt++
		} else if hasCh2 && b.smushChars(ch1, ch2) != 0 {
			amt++
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}

	return maxSmush
}

func (b *FigletBuilder) smushRow(curChar []string, row int) ([]rune, []rune) {
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
		var right rune
		if i < len(addRight) {
			right = addRight[i]
		}

		smushed := b.smushChars(left, right)
		if idx >= 0 && idx < len(addLeft) && smushed != 0 {
			addLeft[idx] = smushed
		}
	}

	return addLeft, addRight
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	var rightTail []rune
	if b.maxSmush < len(addRight) {
		rightTail = addRight[b.maxSmush:]
	}
	b.buffer[row] = string(append(addLeft, rightTail...))
}

func (b *FigletBuilder) justifyString(justify string, buffer []string) []string {
	if justify == "right" {
		for row := 0; row < b.font.height; row++ {
			nSpaces := b.width - runeLen(buffer[row]) - 1
			if nSpaces < 0 {
				nSpaces = 0
			}
			buffer[row] = strings.Repeat(" ", nSpaces) + buffer[row]
		}
	} else if justify == "center" {
		for row := 0; row < b.font.height; row++ {
			nSpaces := (b.width - runeLen(buffer[row])) / 2
			if nSpaces < 0 {
				nSpaces = 0
			}
			buffer[row] = strings.Repeat(" ", nSpaces) + buffer[row]
		}
	}
	return buffer
}

func (b *FigletBuilder) replaceHardblanks(buffer []string) string {
	s := strings.Join(buffer, "\n") + "\n"
	s = strings.ReplaceAll(s, string(b.font.hardBlank), " ")
	return s
}

func (b *FigletBuilder) returnProduct() string {
	if b.buffer[0] != "" {
		b.productQueue = append(b.productQueue, copyBuffer(b.buffer))
	}
	var sb strings.Builder
	for _, buf := range b.productQueue {
		bufCopy := copyBuffer(buf)
		bufJust := b.justifyString(b.justify, bufCopy)
		sb.WriteString(b.replaceHardblanks(bufJust))
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Figlet & FigletString operations
// ---------------------------------------------------------------------------

func splitLinesPy(s string) []string {
	var lines []string
	runes := []rune(s)
	n := len(runes)
	start := 0
	i := 0
	for i < n {
		if runes[i] == '\r' {
			lines = append(lines, string(runes[start:i]))
			if i+1 < n && runes[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
			start = i
		} else if runes[i] == '\n' {
			lines = append(lines, string(runes[start:i]))
			i++
			start = i
		} else {
			i++
		}
	}
	if start < n {
		lines = append(lines, string(runes[start:n]))
	}
	return lines
}

func figletReverse(s string) string {
	lines := splitLinesPy(s)
	var out []string
	for _, row := range lines {
		runes := []rune(row)
		for i, r := range runes {
			if r < 256 {
				runes[i] = reverseMap[r]
			}
		}
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

func figletFlip(s string) string {
	lines := splitLinesPy(s)
	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		row := lines[i]
		runes := []rune(row)
		for j, r := range runes {
			if r < 256 {
				runes[j] = flipMap[r]
			}
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

func figletStripSurroundingNewlines(s string) string {
	lines := splitLinesPy(s)
	var out []string
	charsSeen := false
	for _, row := range lines {
		if strings.TrimSpace(row) != "" || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}
	res := strings.Join(out, "\n") + "\n"
	return strings.TrimRightFunc(res, unicode.IsSpace)
}

func figletNormalizeSurroundingNewlines(s string) string {
	return "\n" + figletStripSurroundingNewlines(s) + "\n"
}

type Figlet struct {
	font      string
	direction string
	justify   string
	width     int
	Font      *FigletFont
}

func newFiglet(fontName, direction, justify string, width int) (*Figlet, error) {
	ff, err := loadFont(fontName)
	if err != nil {
		return nil, err
	}

	resolvedDir := direction
	if resolvedDir == "auto" {
		if ff.printDirection == 1 {
			resolvedDir = "right-to-left"
		} else {
			resolvedDir = "left-to-right"
		}
	}

	resolvedJust := justify
	if resolvedJust == "auto" {
		if resolvedDir == "right-to-left" {
			resolvedJust = "right"
		} else {
			resolvedJust = "left"
		}
	}

	return &Figlet{
		font:      fontName,
		direction: resolvedDir,
		justify:   resolvedJust,
		width:     width,
		Font:      ff,
	}, nil
}

func (f *Figlet) renderText(text string) string {
	builder := &FigletBuilder{
		text:      []rune(text),
		direction: f.direction,
		width:     f.width,
		font:      f.Font,
		justify:   f.justify,
		buffer:    make([]string, f.Font.height),
	}

	for builder.isNotFinished() {
		builder.addCharToProduct()
		builder.goToNextChar()
	}

	return builder.returnProduct()
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	font := "standard"
	width := 80
	justify := "auto"
	direction := "auto"
	reverse := false
	flip := false
	stripNewlines := false
	normNewlines := false
	var textParts []string

	args := os.Args[1:]
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			textParts = append(textParts, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			textParts = append(textParts, args[i:]...)
			break
		}

		// Flag parsing
		if arg == "-f" || arg == "--font" {
			i++
			if i < len(args) {
				font = args[i]
			}
		} else if strings.HasPrefix(arg, "-f=") {
			font = arg[len("-f="):]
		} else if strings.HasPrefix(arg, "--font=") {
			font = arg[len("--font="):]
		} else if arg == "-w" || arg == "--width" {
			i++
			if i < len(args) {
				width, _ = strconv.Atoi(args[i])
			}
		} else if strings.HasPrefix(arg, "-w=") {
			width, _ = strconv.Atoi(arg[len("-w="):])
		} else if strings.HasPrefix(arg, "--width=") {
			width, _ = strconv.Atoi(arg[len("--width="):])
		} else if arg == "-j" || arg == "--justify" {
			i++
			if i < len(args) {
				justify = args[i]
			}
		} else if strings.HasPrefix(arg, "-j=") {
			justify = arg[len("-j="):]
		} else if strings.HasPrefix(arg, "--justify=") {
			justify = arg[len("--justify="):]
		} else if arg == "-D" || arg == "--direction" {
			i++
			if i < len(args) {
				direction = args[i]
			}
		} else if strings.HasPrefix(arg, "-D=") {
			direction = arg[len("-D="):]
		} else if strings.HasPrefix(arg, "--direction=") {
			direction = arg[len("--direction="):]
		} else if arg == "-r" || arg == "--reverse" {
			reverse = true
		} else if arg == "-F" || arg == "--flip" {
			flip = true
		} else if arg == "-s" || arg == "--strip-surrounding-newlines" {
			stripNewlines = true
		} else if arg == "-n" || arg == "--normalize-surrounding-newlines" {
			normNewlines = true
		} else if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 1 {
			// Possibly grouped flags e.g. -rF or -sF
			matchedAll := true
			for _, ch := range arg[1:] {
				switch ch {
				case 'r':
					reverse = true
				case 'F':
					flip = true
				case 's':
					stripNewlines = true
				case 'n':
					normNewlines = true
				default:
					matchedAll = false
				}
			}
			if !matchedAll {
				textParts = append(textParts, args[i:]...)
				break
			}
		} else {
			textParts = append(textParts, args[i:]...)
			break
		}
		i++
	}

	if len(textParts) == 0 {
		os.Exit(1)
	}

	text := strings.Join(textParts, " ")

	f, err := newFiglet(font, direction, justify, width)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: %v\n", err)
		os.Exit(1)
	}

	r := f.renderText(text)
	if reverse {
		r = figletReverse(r)
	}
	if flip {
		r = figletFlip(r)
	}
	if stripNewlines {
		r = figletStripSurroundingNewlines(r)
	} else if normNewlines {
		r = figletNormalizeSurroundingNewlines(r)
	}

	os.Stdout.WriteString(r + "\n")
}
