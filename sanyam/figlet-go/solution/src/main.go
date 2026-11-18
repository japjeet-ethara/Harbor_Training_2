// A Go port of pyfiglet 1.0.4's FIGlet renderer (FigletFont, FigletBuilder, FigletString
// and main in pyfiglet/__init__.py), byte for byte. Fonts are embedded from fonts/.
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

// ---------------------------------------------------------------- Python string helpers

// pyIsSpace is str.isspace / regex \s: unicode White_Space plus the \x1c-\x1f separators.
func pyIsSpace(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

func isLineBreak(r rune) bool {
	return r == '\n' || r == '\r' || r == '\v' || r == '\f' || r == 0x1c || r == 0x1d || r == 0x1e ||
		r == 0x85 || r == 0x2028 || r == 0x2029
}

// pySplitLines is str.splitlines(): no trailing empty element after a final line break.
func pySplitLines(s string) []string {
	var out []string
	rs := []rune(s)
	start := 0
	for i := 0; i < len(rs); i++ {
		if isLineBreak(rs[i]) {
			out = append(out, string(rs[start:i]))
			if rs[i] == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

func rstripSpaces(r []rune) []rune {
	n := len(r)
	for n > 0 && r[n-1] == ' ' {
		n--
	}
	return r[:n]
}

func lstripSpaces(r []rune) []rune {
	i := 0
	for i < len(r) && r[i] == ' ' {
		i++
	}
	return r[i:]
}

func allSpace(r []rune) bool {
	for _, c := range r {
		if !pyIsSpace(c) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- FigletFont

type figletFont struct {
	hardBlank      string
	height         int
	printDirection int // -1 when the header has none (Python: None)
	smushMode      int
	chars          map[rune][][]rune
	width          map[rune]int
}

type fontError string

func (e fontError) Error() string { return string(e) }

// stripEndMarker is re.sub(re.escape(end) + r'{1,2}\s*$', '', line): the leftmost run of
// one or two end markers followed only by whitespace is removed.
func stripEndMarker(line []rune, end rune) []rune {
	for p := 0; p < len(line); p++ {
		if line[p] != end {
			continue
		}
		if p+1 < len(line) && line[p+1] == end && allSpace(line[p+2:]) {
			return line[:p]
		}
		if allSpace(line[p+1:]) {
			return line[:p]
		}
	}
	return line
}

func loadFont(name string, raw []byte) (*figletFont, error) {
	data := strings.NewReplacer("\u0085", " ", " ", " ", " ", " ").Replace(string(raw))
	lines := pySplitLines(data)
	if len(lines) == 0 {
		return nil, fontError(name + " is not a valid figlet font")
	}
	header := []rune(lines[0])
	lines = lines[1:]
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || string(header[1:4]) != "lf2" {
		return nil, fontError(name + " is not a valid figlet font")
	}
	fields := strings.Fields(string(header[5:]))
	if len(fields) < 6 {
		return nil, fontError("malformed header for " + name)
	}
	nums := make([]int, len(fields))
	for i := 1; i < len(fields) && i < 8; i++ {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			return nil, fontError("problem parsing " + name + " font: " + err.Error())
		}
		nums[i] = n
	}
	f := &figletFont{
		hardBlank:      fields[0],
		height:         nums[1],
		printDirection: -1,
		chars:          map[rune][][]rune{},
		width:          map[rune]int{},
	}
	oldLayout, commentLines := nums[4], nums[5]
	if len(fields) > 6 {
		f.printDirection = nums[6]
	}
	if len(fields) > 7 {
		f.smushMode = nums[7]
	} else if oldLayout == 0 {
		f.smushMode = 64
	} else if oldLayout < 0 {
		f.smushMode = 0
	} else {
		f.smushMode = (oldLayout & 31) | 128
	}
	if commentLines > len(lines) {
		return nil, fontError("problem parsing " + name + " font: pop from empty list")
	}
	lines = lines[commentLines:]

	// __char: one glyph of `height` rows, end markers stripped.
	readChar := func() (int, [][]rune, error) {
		var end rune
		haveEnd := false
		width := 0
		rows := make([][]rune, 0, f.height)
		for i := 0; i < f.height; i++ {
			if len(lines) == 0 {
				return 0, nil, fontError("problem parsing " + name + " font: pop from empty list")
			}
			line := []rune(lines[0])
			lines = lines[1:]
			if !haveEnd {
				// reEndMarker (.)\s*$ : the first character followed only by whitespace.
				j := len(line)
				for j > 0 && pyIsSpace(line[j-1]) {
					j--
				}
				if j > 0 {
					end = line[j-1]
				} else if len(line) > 0 {
					end = line[0]
				} else {
					return 0, nil, fontError("problem parsing " + name + " font: 'NoneType' object has no attribute 'group'")
				}
				haveEnd = true
			}
			line = stripEndMarker(line, end)
			if len(line) > width {
				width = len(line)
			}
			rows = append(rows, line)
		}
		return width, rows, nil
	}
	nonEmpty := func(rows [][]rune) bool {
		for _, r := range rows {
			if len(r) > 0 {
				return true
			}
		}
		return false
	}

	for i := rune(32); i < 127; i++ {
		w, rows, err := readChar()
		if err != nil {
			return nil, err
		}
		if i == 32 || nonEmpty(rows) {
			f.chars[i], f.width[i] = rows, w
		}
	}
	if len(lines) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, rows, err := readChar()
			if err != nil {
				return nil, err
			}
			if nonEmpty(rows) {
				f.chars[c], f.width[c] = rows, w
			}
		}
	}
	for len(lines) > 0 {
		line := strings.TrimFunc(lines[0], pyIsSpace)
		lines = lines[1:]
		tag, _, _ := strings.Cut(line, " ")
		if tag == "" {
			continue
		}
		if strings.HasPrefix(tag, "0x") || strings.HasPrefix(tag, "0X") {
			code, err := strconv.ParseInt(strings.ReplaceAll(tag[2:], "_", ""), 16, 64)
			if err != nil {
				return nil, fontError("problem parsing " + name + " font: " + err.Error())
			}
			w, rows, err := readChar()
			if err != nil {
				return nil, err
			}
			if nonEmpty(rows) {
				f.chars[rune(code)], f.width[rune(code)] = rows, w
			}
		}
	}
	return f, nil
}

// ---------------------------------------------------------------- FigletBuilder

type blankMarker struct {
	buffer [][]rune
	iter   int
}

type builder struct {
	text      []rune
	font      *figletFont
	rtl       bool
	width     int
	justify   string
	iterator  int
	maxSmush  int
	curW      int
	prevW     int
	totalW    int
	blanks    []blankMarker
	queue     [][][]rune
	buffer    [][]rune
}

type charNotPrinted struct{}

func (charNotPrinted) Error() string { return "Width is not enough to print this character" }

func newBuilder(text string, font *figletFont, rtl bool, width int, justify string) *builder {
	b := &builder{text: []rune(text), font: font, rtl: rtl, width: width, justify: justify}
	b.buffer = make([][]rune, font.height)
	return b
}

func (b *builder) render() (string, error) {
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
		b.iterator++
	}
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		b.justifyBuffer(buf)
		s := strings.Join(rowsToStrings(buf), "\n") + "\n"
		sb.WriteString(strings.ReplaceAll(s, b.font.hardBlank, " "))
	}
	return sb.String(), nil
}

func rowsToStrings(rows [][]rune) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = string(r)
	}
	return out
}

func (b *builder) charAt(i int) ([][]rune, int, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, 0, false
	}
	rows, ok := b.font.chars[b.text[i]]
	return rows, b.font.width[b.text[i]], ok
}

func (b *builder) copyBuffer() [][]rune {
	c := make([][]rune, len(b.buffer))
	copy(c, b.buffer) // rows are never changed in place, only replaced
	return c
}

func (b *builder) addCharToProduct() error {
	cur, w, ok := b.charAt(b.iterator)
	if b.text[b.iterator] == '\n' {
		b.blanks = append(b.blanks, blankMarker{b.copyBuffer(), b.iterator})
		b.handleNewLine()
		return nil
	}
	if !ok {
		return nil
	}
	if b.width < w {
		return charNotPrinted{}
	}
	b.curW = w
	b.maxSmush = b.smushAmount(b.buffer, cur)
	b.totalW = len(b.buffer[0]) + b.curW - b.maxSmush
	if b.text[b.iterator] == ' ' {
		b.blanks = append(b.blanks, blankMarker{b.copyBuffer(), b.iterator})
	}
	if b.totalW >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addRow(cur, row)
		}
	}
	b.prevW = b.curW // also after a line cut, exactly as pyfiglet does
	return nil
}

// addRow is smushRow + addCurCharRowToBufferRow.
func (b *builder) addRow(cur [][]rune, row int) {
	addLeft, addRight := b.buffer[row], cur[row]
	if b.rtl {
		addLeft, addRight = addRight, addLeft
	}
	left := append([]rune(nil), addLeft...)
	for i := 0; i < b.maxSmush; i++ {
		idx := len(left) - b.maxSmush + i
		right := addRight[i] // IndexError in pyfiglet when the row is too short: crash too
		if idx >= 0 && idx < len(left) {
			smushed, ok := b.smushChars(left[idx], right)
			if !ok {
				panic("TypeError: sequence item is None") // ''.join([... None ...])
			}
			left[idx] = smushed
		}
	}
	b.buffer[row] = append(left, addRight[b.maxSmush:]...)
}

func (b *builder) handleNewLine() {
	if n := len(b.blanks); n > 0 {
		m := b.blanks[n-1]
		b.blanks = b.blanks[:n-1]
		b.queue = append(b.queue, m.buffer)
		b.iterator = m.iter
	} else {
		b.queue = append(b.queue, b.buffer)
		b.iterator--
	}
	// cutBufferCommon (its smushAmount call has no effect and is left out)
	b.totalW = 0
	b.buffer = make([][]rune, b.font.height)
	b.blanks = nil
	b.prevW = 0
}

func (b *builder) justifyBuffer(buf [][]rune) {
	for row := range buf {
		pad := 0
		switch b.justify {
		case "right":
			pad = b.width - len(buf[row]) - 1
		case "center":
			pad = (b.width - len(buf[row])) / 2
		}
		if pad > 0 {
			buf[row] = append([]rune(strings.Repeat(" ", pad)), buf[row]...)
		}
	}
}

func (b *builder) smushAmount(buffer, cur [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curW
	for row := 0; row < b.font.height; row++ {
		lineLeft, lineRight := buffer[row], cur[row]
		if b.rtl {
			lineLeft, lineRight = lineRight, lineLeft
		}
		linebd := len(rstripSpaces(lineLeft)) - 1
		if linebd < 0 {
			linebd = 0
		}
		var ch1, ch2 rune
		hasCh1, hasCh2 := false, false
		if linebd < len(lineLeft) {
			ch1, hasCh1 = lineLeft[linebd], true
		} else {
			linebd = 0
		}
		charbd := len(lineRight) - len(lstripSpaces(lineRight))
		if charbd < len(lineRight) {
			ch2, hasCh2 = lineRight[charbd], true
		} else {
			charbd = len(lineRight)
		}
		amt := charbd + len(lineLeft) - 1 - linebd
		if !hasCh1 || ch1 == ' ' {
			amt++
		} else if hasCh2 {
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

// smushChars returns the character the two touching edges become, or false for "None".
func (b *builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.prevW < 2 || b.curW < 2 {
		return 0, false
	}
	mode := b.font.smushMode
	if mode&smSmush == 0 {
		return 0, false
	}
	hb := b.font.hardBlank
	l, r := string(left), string(right)
	if mode&63 == 0 { // universal smushing
		if l == hb {
			return right, true
		}
		if r == hb {
			return left, true
		}
		if b.rtl {
			return left, true
		}
		return right, true
	}
	if mode&smHardblank != 0 && l == hb && r == hb {
		return left, true
	}
	if l == hb || r == hb {
		return 0, false
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
			[2]string{"()", "<>"})
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
		for _, pair := range []string{l + r, r + l} {
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
	return 0, false
}

// ---------------------------------------------------------------- FigletString

var reverseMap = map[rune]rune{'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<',
	'[': ']', ']': '[', '{': '}', '}': '{'}

var flipMap = map[rune]rune{'/': '\\', '\\': '/', 'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M',
	'P': 'b', 'R': 'b', 'b': 'P', '^': 'v', 'v': '^', '_': '-', 'm': 'w', 'w': 'm'}

func translate(row string, m map[rune]rune) []rune {
	rs := []rune(row)
	for i, c := range rs {
		if t, ok := m[c]; ok {
			rs[i] = t
		}
	}
	return rs
}

func newFromList(rows []string) string { return strings.Join(rows, "\n") + "\n" }

func reverse(s string) string {
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

func flip(s string) string {
	rows := pySplitLines(s)
	var out []string
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, string(translate(rows[i], flipMap)))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s string) string {
	var out []string
	seen := false
	for _, row := range pySplitLines(s) {
		if seen || strings.TrimFunc(row, pyIsSpace) != "" {
			seen = true
			out = append(out, row)
		}
	}
	return strings.TrimRightFunc(newFromList(out), pyIsSpace)
}

// ---------------------------------------------------------------- main

func main() {
	fontName, width, justify, direction := "standard", 80, "auto", "auto"
	var rev, flp, strip, normalize bool
	args := os.Args[1:]
	for len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "-" {
		opt := args[0]
		args = args[1:]
		value := func() string {
			if len(args) == 0 {
				os.Exit(2) // optparse: "option requires an argument"
			}
			v := args[0]
			args = args[1:]
			return v
		}
		switch opt {
		case "--":
			break
		case "-f":
			fontName = value()
		case "-w":
			n, err := strconv.Atoi(value())
			if err != nil {
				os.Exit(2)
			}
			width = n
		case "-j":
			justify = value()
			if justify != "auto" && justify != "left" && justify != "center" && justify != "right" {
				os.Exit(2)
			}
		case "-D":
			direction = value()
			if direction != "auto" && direction != "left-to-right" && direction != "right-to-left" {
				os.Exit(2)
			}
		case "-r":
			rev = true
		case "-F":
			flp = true
		case "-s":
			strip = true
		case "-n":
			normalize = true
		default:
			os.Exit(2) // unknown option, or -c/-L/-l/-i which this port does not implement
		}
		if opt == "--" {
			break
		}
	}
	if len(args) == 0 {
		os.Exit(1) // pyfiglet prints its usage here
	}
	text := strings.Join(args, " ")

	var raw []byte
	var err error
	for _, ext := range []string{"tlf", "flf"} {
		if raw, err = fontFS.ReadFile("fonts/" + fontName + "." + ext); err == nil {
			break
		}
	}
	if err != nil {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", fontName)
		os.Exit(1)
	}
	font, err := loadFont(fontName, raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if direction == "auto" {
		direction = "left-to-right"
		if font.printDirection == 1 {
			direction = "right-to-left"
		}
	}
	if justify == "auto" {
		justify = "left"
		if direction == "right-to-left" {
			justify = "right"
		}
	}
	r, err := newBuilder(text, font, direction == "right-to-left", width, justify).render()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if rev {
		r = reverse(r)
	}
	if flp {
		r = flip(r)
	}
	if strip {
		r = stripSurroundingNewlines(r)
	} else if normalize {
		r = "\n" + stripSurroundingNewlines(r) + "\n"
	}
	os.Stdout.WriteString(r + "\n")
}
