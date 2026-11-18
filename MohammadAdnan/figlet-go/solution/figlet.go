// Go port of pyfiglet (commit 255827efbc09d662875dad50fa0edc1e1263bf45) FIGlet rendering.
// Standard library only. Ported rule by rule from pyfiglet/__init__.py.
package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode"
)

//go:embed fonts/*.flf
var fontFS embed.FS

var errFontNotFound = errors.New("font not found")

// ---------- Python string helpers ----------

// pyIsSpace matches Python's str.isspace() (and the re module's \s for str patterns).
func pyIsSpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pySplitLines matches Python's str.splitlines().
func pySplitLines(s []rune) [][]rune {
	var out [][]rune
	start := 0
	for i := 0; i < len(s); i++ {
		if isLineBreak(s[i]) {
			out = append(out, s[start:i])
			if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func allSpace(s []rune) bool {
	for _, r := range s {
		if !pyIsSpace(r) {
			return false
		}
	}
	return true
}

func trimSpaceRunes(s []rune) []rune {
	a, b := 0, len(s)
	for a < b && pyIsSpace(s[a]) {
		a++
	}
	for b > a && pyIsSpace(s[b-1]) {
		b--
	}
	return s[a:b]
}

// decodeUTF8Replace matches Python's bytes.decode('UTF-8', 'replace'):
// one U+FFFD per maximal invalid subpart.
func decodeUTF8Replace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	i := 0
	for i < len(b) {
		c := b[i]
		if c < 0x80 {
			out = append(out, rune(c))
			i++
			continue
		}
		var need int
		lo, hi := byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c == 0xE0:
			need, lo = 2, 0xA0
		case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
			need = 2
		case c == 0xED:
			need, hi = 2, 0x9F
		case c == 0xF0:
			need, lo = 3, 0x90
		case c >= 0xF1 && c <= 0xF3:
			need = 3
		case c == 0xF4:
			need, hi = 3, 0x8F
		default:
			out = append(out, 0xFFFD)
			i++
			continue
		}
		j := 1
		ok := true
		for k := 0; k < need; k++ {
			if i+j >= len(b) {
				ok = false
				break
			}
			x := b[i+j]
			l, h := byte(0x80), byte(0xBF)
			if k == 0 {
				l, h = lo, hi
			}
			if x < l || x > h {
				ok = false
				break
			}
			j++
		}
		if !ok {
			out = append(out, 0xFFFD)
			i += j
			continue
		}
		r := rune(c)
		switch need {
		case 1:
			r = r & 0x1F
		case 2:
			r = r & 0x0F
		case 3:
			r = r & 0x07
		}
		for k := 1; k <= need; k++ {
			r = r<<6 | rune(b[i+k]&0x3F)
		}
		out = append(out, r)
		i += need + 1
	}
	return out
}

// pyInt parses like Python int(s) (base 10) / int(s, 16).
func pyInt(s string, base int) (int, error) {
	s = strings.TrimFunc(s, pyIsSpace)
	neg := false
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		neg = s[0] == '-'
		s = s[1:]
	}
	if base == 16 && (strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) {
		s = s[2:]
		s = strings.TrimPrefix(s, "_")
	}
	if s == "" || strings.HasPrefix(s, "_") || strings.HasSuffix(s, "_") || strings.Contains(s, "__") {
		return 0, errors.New("invalid int")
	}
	v, err := strconv.ParseInt(strings.ReplaceAll(s, "_", ""), base, 64)
	if err != nil {
		return 0, err
	}
	if neg {
		v = -v
	}
	return int(v), nil
}

// ---------- FigletFont ----------

type Font struct {
	height         int
	hardBlank      string
	printDirection int // -1 when absent
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

func preloadFont(name string) ([]byte, error) {
	for _, ext := range []string{"tlf", "flf"} {
		data, err := fontFS.ReadFile("fonts/" + name + "." + ext)
		if err != nil {
			continue
		}
		if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil && len(zr.File) > 0 {
			f, err := zr.File[0].Open()
			if err != nil {
				return nil, err
			}
			defer f.Close()
			data, err = io.ReadAll(f)
			if err != nil {
				return nil, err
			}
		}
		if len(data) == 0 {
			return nil, errFontNotFound
		}
		return data, nil
	}
	return nil, errFontNotFound
}

// endMarker matches re.search(r'(.)\s*$', line).group(1).
func endMarker(line []rune) (rune, bool) {
	for i := 0; i < len(line); i++ {
		if allSpace(line[i+1:]) {
			return line[i], true
		}
	}
	return 0, false
}

// stripEnd matches re.sub(re.escape(end) + r'{1,2}\s*$', '', line).
func stripEnd(line []rune, end rune) []rune {
	for i := 0; i < len(line); i++ {
		if line[i] != end {
			continue
		}
		if i+1 < len(line) && line[i+1] == end && allSpace(line[i+2:]) {
			return line[:i]
		}
		if allSpace(line[i+1:]) {
			return line[:i]
		}
	}
	return line
}

func loadFont(name string) (*Font, error) {
	raw, err := preloadFont(name)
	if err != nil {
		return nil, err
	}
	text := decodeUTF8Replace(raw)
	for i, r := range text {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			text[i] = ' '
		}
	}
	lines := pySplitLines(text)
	bad := errors.New("font error")
	if len(lines) == 0 {
		return nil, bad
	}
	header := lines[0]
	lines = lines[1:]
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || header[1] != 'l' || header[2] != 'f' || header[3] != '2' || header[4] == '\n' {
		return nil, bad
	}
	fields := strings.FieldsFunc(string(header[5:]), pyIsSpace)
	if len(fields) < 6 {
		return nil, bad
	}
	f := &Font{hardBlank: fields[0], printDirection: -1, chars: map[int][][]rune{}, width: map[int]int{}}
	nums := make([]int, 5)
	for i := 0; i < 5; i++ {
		if nums[i], err = pyInt(fields[i+1], 10); err != nil {
			return nil, bad
		}
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	fullLayout, hasFull := 0, false
	if len(fields) > 6 {
		if f.printDirection, err = pyInt(fields[6], 10); err != nil {
			return nil, bad
		}
	}
	if len(fields) > 7 {
		if fullLayout, err = pyInt(fields[7], 10); err != nil {
			return nil, bad
		}
		hasFull = true
	}
	if !hasFull {
		if oldLayout == 0 {
			fullLayout = 64
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | 128
		}
	}
	f.height = height
	f.smushMode = fullLayout

	pop := func() ([]rune, bool) {
		if len(lines) == 0 {
			return nil, false
		}
		l := lines[0]
		lines = lines[1:]
		return l, true
	}
	for i := 0; i < commentLines; i++ {
		if _, ok := pop(); !ok {
			return nil, bad
		}
	}
	readChar := func() (int, [][]rune, bool) {
		var end rune
		haveEnd := false
		width := 0
		var rows [][]rune
		for i := 0; i < height; i++ {
			line, ok := pop()
			if !ok {
				return 0, nil, false
			}
			if !haveEnd {
				e, ok := endMarker(line)
				if !ok {
					return 0, nil, false
				}
				end, haveEnd = e, true
			}
			line = stripEnd(line, end)
			if len(line) > width {
				width = len(line)
			}
			rows = append(rows, line)
		}
		return width, rows, true
	}
	nonEmpty := func(rows [][]rune) bool {
		for _, r := range rows {
			if len(r) > 0 {
				return true
			}
		}
		return false
	}
	for i := 32; i < 127; i++ {
		w, rows, ok := readChar()
		if !ok {
			return nil, bad
		}
		if i == 32 || nonEmpty(rows) {
			f.chars[i] = rows
			f.width[i] = w
		}
	}
	if len(lines) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, rows, ok := readChar()
			if !ok {
				return nil, bad
			}
			if nonEmpty(rows) {
				f.chars[int(c)] = rows
				f.width[int(c)] = w
			}
		}
	}
	for len(lines) > 0 {
		line, _ := pop()
		s := string(trimSpaceRunes(line))
		tag := strings.SplitN(s, " ", 2)[0]
		if tag == "" {
			continue
		}
		if len(tag) >= 2 && tag[0] == '0' && (tag[1] == 'x' || tag[1] == 'X') {
			code, err := pyInt(tag, 16)
			if err != nil {
				return nil, bad
			}
			w, rows, ok := readChar()
			if !ok {
				return nil, bad
			}
			if nonEmpty(rows) {
				f.chars[code] = rows
				f.width[code] = w
			}
		}
	}
	return f, nil
}

// ---------- FigletBuilder ----------

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

var errRender = errors.New("render error")

type marker struct {
	buffer   [][]rune
	iterator int
}

type builder struct {
	text      []int
	direction string
	width     int
	font      *Font
	justify   string

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []marker
	queue        [][][]rune
	buffer       [][]rune
}

func newBuilder(text []rune, font *Font, direction string, width int, justify string) *builder {
	b := &builder{direction: direction, width: width, font: font, justify: justify}
	for _, r := range text {
		b.text = append(b.text, int(r))
	}
	b.buffer = make([][]rune, font.height)
	return b
}

func copyBuffer(buf [][]rune) [][]rune {
	out := make([][]rune, len(buf))
	copy(out, buf)
	return out
}

func (b *builder) getCharAt(i int) [][]rune {
	if i < 0 || i >= len(b.text) {
		return nil
	}
	return b.font.chars[b.text[i]]
}

func (b *builder) getCharWidthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	w, ok := b.font.width[b.text[i]]
	return w, ok
}

func (b *builder) addCharToProduct() error {
	curChar := b.getCharAt(b.iterator)
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}
	if curChar == nil {
		return nil
	}
	w, _ := b.getCharWidthAt(b.iterator)
	if b.width < w {
		return errRender // CharNotPrinted
	}
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush
	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
	}
	if b.currentTotalWidth >= b.width {
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

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune, error) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := ""
		if idx >= 0 && idx < len(addLeft) {
			left = string(addLeft[idx])
		}
		if i >= len(addRight) {
			return nil, nil, errRender // IndexError in pyfiglet
		}
		right := string(addRight[i])
		smushed, ok := b.smushChars(left, right)
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if !ok {
			return nil, nil, errRender // TypeError in pyfiglet (None in join)
		}
		nl := make([]rune, 0, len(addLeft))
		nl = append(nl, addLeft[:idx]...)
		nl = append(nl, []rune(smushed)...)
		nl = append(nl, addLeft[idx+1:]...)
		addLeft = nl
	}
	return addLeft, addRight, nil
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) error {
	addLeft, addRight, err := b.smushRow(curChar, row)
	if err != nil {
		return err
	}
	nr := make([]rune, 0, len(addLeft)+len(addRight))
	nr = append(nr, addLeft...)
	if b.maxSmush < len(addRight) {
		nr = append(nr, addRight[b.maxSmush:]...)
	}
	b.buffer[row] = nr
	return nil
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([][]rune, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar := b.getCharAt(b.iterator)
	if curChar == nil {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		if m.iterator != -1 {
			b.queue = append(b.queue, m.buffer)
			b.iterator = m.iterator
			b.cutBufferCommon()
			return
		}
	}
	b.queue = append(b.queue, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) justifyString(buffer [][]rune) [][]rune {
	out := make([][]rune, len(buffer))
	for row := 0; row < b.font.height; row++ {
		pad := 0
		switch b.justify {
		case "right":
			pad = b.width - len(buffer[row]) - 1
		case "center":
			pad = (b.width - len(buffer[row])) / 2
		}
		if pad < 0 {
			pad = 0
		}
		out[row] = append([]rune(strings.Repeat(" ", pad)), buffer[row]...)
	}
	return out
}

func (b *builder) returnProduct() string {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		buf = b.justifyString(buf)
		rows := make([]string, len(buf))
		for i, r := range buf {
			rows[i] = string(r)
		}
		s := strings.Join(rows, "\n") + "\n"
		sb.WriteString(strings.ReplaceAll(s, b.font.hardBlank, " "))
	}
	return sb.String()
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
		trimmed := len(lineLeft)
		for trimmed > 0 && lineLeft[trimmed-1] == ' ' {
			trimmed--
		}
		linebd := trimmed - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := ""
		if linebd < len(lineLeft) {
			ch1 = string(lineLeft[linebd])
		} else {
			linebd = 0
		}
		charbd := 0
		for charbd < len(lineRight) && lineRight[charbd] == ' ' {
			charbd++
		}
		ch2 := ""
		if charbd < len(lineRight) {
			ch2 = string(lineRight[charbd])
		} else {
			charbd = len(lineRight)
		}
		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == "" || ch1 == " " {
			amt++
		} else if ch2 != "" {
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

// pyIn matches Python's `sub in s` (the empty string is in every string).
func pyIn(sub, s string) bool { return strings.Contains(s, sub) }

func (b *builder) smushChars(left, right string) (string, bool) {
	if left == " " {
		return right, true
	}
	if right == " " {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return "", false
	}
	mode := b.font.smushMode
	hb := b.font.hardBlank
	if mode&smSmush == 0 {
		return "", false
	}
	if mode&63 == 0 {
		if left == hb {
			return right, true
		}
		if right == hb {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if mode&smHardblank != 0 {
		if left == hb && right == hb {
			return left, true
		}
	}
	if left == hb || right == hb {
		return "", false
	}
	if mode&smEqual != 0 {
		if left == right {
			return left, true
		}
	}
	type pair struct{ a, b string }
	var smushes []pair
	if mode&smLowline != 0 {
		smushes = append(smushes, pair{"_", `|/\[]{}()<>`})
	}
	if mode&smHierarchy != 0 {
		smushes = append(smushes,
			pair{"|", `/\[]{}()<>`},
			pair{`\/`, "[]{}()<>"},
			pair{"[]", "{}()<>"},
			pair{"{}", "()<>"},
			pair{"()", "<>"},
		)
	}
	for _, p := range smushes {
		if pyIn(left, p.a) && pyIn(right, p.b) {
			return right, true
		}
		if pyIn(right, p.a) && pyIn(left, p.b) {
			return left, true
		}
	}
	if mode&smPair != 0 {
		for _, pr := range []string{left + right, right + left} {
			if pr == "[]" || pr == "{}" || pr == "()" {
				return "|", true
			}
		}
	}
	if mode&smBigX != 0 {
		if left == "/" && right == `\` {
			return "|", true
		}
		if right == "/" && left == `\` {
			return "Y", true
		}
		if left == ">" && right == "<" {
			return "X", true
		}
	}
	return "", false
}

// ---------- Figlet + FigletString ----------

type Figlet struct {
	font      *Font
	direction string
	justify   string
	width     int
}

func newFiglet(fontName, direction, justify string, width int) (*Figlet, error) {
	f, err := loadFont(fontName)
	if err != nil {
		return nil, err
	}
	fg := &Figlet{font: f, width: width}
	if direction == "auto" {
		if f.printDirection == 1 {
			fg.direction = "right-to-left"
		} else {
			fg.direction = "left-to-right"
		}
	} else {
		fg.direction = direction
	}
	if justify == "auto" {
		if fg.direction == "left-to-right" {
			fg.justify = "left"
		} else {
			fg.justify = "right"
		}
	} else {
		fg.justify = justify
	}
	return fg, nil
}

const maxSteps = 50_000_000

func (fg *Figlet) render(text []rune) (string, error) {
	b := newBuilder(text, fg.font, fg.direction, fg.width, fg.justify)
	steps := 0
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
		b.iterator++
		if steps++; steps > maxSteps {
			return "", errRender // pyfiglet never finishes here
		}
	}
	return b.returnProduct(), nil
}

var reverseMap, flipMap [256]rune

func init() {
	for i := range reverseMap {
		reverseMap[i] = rune(i)
		flipMap[i] = rune(i)
	}
	set := func(m *[256]rune, from, to string) {
		f, t := []rune(from), []rune(to)
		for i := range f {
			m[f[i]] = t[i]
		}
	}
	set(&reverseMap, `()/<>[\]{}`, `)(\><]/[}{`)
	set(&flipMap, `/AMPRVW\^_bmvw`, `\VWbbAM/v-Pw^m`)
}

func newFromList(rows [][]rune) []rune {
	var out []rune
	for i, r := range rows {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, r...)
	}
	return append(out, '\n')
}

func translate(row []rune, m *[256]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if r >= 0 && r < 256 {
			out[i] = m[r]
		} else {
			out[i] = r
		}
	}
	return out
}

func reverseFS(s []rune) []rune {
	var rows [][]rune
	for _, row := range pySplitLines(s) {
		t := translate(row, &reverseMap)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		rows = append(rows, t)
	}
	return newFromList(rows)
}

func flipFS(s []rune) []rune {
	lines := pySplitLines(s)
	var rows [][]rune
	for i := len(lines) - 1; i >= 0; i-- {
		rows = append(rows, translate(lines[i], &flipMap))
	}
	return newFromList(rows)
}

func stripSurroundingNewlines(s []rune) []rune {
	var rows [][]rune
	seen := false
	for _, row := range pySplitLines(s) {
		if len(trimSpaceRunes(row)) != 0 || seen {
			seen = true
			rows = append(rows, row)
		}
	}
	out := newFromList(rows)
	n := len(out)
	for n > 0 && pyIsSpace(out[n-1]) {
		n--
	}
	return out[:n]
}

func normalizeSurroundingNewlines(s []rune) []rune {
	out := []rune{'\n'}
	out = append(out, stripSurroundingNewlines(s)...)
	return append(out, '\n')
}
