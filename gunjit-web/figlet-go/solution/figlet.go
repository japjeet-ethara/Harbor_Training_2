package main

// A line-for-line port of pyfiglet 1.0.4 (FigletFont, FigletBuilder, FigletString).
// Every quirk of the original is kept on purpose: the goal is identical bytes.

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
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

type Font struct {
	height         int
	hardBlank      string // pyfiglet keeps the whole header token, usually one char
	printDirection int    // -1 when the header has no such field
	smushMode      int
	chars          map[rune][][]rune
	width          map[rune]int
}

// pyIsSpace matches Python's str.isspace (unicode White_Space plus \x1c-\x1f).
func pyIsSpace(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

func pyStrip(s string) string  { return strings.TrimFunc(s, pyIsSpace) }
func pyRstrip(s string) string { return strings.TrimRightFunc(s, pyIsSpace) }

// pySplitLines matches Python's str.splitlines(): every line boundary ends a line,
// and a trailing boundary does not produce an extra empty line.
func pySplitLines(s string) []string {
	var out []string
	start := 0
	rs := []rune(s)
	i := 0
	for i < len(rs) {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\u0085', ' ', ' ':
			out = append(out, string(rs[start:i]))
			i++
			start = i
		case '\r':
			out = append(out, string(rs[start:i]))
			i++
			if i < len(rs) && rs[i] == '\n' {
				i++
			}
			start = i
		default:
			i++
		}
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

// decodeReplace mimics bytes.decode('UTF-8', 'replace') closely enough for font files.
func decodeReplace(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		sb.WriteRune(r) // RuneError for an invalid byte
		b = b[size:]
	}
	return sb.String()
}

func allEmpty(rows [][]rune) bool {
	for _, r := range rows {
		if len(r) != 0 {
			return false
		}
	}
	return true
}

func pyInt(s string) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	return v, err
}

// loadFont is FigletFont.loadFont.
func loadFont(name string, raw []byte) (*Font, error) {
	fail := func(e interface{}) (*Font, error) {
		return nil, fmt.Errorf("problem parsing %s font: %v", name, e)
	}
	data := decodeReplace(raw)
	data = strings.NewReplacer("\u0085", " ", " ", " ", " ", " ").Replace(data)
	lines := pySplitLines(data)
	if len(lines) == 0 {
		return fail("empty font")
	}
	header := []rune(lines[0])
	lines = lines[1:]
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || string(header[1:4]) != "lf2" {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}
	fields := strings.Fields(string(header[5:]))
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	f := &Font{hardBlank: fields[0], printDirection: -1, chars: map[rune][][]rune{}, width: map[rune]int{}}
	nums := make([]int, 5)
	for i := range nums {
		v, err := pyInt(fields[1+i])
		if err != nil {
			return fail(err)
		}
		nums[i] = v
	}
	f.height = nums[0]
	oldLayout, commentLines := nums[3], nums[4]
	fullLayout, haveFull := 0, false
	if len(fields) > 6 {
		v, err := pyInt(fields[6])
		if err != nil {
			return fail(err)
		}
		f.printDirection = v
	}
	if len(fields) > 7 {
		v, err := pyInt(fields[7])
		if err != nil {
			return fail(err)
		}
		fullLayout, haveFull = v, true
	}
	if !haveFull {
		switch {
		case oldLayout == 0:
			fullLayout = 64
		case oldLayout < 0:
			fullLayout = 0
		default:
			fullLayout = (oldLayout & 31) | 128
		}
	}
	f.smushMode = fullLayout

	if commentLines > len(lines) {
		return fail("missing comment lines")
	}
	lines = lines[commentLines:]

	// __char: read one glyph of `height` rows, stripping the end marker(s).
	readChar := func() (int, [][]rune, error) {
		var endRe *regexp.Regexp
		width := 0
		rows := make([][]rune, 0, f.height)
		for k := 0; k < f.height; k++ {
			if len(lines) == 0 {
				return 0, nil, errors.New("pop from empty list")
			}
			line := lines[0]
			lines = lines[1:]
			if endRe == nil {
				t := []rune(pyRstrip(line))
				if len(t) == 0 {
					return 0, nil, errors.New("'NoneType' object has no attribute 'group'")
				}
				end := string(t[len(t)-1])
				endRe = regexp.MustCompile(regexp.QuoteMeta(end) + `{1,2}[\s\v]*$`)
			}
			line = endRe.ReplaceAllString(line, "")
			rs := []rune(line)
			if len(rs) > width {
				width = len(rs)
			}
			rows = append(rows, rs)
		}
		return width, rows, nil
	}
	store := func(code rune, w int, rows [][]rune, force bool) {
		if force || !allEmpty(rows) {
			f.chars[code] = rows
			f.width[code] = w
		}
	}
	for i := 32; i < 127; i++ {
		w, rows, err := readChar()
		if err != nil {
			return fail(err)
		}
		store(rune(i), w, rows, i == 32)
	}
	if len(lines) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, rows, err := readChar()
			if err != nil {
				return fail(err)
			}
			store(c, w, rows, false)
		}
	}
	for len(lines) > 0 {
		line := pyStrip(lines[0])
		lines = lines[1:]
		tag, _, _ := strings.Cut(line, " ")
		if tag == "" {
			continue
		}
		if len(tag) >= 2 && (tag[:2] == "0x" || tag[:2] == "0X") {
			code, err := strconv.ParseInt(strings.ReplaceAll(tag[2:], "_", ""), 16, 64)
			if err != nil {
				return fail(err)
			}
			w, rows, err := readChar()
			if err != nil {
				return fail(err)
			}
			store(rune(code), w, rows, false)
		}
	}
	return f, nil
}

type marker struct {
	buf [][]rune
	it  int
}

// builder is FigletBuilder. Rows are never mutated in place, so shallow copies
// of the buffer (blank markers, the product queue) stay valid.
type builder struct {
	text              []rune
	font              *Font
	direction         string
	width             int
	justify           string
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []marker
	queue             [][][]rune
	buffer            [][]rune
}

var errCharNotPrinted = errors.New("CharNotPrinted: Width is not enough to print this character")

func newBuilder(text string, font *Font, direction string, width int, justify string) *builder {
	return &builder{text: []rune(text), font: font, direction: direction, width: width, justify: justify,
		buffer: make([][]rune, font.height)}
}

func copyBuffer(b [][]rune) [][]rune {
	c := make([][]rune, len(b))
	copy(c, b)
	return c
}

func (b *builder) render() (string, error) {
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
		b.iterator++
	}
	return b.returnProduct(), nil
}

func (b *builder) addCharToProduct() error {
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}
	curChar, ok := b.font.chars[b.text[b.iterator]]
	if !ok {
		return nil
	}
	if b.width < b.font.width[b.text[b.iterator]] {
		return errCharNotPrinted
	}
	b.curCharWidth = b.font.width[b.text[b.iterator]]
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		if len(b.buffer[0]) == 0 && len(b.blankMarkers) == 0 {
			// pyfiglet re-reads the same character on an empty buffer forever here.
			return fmt.Errorf("pyfiglet never finishes: %q is %d wide with no leading blank column at width %d",
				string(b.text[b.iterator]), b.curCharWidth, b.width)
		}
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			if err := b.addCurCharRowToBufferRow(curChar, row); err != nil {
				return err
			}
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) returnProduct() string {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		buf = b.justifyString(buf)
		sb.WriteString(b.replaceHardblanks(buf))
	}
	return sb.String()
}

func (b *builder) getCurChar() ([][]rune, bool) {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return nil, false
	}
	c, ok := b.font.chars[b.text[b.iterator]]
	return c, ok
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune, error) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	if b.maxSmush > 0 {
		addLeft = append([]rune(nil), addLeft...)
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		if i >= len(addRight) {
			return nil, nil, errors.New("IndexError: string index out of range")
		}
		right := addRight[i]
		if idx >= 0 && idx < len(addLeft) {
			smushed, ok := b.smushChars(addLeft[idx], right)
			if !ok {
				return nil, nil, errors.New("TypeError: sequence item: expected str instance, NoneType found")
			}
			addLeft[idx] = smushed
		}
	}
	return addLeft, addRight, nil
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) error {
	addLeft, addRight, err := b.smushRow(curChar, row)
	if err != nil {
		return err
	}
	rest := []rune{}
	if b.maxSmush < len(addRight) {
		rest = addRight[b.maxSmush:]
	}
	newRow := make([]rune, 0, len(addLeft)+len(rest))
	newRow = append(newRow, addLeft...)
	newRow = append(newRow, rest...)
	b.buffer[row] = newRow
	return nil
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([][]rune, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, ok := b.getCurChar()
	if !ok {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

func (b *builder) handleNewLine() {
	if len(b.blankMarkers) == 0 {
		b.queue = append(b.queue, b.buffer)
		b.iterator--
		b.cutBufferCommon()
		return
	}
	m := b.blankMarkers[len(b.blankMarkers)-1]
	b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
	b.queue = append(b.queue, m.buf)
	b.iterator = m.it
	b.cutBufferCommon()
}

func spaces(n int) []rune {
	if n <= 0 {
		return nil
	}
	return []rune(strings.Repeat(" ", n))
}

func (b *builder) justifyString(buf [][]rune) [][]rune {
	out := copyBuffer(buf)
	switch b.justify {
	case "right":
		for row := 0; row < b.font.height; row++ {
			out[row] = append(spaces(b.width-len(buf[row])-1), buf[row]...)
		}
	case "center":
		for row := 0; row < b.font.height; row++ {
			out[row] = append(spaces((b.width-len(buf[row]))/2), buf[row]...) // Go and Python int() both truncate toward zero
		}
	}
	return out
}

func (b *builder) replaceHardblanks(buf [][]rune) string {
	parts := make([]string, len(buf))
	for i, r := range buf {
		parts[i] = string(r)
	}
	s := strings.Join(parts, "\n") + "\n"
	return strings.ReplaceAll(s, b.font.hardBlank, " ")
}

const none rune = -1 // Python's '' / None in smushAmount and smushChars

func rstripSpaces(r []rune) int {
	n := len(r)
	for n > 0 && r[n-1] == ' ' {
		n--
	}
	return n
}

func lstripSpaces(r []rune) int {
	n := 0
	for n < len(r) && r[n] == ' ' {
		n++
	}
	return n
}

func (b *builder) smushAmount(buffer [][]rune, curChar [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}
		linebd := rstripSpaces(lineLeft) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := none
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}
		charbd := lstripSpaces(lineRight)
		ch2 := none
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
		}
		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == none || ch1 == ' ' {
			amt++
		} else if ch2 != none {
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

func (b *builder) isHardBlank(r rune) bool { return string(r) == b.font.hardBlank }

// smushChars returns the merged character, or ok=false where pyfiglet returns None.
func (b *builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return none, false
	}
	mode := b.font.smushMode
	if mode&smSmush == 0 {
		return none, false
	}
	if mode&63 == 0 {
		if b.isHardBlank(left) {
			return right, true
		}
		if b.isHardBlank(right) {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if mode&smHardblank != 0 && b.isHardBlank(left) && b.isHardBlank(right) {
		return left, true
	}
	if b.isHardBlank(left) || b.isHardBlank(right) {
		return none, false
	}
	if mode&smEqual != 0 && left == right {
		return left, true
	}
	var smushes [][2]string
	if mode&smLowline != 0 {
		smushes = append(smushes, [2]string{"_", `|/\[]{}()<>`})
	}
	if mode&smHierarchy != 0 {
		smushes = append(smushes,
			[2]string{"|", `/\[]{}()<>`},
			[2]string{`\/`, "[]{}()<>"},
			[2]string{"[]", "{}()<>"},
			[2]string{"{}", "()<>"},
			[2]string{"()", "<>"},
		)
	}
	for _, s := range smushes {
		if strings.ContainsRune(s[0], left) && strings.ContainsRune(s[1], right) {
			return right, true
		}
		if strings.ContainsRune(s[0], right) && strings.ContainsRune(s[1], left) {
			return left, true
		}
	}
	if mode&smPair != 0 {
		for _, pair := range []string{string([]rune{left, right}), string([]rune{right, left})} {
			if pair == "[]" || pair == "{}" || pair == "()" {
				return '|', true
			}
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
	return none, false
}

// FigletString post-processing (reverse, flip, strip, normalize).

var reverseMap = map[rune]rune{'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<', '[': ']', ']': '[', '{': '}', '}': '{'}

var flipMap = map[rune]rune{'/': '\\', '\\': '/', 'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M', 'P': 'b', 'R': 'b', 'b': 'P',
	'^': 'v', 'v': '^', '_': '-', 'm': 'w', 'w': 'm'}

func translate(row string, table map[rune]rune) []rune {
	rs := []rune(row)
	for i, r := range rs {
		if t, ok := table[r]; ok {
			rs[i] = t
		}
	}
	return rs
}

func newFromList(rows []string) string { return strings.Join(rows, "\n") + "\n" }

func figReverse(s string) string {
	var out []string
	for _, row := range pySplitLines(s) {
		rs := translate(row, reverseMap)
		for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
			rs[i], rs[j] = rs[j], rs[i]
		}
		out = append(out, string(rs))
	}
	return newFromList(out)
}

func figFlip(s string) string {
	rows := pySplitLines(s)
	var out []string
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, string(translate(rows[i], flipMap)))
	}
	return newFromList(out)
}

func figStrip(s string) string {
	var out []string
	seen := false
	for _, row := range pySplitLines(s) {
		if pyStrip(row) != "" || seen {
			seen = true
			out = append(out, row)
		}
	}
	return pyRstrip(newFromList(out))
}

func figNormalize(s string) string { return "\n" + figStrip(s) + "\n" }
