package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

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
	name           string
	hardblank      rune
	height         int
	printDirection int
	smushMode      int
	glyphs         map[rune][]string
	widths         map[rune]int
}

type marker struct {
	buffer []string
	index  int
}

type builder struct {
	text              []rune
	face              *font
	direction         string
	justify           string
	width             int
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []marker
	product           [][]string
	buffer            []string
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
	text      []string
}

//go:embed pyfiglet/fonts
var embeddedFonts embed.FS

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if len(opts.text) == 0 {
		fmt.Fprintln(os.Stderr, "text is required")
		os.Exit(2)
	}
	face, err := loadFont(opts.font)
	if err != nil {
		fmt.Fprintf(os.Stderr, "font %q: %v\n", opts.font, err)
		os.Exit(1)
	}
	direction := opts.direction
	if direction == "auto" {
		direction = "left-to-right"
		if face.printDirection == 1 {
			direction = "right-to-left"
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

	rendered, err := render(strings.Join(opts.text, " "), face, direction, justify, opts.width)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.reverse {
		rendered = reverseArt(rendered)
	}
	if opts.flip {
		rendered = flipArt(rendered)
	}
	if opts.strip {
		rendered = stripSurroundingNewlines(rendered)
	} else if opts.normalize {
		rendered = "\n" + stripSurroundingNewlines(rendered) + "\n"
	}
	_, _ = fmt.Fprint(os.Stdout, rendered+"\n")
}

func parseOptions(args []string) (options, error) {
	opts := options{
		font:      "standard",
		direction: "auto",
		justify:   "auto",
		width:     80,
	}
	takeValue := func(i *int, option, inline string) (string, error) {
		if inline != "" {
			return inline, nil
		}
		*i++
		if *i >= len(args) {
			return "", fmt.Errorf("option %s requires a value", option)
		}
		return args[*i], nil
	}
	optionsEnded := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if optionsEnded {
			opts.text = append(opts.text, arg)
			continue
		}
		if arg == "--" {
			optionsEnded = true
			continue
		}
		name, inline, hasInline := strings.Cut(arg, "=")
		switch name {
		case "-f", "--font":
			value, err := takeValue(&i, name, inline)
			if err != nil {
				return opts, err
			}
			opts.font = value
		case "-w", "--width":
			value, err := takeValue(&i, name, inline)
			if err != nil {
				return opts, err
			}
			width, err := strconv.Atoi(value)
			if err != nil {
				return opts, fmt.Errorf("invalid width %q", value)
			}
			opts.width = width
		case "-D", "--direction":
			value, err := takeValue(&i, name, inline)
			if err != nil {
				return opts, err
			}
			if value != "auto" && value != "left-to-right" && value != "right-to-left" {
				return opts, fmt.Errorf("invalid direction %q", value)
			}
			opts.direction = value
		case "-j", "--justify":
			value, err := takeValue(&i, name, inline)
			if err != nil {
				return opts, err
			}
			if value != "auto" && value != "left" && value != "center" && value != "right" {
				return opts, fmt.Errorf("invalid justification %q", value)
			}
			opts.justify = value
		case "-r":
			if hasInline {
				return opts, fmt.Errorf("option -r does not take a value")
			}
			opts.reverse = true
		case "-F":
			if hasInline {
				return opts, fmt.Errorf("option -F does not take a value")
			}
			opts.flip = true
		case "-s":
			if hasInline {
				return opts, fmt.Errorf("option -s does not take a value")
			}
			opts.strip = true
		case "-n":
			if hasInline {
				return opts, fmt.Errorf("option -n does not take a value")
			}
			opts.normalize = true
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown option %q", arg)
			}
			opts.text = append(opts.text, arg)
		}
	}
	return opts, nil
}

func loadFont(name string) (*font, error) {
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return nil, fmt.Errorf("invalid font name")
	}
	var data []byte
	var readErr error
	for _, extension := range []string{".tlf", ".flf"} {
		data, readErr = fs.ReadFile(embeddedFonts, "pyfiglet/fonts/"+name+extension)
		if readErr == nil {
			return parseFont(name, string(data))
		}
	}
	return nil, fmt.Errorf("not found: %w", readErr)
}

func parseFont(name, data string) (*font, error) {
	data = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(data)
	lines := splitFontLines(data)
	if len(lines) == 0 || len(lines[0]) < 5 ||
		(lines[0][0:1] != "f" && lines[0][0:1] != "t") ||
		lines[0][1:4] != "lf2" {
		return nil, fmt.Errorf("invalid FIGfont header")
	}
	header := strings.Fields(lines[0][5:])
	if len(header) < 6 {
		return nil, fmt.Errorf("malformed FIGfont header")
	}
	height, err := strconv.Atoi(header[1])
	if err != nil || height <= 0 {
		return nil, fmt.Errorf("invalid FIGfont height")
	}
	oldLayout, err := strconv.Atoi(header[4])
	if err != nil {
		return nil, fmt.Errorf("invalid FIGfont layout")
	}
	commentLines, err := strconv.Atoi(header[5])
	if err != nil || commentLines < 0 || commentLines >= len(lines) {
		return nil, fmt.Errorf("invalid FIGfont comment count")
	}
	var printDirection *int
	if len(header) > 6 {
		value, parseErr := strconv.Atoi(header[6])
		if parseErr != nil {
			return nil, fmt.Errorf("invalid FIGfont print direction")
		}
		printDirection = &value
	}
	fullLayout := 0
	if len(header) > 7 {
		fullLayout, err = strconv.Atoi(header[7])
		if err != nil {
			return nil, fmt.Errorf("invalid FIGfont full layout")
		}
	} else if oldLayout == 0 {
		fullLayout = 64
	} else if oldLayout < 0 {
		fullLayout = 0
	} else {
		fullLayout = (oldLayout & 31) | 128
	}
	hardblankRunes := []rune(header[0])
	if len(hardblankRunes) == 0 {
		return nil, fmt.Errorf("invalid FIGfont hardblank")
	}
	direction := -1
	if printDirection != nil {
		direction = *printDirection
	}
	face := &font{
		name:           name,
		hardblank:      hardblankRunes[0],
		height:         height,
		printDirection: direction,
		smushMode:      fullLayout,
		glyphs:         make(map[rune][]string),
		widths:         make(map[rune]int),
	}
	lines = lines[1+commentLines:]
	nextGlyph := func() ([]string, int, error) {
		if len(lines) < height {
			return nil, 0, fmt.Errorf("incomplete glyph")
		}
		glyphLines := make([]string, height)
		width := 0
		marker := rune(0)
		for row := 0; row < height; row++ {
			line := []rune(lines[row])
			end := len(line) - 1
			for end >= 0 && isUnicodeSpace(line[end]) {
				end--
			}
			if end < 0 {
				return nil, 0, fmt.Errorf("missing glyph end marker")
			}
			if row == 0 {
				marker = line[end]
			}
			start := end
			for start > 0 && start > end-2 && line[start-1] == marker {
				start--
			}
			visible := string(line[:start])
			glyphLines[row] = visible
			if n := utf8.RuneCountInString(visible); n > width {
				width = n
			}
		}
		lines = lines[height:]
		return glyphLines, width, nil
	}
	for code := rune(32); code <= 126; code++ {
		glyph, width, err := nextGlyph()
		if err != nil {
			return nil, err
		}
		if code == ' ' || strings.Join(glyph, "") != "" {
			face.glyphs[code] = glyph
			face.widths[code] = width
		}
	}
	if len(lines) > 0 {
		for _, code := range []rune("ÄÖÜäöüß") {
			glyph, width, err := nextGlyph()
			if err != nil {
				return nil, err
			}
			if strings.Join(glyph, "") != "" {
				face.glyphs[code] = glyph
				face.widths[code] = width
			}
		}
	}
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 || !strings.HasPrefix(strings.ToLower(parts[0]), "0x") {
			continue
		}
		code, err := strconv.ParseInt(parts[0][2:], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid extended character code")
		}
		glyph, width, err := nextGlyph()
		if err != nil {
			return nil, err
		}
		if strings.Join(glyph, "") != "" {
			face.glyphs[rune(code)] = glyph
			face.widths[rune(code)] = width
		}
	}
	return face, nil
}

func splitFontLines(data string) []string {
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	lines := strings.Split(data, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func isUnicodeSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' ||
		r == '\r' || r == 0x85 || r == 0xa0 || r == 0x1680 ||
		(r >= 0x2000 && r <= 0x200a) || r == 0x2028 || r == 0x2029 ||
		r == 0x202f || r == 0x205f || r == 0x3000
}

func render(text string, face *font, direction, justify string, width int) (string, error) {
	b := &builder{
		text:      []rune(text),
		face:      face,
		direction: direction,
		justify:   justify,
		width:     width,
		buffer:    make([]string, face.height),
	}
	for b.iterator < len(b.text) {
		if err := b.addChar(); err != nil {
			return "", err
		}
		b.iterator++
	}
	if len(b.buffer) > 0 && b.buffer[0] != "" {
		b.product = append(b.product, append([]string(nil), b.buffer...))
	}
	var output strings.Builder
	for _, buffer := range b.product {
		for row := range buffer {
			buffer[row] = justifyRow(buffer[row], justify, width)
			buffer[row] = strings.ReplaceAll(buffer[row], string(face.hardblank), " ")
		}
		output.WriteString(strings.Join(buffer, "\n"))
		output.WriteByte('\n')
	}
	return output.String(), nil
}

func (b *builder) addChar() error {
	current := b.currentGlyph()
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{
			buffer: append([]string(nil), b.buffer...),
			index:  b.iterator,
		})
		b.handleNewline()
		return nil
	}
	if current == nil {
		return nil
	}
	b.curCharWidth = b.face.widths[b.text[b.iterator]]
	if b.width < b.curCharWidth {
		return fmt.Errorf("width is not enough to print this character")
	}
	b.maxSmush = b.currentSmushAmount(current)
	b.currentTotalWidth = len([]rune(b.buffer[0])) + b.curCharWidth - b.maxSmush
	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{
			buffer: append([]string(nil), b.buffer...),
			index:  b.iterator,
		})
	}
	if b.currentTotalWidth >= b.width {
		b.handleNewline()
	} else {
		for row := 0; row < b.face.height; row++ {
			b.addCurrentRow(current, row)
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) handleNewline() {
	last := marker{index: -1}
	if n := len(b.blankMarkers); n > 0 {
		last = b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
	}
	if last.index != -1 {
		b.product = append(b.product, last.buffer)
		b.iterator = last.index
		b.cutBufferCommon()
		return
	}
	b.product = append(b.product, append([]string(nil), b.buffer...))
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.face.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	if current := b.currentGlyph(); current != nil {
		b.maxSmush = b.currentSmushAmount(current)
	}
}

func (b *builder) currentGlyph() []string {
	return b.face.glyphs[b.text[b.iterator]]
}

func (b *builder) currentSmushAmount(current []string) int {
	return b.smushAmount(b.buffer, current)
}

func (b *builder) smushAmount(left, right []string) int {
	mode := b.face.smushMode
	if mode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.face.height; row++ {
		lineLeft, lineRight := []rune(left[row]), []rune(right[row])
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}
		linebd := len(trimRightSpaces(lineLeft)) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := rune(0)
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}
		charbd := len(lineRight) - len(trimLeftSpaces(lineRight))
		ch2 := rune(0)
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
		}
		amount := charbd + len(lineLeft) - 1 - linebd
		if ch1 == 0 || ch1 == ' ' {
			amount++
		} else if ch2 != 0 {
			if _, ok := b.smushChars(ch1, ch2); ok {
				amount++
			}
		}
		if amount < maxSmush {
			maxSmush = amount
		}
	}
	return maxSmush
}

func trimRightSpaces(value []rune) []rune {
	end := len(value)
	for end > 0 && value[end-1] == ' ' {
		end--
	}
	return value[:end]
}

func trimLeftSpaces(value []rune) []rune {
	start := 0
	for start < len(value) && value[start] == ' ' {
		start++
	}
	return value[start:]
}

func (b *builder) addCurrentRow(current []string, row int) {
	left, right := []rune(b.buffer[row]), []rune(current[row])
	if b.direction == "right-to-left" {
		left, right = right, left
	}
	for i := 0; i < b.maxSmush; i++ {
		index := len(left) - b.maxSmush + i
		var leftChar rune
		if index >= 0 && index < len(left) {
			leftChar = left[index]
		}
		if i >= len(right) {
			continue
		}
		smushed, ok := b.smushChars(leftChar, right[i])
		if ok && index >= 0 && index < len(left) {
			left[index] = smushed
		}
	}
	if b.maxSmush < len(right) {
		b.buffer[row] = string(left) + string(right[b.maxSmush:])
	} else {
		b.buffer[row] = string(left)
	}
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
	mode := b.face.smushMode
	if mode&smSmush == 0 {
		return 0, false
	}
	if mode&63 == 0 {
		if left == b.face.hardblank {
			return right, true
		}
		if right == b.face.hardblank {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if mode&smHardblank != 0 && left == b.face.hardblank && right == b.face.hardblank {
		return left, true
	}
	if left == b.face.hardblank || right == b.face.hardblank {
		return 0, false
	}
	if mode&smEqual != 0 && left == right {
		return left, true
	}
	if mode&smLowline != 0 {
		if left == '_' && strings.ContainsRune("|/\\[]{}()<>", right) {
			return right, true
		}
		if right == '_' && strings.ContainsRune("|/\\[]{}()<>", left) {
			return left, true
		}
	}
	if mode&smHierarchy != 0 {
		pairs := [][2]string{
			{"|", "/\\[]{}()<>"},
			{"\\/", "[]{}()<>"},
			{"[]", "{}()<>"},
			{"{}", "()<>"},
			{"()", "<>"},
		}
		for _, pair := range pairs {
			if strings.ContainsRune(pair[0], left) && strings.ContainsRune(pair[1], right) {
				return right, true
			}
			if strings.ContainsRune(pair[0], right) && strings.ContainsRune(pair[1], left) {
				return left, true
			}
		}
	}
	if mode&smPair != 0 {
		pair := string([]rune{left, right})
		reversePair := string([]rune{right, left})
		if pair == "[]" || pair == "{}" || pair == "()" ||
			reversePair == "[]" || reversePair == "{}" || reversePair == "()" {
			return '|', true
		}
	}
	if mode&smBigX != 0 {
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

func justifyRow(row, justify string, width int) string {
	length := utf8.RuneCountInString(row)
	switch justify {
	case "right":
		padding := width - length - 1
		if padding > 0 {
			return strings.Repeat(" ", padding) + row
		}
	case "center":
		padding := (width - length) / 2
		if padding > 0 {
			return strings.Repeat(" ", padding) + row
		}
	}
	return row
}

func reverseArt(art string) string {
	rows := splitRenderedLines(art)
	for i, row := range rows {
		runes := []rune(translate(row, reverseMap))
		for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
			runes[left], runes[right] = runes[right], runes[left]
		}
		rows[i] = string(runes)
	}
	return strings.Join(rows, "\n") + "\n"
}

func flipArt(art string) string {
	rows := splitRenderedLines(art)
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	for i := range rows {
		rows[i] = translate(rows[i], flipMap)
	}
	return strings.Join(rows, "\n") + "\n"
}

func splitRenderedLines(art string) []string {
	if art == "" {
		return nil
	}
	art = strings.TrimSuffix(art, "\n")
	return strings.Split(art, "\n")
}

func translate(value, table string) string {
	runes := []rune(value)
	mapping := []rune(table)
	for i, r := range runes {
		if r >= 0 && int(r) < len(mapping) {
			runes[i] = mapping[r]
		}
	}
	return string(runes)
}

func stripSurroundingNewlines(art string) string {
	rows := splitRenderedLines(art)
	result := make([]string, 0, len(rows))
	seenContent := false
	for _, row := range rows {
		if strings.TrimSpace(row) != "" || seenContent {
			seenContent = true
			result = append(result, row)
		}
	}
	return strings.TrimRightFunc(strings.Join(result, "\n"), isUnicodeSpace)
}

const reverseMap = "" +
	"\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
	"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
	" !\"#$%&')(*+,-.\\" +
	"0123456789:;>=<?" +
	"@ABCDEFGHIJKLMNO" +
	"PQRSTUVWXYZ]/[^_" +
	"`abcdefghijklmno" +
	"pqrstuvwxyz}|{~\x7f"

const flipMap = "" +
	"\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
	"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
	" !\"#$%&'()*+,-.\\" +
	"0123456789:;<=>?" +
	"@VBCDEFGHIJKLWNO" +
	"bQbSTUAMXYZ[/]v-" +
	"`aPcdefghijklwno" +
	"pqrstu^mxyz{|}~\x7f"
