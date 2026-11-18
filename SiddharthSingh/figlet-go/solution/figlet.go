package main

import (
	"embed"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:embed fonts/*.flf
var fontFS embed.FS

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

type Font struct {
	name           string
	hardBlank      rune
	height         int
	baseLine       int
	maxLength      int
	oldLayout      int
	commentLines   int
	printDirection int
	smushMode      int
	chars          map[rune][][]rune
	width          map[rune]int
}

func pyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	return unicode.IsSpace(r)
}

func pySplitLines(s string) []string {
	if s == "" {
		return nil
	}
	rs := []rune(s)
	out := make([]string, 0, 8)
	cur := make([]rune, 0, 64)
	for i := 0; i < len(rs); {
		r := rs[i]
		isBreak := false
		skip := 1
		switch r {
		case '\r':
			isBreak = true
			if i+1 < len(rs) && rs[i+1] == '\n' {
				skip = 2
			}
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			isBreak = true
		}
		if isBreak {
			out = append(out, string(cur))
			cur = cur[:0]
			i += skip
		} else {
			cur = append(cur, r)
			i++
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

func decodeUTF8Replace(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	var b strings.Builder
	b.Grow(len(data))
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
			i++
		} else {
			b.WriteRune(r)
			i += size
		}
	}
	return b.String()
}

func runeLen(s []rune) int { return len(s) }

func rstripSpace(s []rune) []rune {
	i := len(s)
	for i > 0 && s[i-1] == ' ' {
		i--
	}
	return s[:i]
}

func lstripSpace(s []rune) []rune {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return s[i:]
}

func trimRightPySpace(s []rune) []rune {
	i := len(s)
	for i > 0 && pyIsSpace(s[i-1]) {
		i--
	}
	return s[:i]
}

func joined(rows [][]rune) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(string(r))
	}
	return b.String()
}

type charReader struct {
	lines   []string
	idx     int
	height  int
	hardEnd rune
}

func stripEnd(line []rune, end rune) []rune {
	line = trimRightPySpace(line)
	if len(line) > 0 && line[len(line)-1] == end {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == end {
		line = line[:len(line)-1]
	}
	return line
}

func lastNonSpace(line []rune) (rune, bool) {
	for i := len(line) - 1; i >= 0; i-- {
		if !pyIsSpace(line[i]) {
			return line[i], true
		}
	}
	if len(line) > 0 {
		return line[0], true
	}
	return 0, false
}

func (cr *charReader) readChar() (int, [][]rune, error) {
	var end rune
	haveEnd := false
	width := 0
	rows := make([][]rune, 0, cr.height)
	for r := 0; r < cr.height; r++ {
		if cr.idx >= len(cr.lines) {
			return 0, nil, fmt.Errorf("unexpected end of font data")
		}
		line := []rune(cr.lines[cr.idx])
		cr.idx++
		if !haveEnd {
			var ok bool
			end, ok = lastNonSpace(line)
			if !ok {
				return 0, nil, fmt.Errorf("invalid end marker")
			}
			haveEnd = true
		}
		line = stripEnd(line, end)
		if len(line) > width {
			width = len(line)
		}
		rows = append(rows, line)
	}
	return width, rows, nil
}

func loadFont(name string) (*Font, error) {
	fileName := name + ".flf"
	if strings.HasSuffix(name, ".flf") || strings.HasSuffix(name, ".tlf") {
		fileName = name
	}
	data, err := fontFS.ReadFile("fonts/" + fileName)
	if err != nil {
		if !strings.HasSuffix(name, ".flf") && !strings.HasSuffix(name, ".tlf") {
			data, err = fontFS.ReadFile("fonts/" + name + ".tlf")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("font not found: %s", name)
	}

	text := decodeUTF8Replace(data)
	text = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(text)
	lines := pySplitLines(text)
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font: %s", name)
	}

	headerLine := lines[0]
	lines = lines[1:]
	hr := []rune(headerLine)
	if len(hr) < 5 || (hr[0] != 'f' && hr[0] != 't') || string(hr[1:4]) != "lf2" {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}
	fields := strings.Fields(string(hr[5:]))
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	hf := []rune(fields[0])
	if len(hf) == 0 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	hardBlank := hf[0]
	height, err1 := strconv.Atoi(fields[1])
	baseLine, err2 := strconv.Atoi(fields[2])
	maxLength, err3 := strconv.Atoi(fields[3])
	oldLayout, err4 := strconv.Atoi(fields[4])
	commentLines, err5 := strconv.Atoi(fields[5])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	printDirection := 0
	fullLayout := -1
	if len(fields) > 6 {
		v, err := strconv.Atoi(fields[6])
		if err != nil {
			return nil, fmt.Errorf("malformed header for %s", name)
		}
		printDirection = v
	}
	if len(fields) > 7 {
		v, err := strconv.Atoi(fields[7])
		if err != nil {
			return nil, fmt.Errorf("malformed header for %s", name)
		}
		fullLayout = v
	}
	if fullLayout < 0 {
		if oldLayout == 0 {
			fullLayout = 64
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | 128
		}
	}

	if commentLines > len(lines) {
		return nil, fmt.Errorf("problem parsing %s font", name)
	}
	lines = lines[commentLines:]

	f := &Font{
		name:           name,
		hardBlank:      hardBlank,
		height:         height,
		baseLine:       baseLine,
		maxLength:      maxLength,
		oldLayout:      oldLayout,
		commentLines:   commentLines,
		printDirection: printDirection,
		smushMode:      fullLayout,
		chars:          make(map[rune][][]rune),
		width:          make(map[rune]int),
	}
	cr := &charReader{lines: lines, height: height}

	for i := rune(32); i < 127; i++ {
		w, rows, err := cr.readChar()
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
		}
		if i == 32 || joined(rows) != "" {
			f.chars[i] = rows
			f.width[i] = w
		}
	}

	if cr.idx < len(cr.lines) {
		for _, i := range []rune("ÄÖÜäöüß") {
			w, rows, err := cr.readChar()
			if err != nil {
				return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
			}
			if joined(rows) != "" {
				f.chars[i] = rows
				f.width[i] = w
			}
		}
	}

	for cr.idx < len(cr.lines) {
		line := strings.TrimSpace(cr.lines[cr.idx])
		cr.idx++
		if line == "" {
			continue
		}
		code := line
		if sp := strings.IndexByte(line, ' '); sp >= 0 {
			code = line[:sp]
		}
		if code == "" {
			continue
		}
		if len(code) >= 2 && code[0] == '0' && (code[1] == 'x' || code[1] == 'X') {
			v, err := strconv.ParseInt(code, 0, 32)
			if err != nil {
				return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
			}
			w, rows, err := cr.readChar()
			if err != nil {
				return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
			}
			if joined(rows) != "" {
				f.chars[rune(v)] = rows
				f.width[rune(v)] = w
			}
		}
	}

	return f, nil
}

type blankMarker struct {
	buffer   [][]rune
	iterator int
}

type Builder struct {
	text      []rune
	direction string
	width     int
	font      *Font
	justify   string

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []blankMarker
	product      [][][]rune
	buffer       [][]rune
}

func newBuilder(text string, font *Font, direction string, width int, justify string) *Builder {
	b := &Builder{
		text:      []rune(text),
		direction: direction,
		width:     width,
		font:      font,
		justify:   justify,
	}
	b.buffer = makeEmptyBuffer(font.height)
	return b
}

func makeEmptyBuffer(height int) [][]rune {
	buf := make([][]rune, height)
	for i := range buf {
		buf[i] = []rune{}
	}
	return buf
}

func cloneBufferRows(buf [][]rune) [][]rune {
	out := make([][]rune, len(buf))
	copy(out, buf)
	return out
}

func (b *Builder) getCharAt(i int) ([][]rune, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	rows, ok := b.font.chars[b.text[i]]
	return rows, ok
}

func (b *Builder) getCharWidthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	w, ok := b.font.width[b.text[i]]
	return w, ok
}

func (b *Builder) getCurChar() ([][]rune, bool) {
	return b.getCharAt(b.iterator)
}

func (b *Builder) getCurWidth() (int, bool) {
	return b.getCharWidthAt(b.iterator)
}

func (b *Builder) getLeftSmushedChar(i int, addLeft []rune) (rune, bool, int) {
	idx := len(addLeft) - b.maxSmush + i
	if idx >= 0 && idx < len(addLeft) {
		return addLeft[idx], true, idx
	}
	return 0, false, idx
}

func updateSmushedCharInLeftBuffer(addLeft []rune, idx int, smushed rune) []rune {
	if idx < 0 || idx >= len(addLeft) {
		return addLeft
	}
	l := make([]rune, len(addLeft))
	copy(l, addLeft)
	l[idx] = smushed
	return l
}

func (b *Builder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		left, leftOK, idx := b.getLeftSmushedChar(i, addLeft)
		var right rune
		rightOK := false
		if i < len(addRight) {
			right = addRight[i]
			rightOK = true
		}
		if smushed, ok := b.smushChars(left, leftOK, right, rightOK); ok {
			addLeft = updateSmushedCharInLeftBuffer(addLeft, idx, smushed)
		}
	}
	return addLeft, addRight
}

func (b *Builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	start := b.maxSmush
	if start > len(addRight) {
		start = len(addRight)
	}
	next := make([]rune, 0, len(addLeft)+len(addRight)-start)
	next = append(next, addLeft...)
	next = append(next, addRight[start:]...)
	b.buffer[row] = next
}

func (b *Builder) currentSmushAmount(curChar [][]rune) int {
	return b.smushAmount(b.buffer, curChar)
}

func (b *Builder) addCharToProduct() {
	curChar, curOK := b.getCurChar()

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneBufferRows(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if !curOK {
		return
	}
	curWidth, _ := b.getCurWidth()
	if b.width < curWidth {
		return
	}
	b.curCharWidth = curWidth
	b.maxSmush = b.currentSmushAmount(curChar)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{cloneBufferRows(b.buffer), b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		empty := true
		for _, row := range b.buffer {
			if len(row) != 0 {
				empty = false
				break
			}
		}
		hang := empty && b.currentTotalWidth == b.width && b.text[b.iterator] != ' '
		if !hang {
			b.handleNewLine()
			b.prevCharWidth = b.curCharWidth
			return
		}
	}
	for row := 0; row < b.font.height; row++ {
		b.addCurCharRowToBufferRow(curChar, row)
	}
	b.prevCharWidth = b.curCharWidth
}

func (b *Builder) goToNextChar() { b.iterator++ }

func (b *Builder) isNotFinished() bool { return b.iterator < len(b.text) }

func (b *Builder) flushLastBuffer() { b.product = append(b.product, b.buffer) }

func (b *Builder) getLastBlank() (blankMarker, bool) {
	if len(b.blankMarkers) == 0 {
		return blankMarker{}, false
	}
	m := b.blankMarkers[len(b.blankMarkers)-1]
	b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
	return m, true
}

func (b *Builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = makeEmptyBuffer(b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, ok := b.getCurChar()
	if !ok {
		return
	}
	b.maxSmush = b.currentSmushAmount(curChar)
}

func (b *Builder) cutBufferAtLastBlank(savedBuffer [][]rune, savedIterator int) {
	b.product = append(b.product, savedBuffer)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *Builder) cutBufferAtLastChar() {
	b.product = append(b.product, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *Builder) handleNewLine() {
	m, ok := b.getLastBlank()
	if ok {
		b.cutBufferAtLastBlank(m.buffer, m.iterator)
	} else {
		b.cutBufferAtLastChar()
	}
}

func spaces(n int) []rune {
	if n <= 0 {
		return nil
	}
	s := make([]rune, n)
	for i := range s {
		s[i] = ' '
	}
	return s
}

func (b *Builder) justifyString(justify string, buffer [][]rune) [][]rune {
	out := make([][]rune, len(buffer))
	switch justify {
	case "right":
		for row := range buffer {
			n := b.width - len(buffer[row]) - 1
			line := append(spaces(n), buffer[row]...)
			out[row] = line
		}
	case "center":
		for row := range buffer {
			n := (b.width - len(buffer[row])) / 2
			line := append(spaces(n), buffer[row]...)
			out[row] = line
		}
	default:
		for row := range buffer {
			line := make([]rune, len(buffer[row]))
			copy(line, buffer[row])
			out[row] = line
		}
	}
	return out
}

func (b *Builder) replaceHardblanks(buffer [][]rune) string {
	var sb strings.Builder
	for _, row := range buffer {
		for _, r := range row {
			if r == b.font.hardBlank {
				sb.WriteRune(' ')
			} else {
				sb.WriteRune(r)
			}
		}
		sb.WriteRune('\n')
	}
	return sb.String()
}

func (b *Builder) smushAmount(buffer [][]rune, curChar [][]rune) int {
	if (b.font.smushMode & (smSmush | smKern)) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		trimmedLeft := rstripSpace(lineLeft)
		linebd := len(trimmedLeft) - 1
		if linebd < 0 {
			linebd = 0
		}
		var ch1 rune
		ch1OK := false
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
			ch1OK = true
		} else {
			linebd = 0
		}

		trimmedRight := lstripSpace(lineRight)
		charbd := len(lineRight) - len(trimmedRight)
		var ch2 rune
		ch2OK := false
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
			ch2OK = true
		} else {
			charbd = len(lineRight)
		}

		amt := charbd + len(lineLeft) - 1 - linebd
		if !ch1OK || ch1 == ' ' {
			amt++
		} else if ch2OK {
			if _, ok := b.smushChars(ch1, true, ch2, true); ok {
				amt++
			}
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

func (b *Builder) smushChars(left rune, leftOK bool, right rune, rightOK bool) (rune, bool) {
	if leftOK && left == ' ' {
		return right, rightOK
	}
	if rightOK && right == ' ' {
		return left, leftOK
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}
	if (b.font.smushMode & smSmush) == 0 {
		return 0, false
	}
	if (b.font.smushMode & 63) == 0 {
		if leftOK && left == b.font.hardBlank {
			return right, rightOK
		}
		if rightOK && right == b.font.hardBlank {
			return left, leftOK
		}
		if b.direction == "right-to-left" {
			return left, leftOK
		}
		return right, rightOK
	}
	if (b.font.smushMode & smHardblank) != 0 {
		if leftOK && rightOK && left == b.font.hardBlank && right == b.font.hardBlank {
			return left, true
		}
	}
	if (leftOK && left == b.font.hardBlank) || (rightOK && right == b.font.hardBlank) {
		return 0, false
	}
	if (b.font.smushMode & smEqual) != 0 {
		if leftOK && rightOK && left == right {
			return left, true
		}
	}

	type rule struct {
		a string
		b string
	}
	var smushes []rule
	if (b.font.smushMode & smLowline) != 0 {
		smushes = append(smushes, rule{"_", "|/\\[]{}()<>"})
	}
	if (b.font.smushMode & smHierarchy) != 0 {
		smushes = append(smushes,
			rule{"|", "/\\[]{}()<>"},
			rule{"\\/", "[]{}()<>"},
			rule{"[]", "{}()<>"},
			rule{"{}", "()<>"},
			rule{"()", "<>"},
		)
	}
	for _, rl := range smushes {
		if leftOK && rightOK && strings.ContainsRune(rl.a, left) && strings.ContainsRune(rl.b, right) {
			return right, true
		}
		if leftOK && rightOK && strings.ContainsRune(rl.a, right) && strings.ContainsRune(rl.b, left) {
			return left, true
		}
	}
	if (b.font.smushMode&smPair) != 0 && leftOK && rightOK {
		pair := string([]rune{left, right})
		pair2 := string([]rune{right, left})
		if pair == "[]" || pair == "{}" || pair == "()" || pair2 == "[]" || pair2 == "{}" || pair2 == "()" {
			return '|', true
		}
	}
	if (b.font.smushMode&smBigX) != 0 && leftOK && rightOK {
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

func (b *Builder) formatProduct() string {
	var sb strings.Builder
	for _, buffer := range b.product {
		justified := b.justifyString(b.justify, buffer)
		sb.WriteString(b.replaceHardblanks(justified))
	}
	return sb.String()
}

func (b *Builder) returnProduct() string {
	if len(b.buffer) > 0 && len(b.buffer[0]) != 0 {
		b.flushLastBuffer()
	}
	return b.formatProduct()
}

func renderText(text string, font *Font, direction, justify string, width int) string {
	b := newBuilder(text, font, direction, width, justify)
	for b.isNotFinished() {
		b.addCharToProduct()
		b.goToNextChar()
	}
	return b.returnProduct()
}

func newFromList(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

func reverseFiglet(s string) string {
	lines := pySplitLines(s)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		rr := []rune(line)
		for i, r := range rr {
			if r >= 0 && r < 256 {
				rr[i] = reverseMap[r]
			}
		}
		for i, j := 0, len(rr)-1; i < j; i, j = i+1, j-1 {
			rr[i], rr[j] = rr[j], rr[i]
		}
		out = append(out, string(rr))
	}
	return newFromList(out)
}

func flipFiglet(s string) string {
	lines := pySplitLines(s)
	out := make([]string, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		rr := []rune(lines[i])
		for j, r := range rr {
			if r >= 0 && r < 256 {
				rr[j] = flipMap[r]
			}
		}
		out = append(out, string(rr))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s string) string {
	lines := pySplitLines(s)
	out := make([]string, 0, len(lines))
	seen := false
	for _, line := range lines {
		if strings.TrimFunc(line, pyIsSpace) != "" || seen {
			seen = true
			out = append(out, line)
		}
	}
	r := newFromList(out)
	return strings.TrimRightFunc(r, pyIsSpace)
}

func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}
