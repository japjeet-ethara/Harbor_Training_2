package main

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Embed standard fonts so the binary runs from any working directory.
//
//go:embed pyfiglet/fonts/*
var embeddedFonts embed.FS

// Layout / Smush mode bit constants (matching FIGlet 2.2 / pyfiglet).
const (
	SmEqual     = 1   // smush equal characters (not hardblanks)
	SmLowline   = 2   // smush _ with any char in hierarchy
	SmHierarchy = 4   // hierarchy: |, /\, [], {}, (), <>
	SmPair      = 8   // [ + ] -> |, { + } -> |, ( + ) -> |
	SmBigX      = 16  // / + \ -> |, \ + / -> Y, > + < -> X
	SmHardblank = 32  // hardblank + hardblank -> hardblank
	SmKern      = 64  // kerning
	SmSmush     = 128 // smushing
)

// FigletFont holds parsed FIGfont data.
type FigletFont struct {
	Name           string
	Height         int
	BaseLine       int
	MaxLength      int
	OldLayout      int
	CommentLines   int
	PrintDirection int
	SmushMode      int
	HardBlank      rune
	Comments       string
	Chars          map[int][][]rune
	Width          map[int]int
}

// loadFontData locates font data from the filesystem or embedded FS.
func loadFontData(fontName string) ([]byte, error) {
	// If direct path was provided
	if strings.HasSuffix(fontName, ".flf") || strings.HasSuffix(fontName, ".tlf") {
		if data, err := os.ReadFile(fontName); err == nil {
			return data, nil
		}
	}

	extensions := []string{".flf", ".tlf"}

	// 1. Check current directory and known filesystem locations
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	locations := []string{
		"pyfiglet/fonts",
		"/workspace/repo/pyfiglet/fonts",
		filepath.Join(exeDir, "pyfiglet", "fonts"),
		filepath.Join(exeDir, "..", "pyfiglet", "fonts"),
		".",
	}

	for _, ext := range extensions {
		filename := fontName + ext
		for _, loc := range locations {
			fullPath := filepath.Join(loc, filename)
			if data, err := os.ReadFile(fullPath); err == nil {
				return data, nil
			}
		}
	}

	// 2. Check embedded font files
	for _, ext := range extensions {
		embedPath := "pyfiglet/fonts/" + fontName + ext
		if data, err := embeddedFonts.ReadFile(embedPath); err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("requested font %q not found", fontName)
}

// parseFont parses the raw FIGfont data into a FigletFont struct.
func parseFont(name string, rawData []byte) (*FigletFont, error) {
	// Replace non-CRLF unicode line splitting characters with spaces (as in pyfiglet)
	text := string(rawData)
	text = strings.ReplaceAll(text, "\u0085", " ")
	text = strings.ReplaceAll(text, "\u2028", " ")
	text = strings.ReplaceAll(text, "\u2029", " ")

	// Split lines
	rawLines := strings.Split(text, "\n")
	var lines []string
	for _, l := range rawLines {
		lines = append(lines, strings.TrimRight(l, "\r"))
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font file")
	}

	headerLine := lines[0]
	lines = lines[1:]

	// Magic number check: ^[tf]lf2.
	magicRe := regexp.MustCompile(`^[tf]lf2.`)
	loc := magicRe.FindStringIndex(headerLine)
	if loc == nil {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}

	afterMagic := headerLine[loc[1]:]
	headerFields := strings.Fields(afterMagic)
	if len(headerFields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}

	hardBlankRunes := []rune(headerFields[0])
	var hardBlank rune
	if len(hardBlankRunes) > 0 {
		hardBlank = hardBlankRunes[0]
	}

	height, _ := strconv.Atoi(headerFields[1])
	baseLine, _ := strconv.Atoi(headerFields[2])
	maxLength, _ := strconv.Atoi(headerFields[3])
	oldLayout, _ := strconv.Atoi(headerFields[4])
	commentLines, _ := strconv.Atoi(headerFields[5])

	printDirection := 0
	fullLayout := -1

	if len(headerFields) > 6 {
		printDirection, _ = strconv.Atoi(headerFields[6])
	}
	if len(headerFields) > 7 {
		fullLayout, _ = strconv.Atoi(headerFields[7])
	}

	// Backwards compatibility layout conversion
	if fullLayout == -1 {
		if oldLayout == 0 {
			fullLayout = 64
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | 128
		}
	}

	font := &FigletFont{
		Name:           name,
		Height:         height,
		BaseLine:       baseLine,
		MaxLength:      maxLength,
		OldLayout:      oldLayout,
		CommentLines:   commentLines,
		PrintDirection: printDirection,
		SmushMode:      fullLayout,
		HardBlank:      hardBlank,
		Chars:          make(map[int][][]rune),
		Width:          make(map[int]int),
	}

	// Skip comment lines
	for i := 0; i < commentLines && len(lines) > 0; i++ {
		font.Comments += lines[0]
		lines = lines[1:]
	}

	endMarkerRe := regexp.MustCompile(`(.)\s*$`)

	// Helper to extract one glyph definition of `height` lines
	readChar := func() (int, [][]rune) {
		width := 0
		var charLines [][]rune
		var endChar string
		var endRe *regexp.Regexp

		for row := 0; row < height; row++ {
			if len(lines) == 0 {
				charLines = append(charLines, []rune{})
				continue
			}
			line := lines[0]
			lines = lines[1:]

			if endRe == nil {
				matches := endMarkerRe.FindStringSubmatch(line)
				if len(matches) > 1 {
					endChar = matches[1]
					endRe = regexp.MustCompile(regexp.QuoteMeta(endChar) + `{1,2}\s*$`)
				}
			}

			if endRe != nil {
				line = endRe.ReplaceAllString(line, "")
			}

			lineRunes := []rune(line)
			if len(lineRunes) > width {
				width = len(lineRunes)
			}
			charLines = append(charLines, lineRunes)
		}
		return width, charLines
	}

	// 1. Load ASCII standard character set (32 - 126)
	for i := 32; i < 127; i++ {
		w, letter := readChar()
		isAllEmpty := true
		for _, l := range letter {
			if len(l) > 0 {
				isAllEmpty = false
				break
			}
		}
		if i == 32 || !isAllEmpty {
			font.Chars[i] = letter
			font.Width[i] = w
		}
	}

	// 2. Load German Umlaute (Ä, Ö, Ü, ä, ö, ü, ß)
	germanRunes := []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'}
	if len(lines) > 0 {
		for _, r := range germanRunes {
			if len(lines) == 0 {
				break
			}
			w, letter := readChar()
			isAllEmpty := true
			for _, l := range letter {
				if len(l) > 0 {
					isAllEmpty = false
					break
				}
			}
			if !isAllEmpty {
				font.Chars[int(r)] = letter
				font.Width[int(r)] = w
			}
		}
	}

	// 3. Load ASCII extended character set (0x...)
	hexPrefixRe := regexp.MustCompile(`(?i)^0x([0-9a-f]+)`)
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		codeStr := parts[0]
		matches := hexPrefixRe.FindStringSubmatch(codeStr)
		if len(matches) > 1 {
			val, err := strconv.ParseInt(matches[1], 16, 32)
			if err == nil {
				w, letter := readChar()
				isAllEmpty := true
				for _, l := range letter {
					if len(l) > 0 {
						isAllEmpty = false
						break
					}
				}
				if !isAllEmpty {
					font.Chars[int(val)] = letter
					font.Width[int(val)] = w
				}
			}
		}
	}

	return font, nil
}

// BlankMarker records buffer and iterator position at whitespace.
type BlankMarker struct {
	Buffer   [][]rune
	Iterator int
}

// FigletBuilder drives character layout, line breaks, kerning, and smushing.
type FigletBuilder struct {
	Text          []int
	Direction     string
	Width         int
	Font          *FigletFont
	Justify       string
	Iterator      int
	MaxSmush      int
	CurCharWidth  int
	PrevCharWidth int
	TotalWidth    int
	BlankMarkers  []BlankMarker
	Queue         [][][]rune
	Buffer        [][]rune
}

func newFigletBuilder(text string, font *FigletFont, direction string, width int, justify string) *FigletBuilder {
	var runes []int
	for _, r := range text {
		runes = append(runes, int(r))
	}

	buf := make([][]rune, font.Height)
	for i := range buf {
		buf[i] = []rune{}
	}

	b := &FigletBuilder{
		Text:         runes,
		Direction:    direction,
		Width:        width,
		Font:         font,
		Justify:      justify,
		Iterator:     0,
		BlankMarkers: make([]BlankMarker, 0),
		Queue:        make([][][]rune, 0),
		Buffer:       buf,
	}
	return b
}

func (b *FigletBuilder) isNotFinished() bool {
	return b.Iterator < len(b.Text)
}

func (b *FigletBuilder) goToNextChar() {
	b.Iterator++
}

func (b *FigletBuilder) getCharAt(idx int) [][]rune {
	if idx < 0 || idx >= len(b.Text) {
		return nil
	}
	code := b.Text[idx]
	if ch, ok := b.Font.Chars[code]; ok {
		return ch
	}
	return nil
}

func (b *FigletBuilder) getCharWidthAt(idx int) int {
	if idx < 0 || idx >= len(b.Text) {
		return 0
	}
	code := b.Text[idx]
	if w, ok := b.Font.Width[code]; ok {
		return w
	}
	return 0
}

func (b *FigletBuilder) getCurChar() [][]rune {
	return b.getCharAt(b.Iterator)
}

func (b *FigletBuilder) getCurWidth() int {
	return b.getCharWidthAt(b.Iterator)
}

func (b *FigletBuilder) cloneBuffer() [][]rune {
	cp := make([][]rune, len(b.Buffer))
	for i, row := range b.Buffer {
		r := make([]rune, len(row))
		copy(r, row)
		cp[i] = r
	}
	return cp
}

func (b *FigletBuilder) cutBufferCommon() {
	b.TotalWidth = 0
	b.Buffer = make([][]rune, b.Font.Height)
	for i := range b.Buffer {
		b.Buffer[i] = []rune{}
	}
	b.BlankMarkers = nil
	b.PrevCharWidth = 0
	curChar := b.getCurChar()
	if curChar == nil {
		return
	}
	b.MaxSmush = b.currentSmushAmount(curChar)
}

func (b *FigletBuilder) cutBufferAtLastBlank(savedBuffer [][]rune, savedIterator int) {
	b.Queue = append(b.Queue, savedBuffer)
	b.Iterator = savedIterator
	b.cutBufferCommon()
}

func (b *FigletBuilder) cutBufferAtLastChar() {
	b.Queue = append(b.Queue, b.Buffer)
	b.Iterator--
	b.cutBufferCommon()
}

func (b *FigletBuilder) handleNewLine() {
	if len(b.BlankMarkers) > 0 {
		last := b.BlankMarkers[len(b.BlankMarkers)-1]
		b.BlankMarkers = b.BlankMarkers[:len(b.BlankMarkers)-1]
		b.cutBufferAtLastBlank(last.Buffer, last.Iterator)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *FigletBuilder) smushRunes(left, right rune) rune {
	if left == ' ' {
		return right
	}
	if right == ' ' {
		return left
	}

	if b.PrevCharWidth < 2 || b.CurCharWidth < 2 {
		return 0
	}

	// Kerning only
	if (b.Font.SmushMode & SmSmush) == 0 {
		return 0
	}

	// Universal overlapping (when none of the 6 smush rules are enabled)
	if (b.Font.SmushMode & 63) == 0 {
		if left == b.Font.HardBlank {
			return right
		}
		if right == b.Font.HardBlank {
			return left
		}
		if b.Direction == "right-to-left" {
			return left
		}
		return right
	}

	// Rule 6: Hardblank smushing
	if (b.Font.SmushMode & SmHardblank) != 0 {
		if left == b.Font.HardBlank && right == b.Font.HardBlank {
			return left
		}
	}

	if left == b.Font.HardBlank || right == b.Font.HardBlank {
		return 0
	}

	// Rule 1: Equal character smushing
	if (b.Font.SmushMode & SmEqual) != 0 {
		if left == right {
			return left
		}
	}

	// Rule 2: Lowline (underscore) smushing
	if (b.Font.SmushMode & SmLowline) != 0 {
		if left == '_' && strings.ContainsRune("|/\\[]{}()<>", right) {
			return right
		}
		if right == '_' && strings.ContainsRune("|/\\[]{}()<>", left) {
			return left
		}
	}

	// Rule 3: Hierarchy smushing
	if (b.Font.SmushMode & SmHierarchy) != 0 {
		hierarchyPairs := []struct {
			a string
			b string
		}{
			{"|", "/\\[]{}()<>"},
			{"/\\", "[]{}()<>"},
			{"[]", "{}()<>"},
			{"{}", "()<>"},
			{"()", "<>"},
		}
		for _, p := range hierarchyPairs {
			if strings.ContainsRune(p.a, left) && strings.ContainsRune(p.b, right) {
				return right
			}
			if strings.ContainsRune(p.a, right) && strings.ContainsRune(p.b, left) {
				return left
			}
		}
	}

	// Rule 4: Opposite bracket/brace/parenthesis smushing
	if (b.Font.SmushMode & SmPair) != 0 {
		if (left == '[' && right == ']') || (left == '{' && right == '}') || (left == '(' && right == ')') ||
			(right == '[' && left == ']') || (right == '{' && left == '}') || (right == '(' && left == ')') {
			return '|'
		}
	}

	// Rule 5: Big-X smushing
	if (b.Font.SmushMode & SmBigX) != 0 {
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

func (b *FigletBuilder) smushAmount(buffer [][]rune, curChar [][]rune) int {
	if (b.Font.SmushMode & (SmSmush | SmKern)) == 0 {
		return 0
	}

	maxSmush := b.CurCharWidth
	for row := 0; row < b.Font.Height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.Direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		end := len(lineLeft)
		for end > 0 && lineLeft[end-1] == ' ' {
			end--
		}
		linebd := end - 1
		if linebd < 0 {
			linebd = 0
		}

		var ch1 rune
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		}

		start := 0
		for start < len(lineRight) && lineRight[start] == ' ' {
			start++
		}
		charbd := start
		var ch2 rune
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		}

		amt := charbd + len(lineLeft) - 1 - linebd

		if ch1 == 0 || ch1 == ' ' {
			amt++
		} else if ch2 != 0 && b.smushRunes(ch1, ch2) != 0 {
			amt++
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

func (b *FigletBuilder) currentSmushAmount(curChar [][]rune) int {
	return b.smushAmount(b.Buffer, curChar)
}

func (b *FigletBuilder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := make([]rune, len(b.Buffer[row]))
	copy(addLeft, b.Buffer[row])

	addRight := make([]rune, len(curChar[row]))
	copy(addRight, curChar[row])

	if b.Direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for i := 0; i < b.MaxSmush; i++ {
		idx := len(addLeft) - b.MaxSmush + i
		var left rune
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		var right rune
		if i < len(addRight) {
			right = addRight[i]
		}
		smushed := b.smushRunes(left, right)
		if smushed != 0 {
			if idx >= 0 && idx < len(addLeft) {
				addLeft[idx] = smushed
			}
		}
	}
	return addLeft, addRight
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	var rightPart []rune
	if b.MaxSmush < len(addRight) {
		rightPart = addRight[b.MaxSmush:]
	}
	combined := make([]rune, 0, len(addLeft)+len(rightPart))
	combined = append(combined, addLeft...)
	combined = append(combined, rightPart...)
	b.Buffer[row] = combined
}

func (b *FigletBuilder) addCharToProduct() {
	if b.Iterator >= len(b.Text) {
		return
	}

	// Handle newline directly
	if b.Text[b.Iterator] == '\n' {
		b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: b.cloneBuffer(), Iterator: b.Iterator})
		b.handleNewLine()
		return
	}

	curChar := b.getCurChar()
	if curChar == nil {
		return
	}

	// In pyfiglet: if width < curWidth, CharNotPrinted error is raised.
	if b.Width < b.getCurWidth() {
		return
	}

	b.CurCharWidth = b.getCurWidth()
	b.MaxSmush = b.currentSmushAmount(curChar)

	buf0Len := 0
	if len(b.Buffer) > 0 {
		buf0Len = len(b.Buffer[0])
	}
	b.TotalWidth = buf0Len + b.CurCharWidth - b.MaxSmush

	if b.Text[b.Iterator] == ' ' {
		b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: b.cloneBuffer(), Iterator: b.Iterator})
	}

	if b.TotalWidth >= b.Width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.Font.Height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}

	b.PrevCharWidth = b.CurCharWidth
}

func (b *FigletBuilder) render() string {
	for b.isNotFinished() {
		b.addCharToProduct()
		b.goToNextChar()
	}

	// Flush remaining buffer if not empty
	if len(b.Buffer) > 0 && len(b.Buffer[0]) > 0 {
		b.Queue = append(b.Queue, b.Buffer)
	}

	var sb strings.Builder
	for _, buf := range b.Queue {
		// Justify
		formattedBuf := make([][]rune, len(buf))
		for r, row := range buf {
			rowCp := make([]rune, len(row))
			copy(rowCp, row)
			formattedBuf[r] = rowCp
		}

		if b.Justify == "right" {
			for row := 0; row < b.Font.Height; row++ {
				spaces := b.Width - len(formattedBuf[row]) - 1
				if spaces > 0 {
					pad := make([]rune, spaces)
					for s := range pad {
						pad[s] = ' '
					}
					formattedBuf[row] = append(pad, formattedBuf[row]...)
				}
			}
		} else if b.Justify == "center" {
			for row := 0; row < b.Font.Height; row++ {
				spaces := (b.Width - len(formattedBuf[row])) / 2
				if spaces > 0 {
					pad := make([]rune, spaces)
					for s := range pad {
						pad[s] = ' '
					}
					formattedBuf[row] = append(pad, formattedBuf[row]...)
				}
			}
		}

		for row := 0; row < b.Font.Height; row++ {
			lineRunes := formattedBuf[row]
			for col := 0; col < len(lineRunes); col++ {
				if lineRunes[col] == b.Font.HardBlank {
					lineRunes[col] = ' '
				}
			}
			sb.WriteString(string(lineRunes))
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

// Reverse transforms the rendered ASCII art horizontally (mirror image).
func reverseArt(s string) string {
	reverseMap := map[rune]rune{
		'(': ')', ')': '(',
		'/': '\\', '\\': '/',
		'<': '>', '>': '<',
		'[': ']', ']': '[',
		'{': '}', '}': '{',
	}

	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var out []string
	for _, line := range lines {
		runes := []rune(line)
		n := len(runes)
		rev := make([]rune, n)
		for i := 0; i < n; i++ {
			r := runes[n-1-i]
			if mapped, ok := reverseMap[r]; ok {
				r = mapped
			}
			rev[i] = r
		}
		out = append(out, string(rev))
	}
	return strings.Join(out, "\n") + "\n"
}

// Flip transforms the rendered ASCII art upside-down.
func flipArt(s string) string {
	flipMap := map[rune]rune{
		'/': '\\', '\\': '/',
		'A': 'V', 'V': 'A',
		'M': 'W', 'W': 'M',
		'P': 'b', 'b': 'P',
		'R': 'b',
		'^': 'v', 'v': '^',
		'_': '-',
		'm': 'w', 'w': 'm',
	}

	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		runes := []rune(line)
		for j, r := range runes {
			if mapped, ok := flipMap[r]; ok {
				runes[j] = mapped
			}
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

// Strip surrounding blank lines.
func stripSurroundingNewlines(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var out []string
	charsSeen := false
	for _, line := range lines {
		if strings.TrimSpace(line) != "" || charsSeen {
			charsSeen = true
			out = append(out, line)
		}
	}

	joined := strings.Join(out, "\n") + "\n"
	return strings.TrimRightFunc(joined, unicode.IsSpace)
}

// Normalize surrounding blank lines (one before, one after).
func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}

func main() {
	// Parse CLI options matching pyfiglet
	fontName := "standard"
	width := 80
	direction := "auto"
	justify := "auto"
	reverse := false
	flip := false
	stripNL := false
	normNL := false

	var textParts []string

	args := os.Args[1:]
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "-f" && i+1 < len(args) {
			fontName = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "-f=") {
			fontName = arg[3:]
			i++
		} else if arg == "--font" && i+1 < len(args) {
			fontName = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "--font=") {
			fontName = arg[7:]
			i++
		} else if arg == "-w" && i+1 < len(args) {
			w, err := strconv.Atoi(args[i+1])
			if err == nil {
				width = w
			}
			i += 2
		} else if strings.HasPrefix(arg, "-w=") {
			w, err := strconv.Atoi(arg[3:])
			if err == nil {
				width = w
			}
			i++
		} else if arg == "--width" && i+1 < len(args) {
			w, err := strconv.Atoi(args[i+1])
			if err == nil {
				width = w
			}
			i += 2
		} else if strings.HasPrefix(arg, "--width=") {
			w, err := strconv.Atoi(arg[8:])
			if err == nil {
				width = w
			}
			i++
		} else if arg == "-j" && i+1 < len(args) {
			justify = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "-j=") {
			justify = arg[3:]
			i++
		} else if arg == "--justify" && i+1 < len(args) {
			justify = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "--justify=") {
			justify = arg[10:]
			i++
		} else if arg == "-D" && i+1 < len(args) {
			direction = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "-D=") {
			direction = arg[3:]
			i++
		} else if arg == "--direction" && i+1 < len(args) {
			direction = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "--direction=") {
			direction = arg[12:]
			i++
		} else if arg == "-r" || arg == "--reverse" {
			reverse = true
			i++
		} else if arg == "-F" || arg == "--flip" {
			flip = true
			i++
		} else if arg == "-s" || arg == "--strip-surrounding-newlines" {
			stripNL = true
			i++
		} else if arg == "-n" || arg == "--normalize-surrounding-newlines" {
			normNL = true
			i++
		} else {
			textParts = append(textParts, arg)
			i++
		}
	}

	text := strings.Join(textParts, " ")

	// Load font
	fontData, err := loadFontData(fontName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: requested font %q not found.\n", fontName)
		os.Exit(1)
	}

	font, err := parseFont(fontName, fontData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: %v\n", err)
		os.Exit(1)
	}

	// Resolve auto direction
	effDirection := direction
	if effDirection == "auto" {
		if font.PrintDirection == 1 {
			effDirection = "right-to-left"
		} else {
			effDirection = "left-to-right"
		}
	}

	// Resolve auto justification
	effJustify := justify
	if effJustify == "auto" {
		if effDirection == "right-to-left" {
			effJustify = "right"
		} else {
			effJustify = "left"
		}
	}

	builder := newFigletBuilder(text, font, effDirection, width, effJustify)
	r := builder.render()

	if reverse {
		r = reverseArt(r)
	}
	if flip {
		r = flipArt(r)
	}
	if stripNL {
		r = stripSurroundingNewlines(r)
	} else if normNL {
		r = normalizeSurroundingNewlines(r)
	}

	// Output exactly matching pyfiglet CLI (writes r then \n)
	var outBuf bytes.Buffer
	outBuf.WriteString(r)
	outBuf.WriteByte('\n')
	os.Stdout.Write(outBuf.Bytes())
}
