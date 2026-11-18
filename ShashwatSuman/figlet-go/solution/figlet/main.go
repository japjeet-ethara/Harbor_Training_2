// Command figlet is a byte-for-byte Go port of pyfiglet's command-line
// renderer (pyfiglet commit 255827efbc09d662875dad50fa0edc1e1263bf45).
//
// The code deliberately follows pyfiglet/__init__.py line by line, including
// its quirks (e.g. "/" + "\" smushes to "|" and "\" + "/" to "Y").
package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed pyfiglet/fonts
var fontFS embed.FS

const embeddedFontDir = "pyfiglet/fonts"

var fontDirectories = []string{
	"/usr/local/share/pyfiglet/",
	"/usr/local/share/figlet",
	"/usr/share/figlet",
}

// ---------------------------------------------------------------------------
// Python string helpers (operating on []rune == Python str code points)
// ---------------------------------------------------------------------------

// isPySpace mirrors Python's str.isspace() for a single code point.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0d, r >= 0x1c && r <= 0x20:
		return true
	case r == 0x85, r == 0xa0, r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', 0x0b, 0x0c, 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// splitlines mirrors Python's str.splitlines() (keepends=False).
func splitlines(s []rune) [][]rune {
	var out [][]rune
	start := 0
	i := 0
	for i < len(s) {
		r := s[i]
		if isLineBreak(r) {
			out = append(out, s[start:i])
			if r == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			i++
			start = i
			continue
		}
		i++
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func pyStrip(s []rune) []rune {
	a, b := 0, len(s)
	for a < b && isPySpace(s[a]) {
		a++
	}
	for b > a && isPySpace(s[b-1]) {
		b--
	}
	return s[a:b]
}

func pyRstrip(s []rune) []rune {
	b := len(s)
	for b > 0 && isPySpace(s[b-1]) {
		b--
	}
	return s[:b]
}

// pySplit mirrors Python's str.split() with no arguments.
func pySplit(s []rune) [][]rune {
	var out [][]rune
	i := 0
	for i < len(s) {
		for i < len(s) && isPySpace(s[i]) {
			i++
		}
		if i >= len(s) {
			break
		}
		j := i
		for j < len(s) && !isPySpace(s[j]) {
			j++
		}
		out = append(out, s[i:j])
		i = j
	}
	return out
}

func rstripSpaceLen(s []rune) int {
	b := len(s)
	for b > 0 && s[b-1] == ' ' {
		b--
	}
	return b
}

func lstripSpaceCount(s []rune) int {
	a := 0
	for a < len(s) && s[a] == ' ' {
		a++
	}
	return a
}

// decodeUTF8Replace mirrors bytes.decode('UTF-8', 'replace') in CPython,
// which replaces each maximal invalid subpart with one U+FFFD.
func decodeUTF8Replace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	i := 0
	n := len(b)
	for i < n {
		c := b[i]
		if c < 0x80 {
			out = append(out, rune(c))
			i++
			continue
		}
		var need int
		var lo, hi byte = 0x80, 0xbf
		switch {
		case c >= 0xc2 && c <= 0xdf:
			need = 1
		case c == 0xe0:
			need, lo = 2, 0xa0
		case c >= 0xe1 && c <= 0xec, c == 0xee, c == 0xef:
			need = 2
		case c == 0xed:
			need, hi = 2, 0x9f
		case c == 0xf0:
			need, lo = 3, 0x90
		case c >= 0xf1 && c <= 0xf3:
			need = 3
		case c == 0xf4:
			need, hi = 3, 0x8f
		default:
			out = append(out, 0xfffd)
			i++
			continue
		}
		var cp rune
		switch need {
		case 1:
			cp = rune(c & 0x1f)
		case 2:
			cp = rune(c & 0x0f)
		case 3:
			cp = rune(c & 0x07)
		}
		j := 1
		ok := true
		for ; j <= need; j++ {
			if i+j >= n {
				ok = false
				break
			}
			cc := b[i+j]
			l, h := byte(0x80), byte(0xbf)
			if j == 1 {
				l, h = lo, hi
			}
			if cc < l || cc > h {
				ok = false
				break
			}
			cp = cp<<6 | rune(cc&0x3f)
		}
		if !ok {
			out = append(out, 0xfffd)
			i += j
			continue
		}
		out = append(out, cp)
		i += need + 1
	}
	return out
}

// ---------------------------------------------------------------------------
// FigletFont
// ---------------------------------------------------------------------------

type FigletFont struct {
	name           string
	height         int
	hardBlank      []rune
	printDirection int
	hasPrintDir    bool
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

var errFontNotFound = errors.New("font not found")

func readEmbedded(fn string) ([]byte, bool) {
	if !fs.ValidPath(fn) {
		return nil, false
	}
	data, err := fontFS.ReadFile(embeddedFontDir + "/" + fn)
	if err != nil {
		return nil, false
	}
	return data, true
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func preloadFont(font string) ([]rune, error) {
	var data []byte
	found := false
	for _, ext := range []string{"tlf", "flf"} {
		fn := font + "." + ext
		if d, ok := readEmbedded(fn); ok {
			data, found = d, true
			break
		}
		for _, loc := range append([]string{"./"}, fontDirectories...) {
			full := filepath.Join(loc, fn)
			if isFile(full) {
				d, err := os.ReadFile(full)
				if err == nil {
					data, found = d, true
				}
				break
			}
		}
	}
	if found {
		if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
			if len(zr.File) == 0 {
				return nil, errors.New("empty zip font")
			}
			rc, err := zr.File[0].Open()
			if err != nil {
				return nil, err
			}
			d, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			data = d
		}
	}
	if len(data) == 0 {
		return nil, errFontNotFound
	}
	return decodeUTF8Replace(data), nil
}

func parsePyInt(s string) (int, error) {
	return strconv.Atoi(strings.ReplaceAll(s, "_", ""))
}

// endMarkerChar mirrors re.search(r'(.)\s*$', line).group(1).
func endMarkerChar(line []rune) (rune, bool) {
	if len(line) == 0 {
		return 0, false
	}
	for k := len(line) - 1; k >= 0; k-- {
		if !isPySpace(line[k]) {
			return line[k], true
		}
	}
	return line[0], true
}

// stripEndMarker mirrors re.sub(re.escape(end) + r"{1,2}\s*$", "", line).
func stripEndMarker(line []rune, end rune) []rune {
	n := len(line)
	// allWS[i] == line[i:] consists only of whitespace
	allWS := make([]bool, n+1)
	allWS[n] = true
	for i := n - 1; i >= 0; i-- {
		allWS[i] = allWS[i+1] && isPySpace(line[i])
	}
	for p := 0; p < n; p++ {
		if line[p] != end {
			continue
		}
		if (p+1 < n && line[p+1] == end && allWS[p+2]) || allWS[p+1] {
			return line[:p]
		}
	}
	return line
}

func loadFont(name string) (*FigletFont, error) {
	raw, err := preloadFont(name)
	if err != nil {
		return nil, err
	}
	f := &FigletFont{name: name, chars: map[int][][]rune{}, width: map[int]int{}}
	ferr := func(msg string) error {
		return fmt.Errorf("problem parsing %s font: %s", name, msg)
	}

	text := make([]rune, len(raw))
	for i, r := range raw {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			r = ' '
		}
		text[i] = r
	}
	data := splitlines(text)
	pos := 0
	pop := func() ([]rune, bool) {
		if pos >= len(data) {
			return nil, false
		}
		l := data[pos]
		pos++
		return l, true
	}

	header, ok := pop()
	if !ok {
		return nil, ferr("empty")
	}
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') ||
		header[1] != 'l' || header[2] != 'f' || header[3] != '2' || header[4] == '\n' {
		return nil, ferr("not a valid figlet font")
	}
	fields := pySplit(header[5:])
	if len(fields) < 6 {
		return nil, ferr("malformed header")
	}
	f.hardBlank = fields[0]
	nums := make([]int, 0, 7)
	for i := 1; i < len(fields) && i <= 7; i++ {
		if i >= 6 && i > len(fields)-1 {
			break
		}
		v, err := parsePyInt(string(fields[i]))
		if err != nil {
			return nil, ferr("bad header number")
		}
		nums = append(nums, v)
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	fullLayout := 0
	hasFull := false
	if len(fields) > 6 {
		f.printDirection = nums[5]
		f.hasPrintDir = true
	}
	if len(fields) > 7 {
		fullLayout = nums[6]
		hasFull = true
	}
	if !hasFull {
		switch {
		case oldLayout == 0:
			fullLayout = 64
		case oldLayout < 0:
			fullLayout = 0
		default:
			fullLayout = (oldLayout & 31) | 128
		}
	}
	f.height = height
	f.smushMode = fullLayout

	for i := 0; i < commentLines; i++ {
		if _, ok := pop(); !ok {
			return nil, ferr("pop from empty list")
		}
	}

	loadChar := func() (int, [][]rune, error) {
		var end rune
		haveEnd := false
		width := 0
		var rows [][]rune
		for i := 0; i < height; i++ {
			line, ok := pop()
			if !ok {
				return 0, nil, ferr("pop from empty list")
			}
			if !haveEnd {
				e, ok := endMarkerChar(line)
				if !ok {
					return 0, nil, ferr("no end marker")
				}
				end, haveEnd = e, true
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

	for i := 32; i < 127; i++ {
		w, letter, err := loadChar()
		if err != nil {
			return nil, err
		}
		if i == 32 || nonEmpty(letter) {
			f.chars[i] = letter
			f.width[i] = w
		}
	}

	if pos < len(data) {
		for _, c := range "ÄÖÜäöüß" {
			w, letter, err := loadChar()
			if err != nil {
				return nil, err
			}
			if nonEmpty(letter) {
				f.chars[int(c)] = letter
				f.width[int(c)] = w
			}
		}
	}

	for pos < len(data) {
		line, _ := pop()
		line = pyStrip(line)
		code := line
		for k, r := range line {
			if r == ' ' {
				code = line[:k]
				break
			}
		}
		if len(code) == 0 {
			continue
		}
		if len(code) >= 2 && code[0] == '0' && (code[1] == 'x' || code[1] == 'X') {
			hex := strings.ReplaceAll(string(code[2:]), "_", "")
			v, err := strconv.ParseInt(hex, 16, 64)
			if err != nil || hex == "" || strings.HasPrefix(hex, "+") || strings.HasPrefix(hex, "-") {
				return nil, ferr("invalid literal for int()")
			}
			w, letter, err := loadChar()
			if err != nil {
				return nil, err
			}
			if nonEmpty(letter) {
				f.chars[int(v)] = letter
				f.width[int(v)] = w
			}
		}
	}
	return f, nil
}

// ---------------------------------------------------------------------------
// FigletBuilder
// ---------------------------------------------------------------------------

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

// noChar stands for Python's empty string ("") used as a "character".
const noChar rune = -1

type marker struct {
	buf [][]rune
	it  int
}

type builder struct {
	text    []rune
	dir     string
	width   int
	font    *FigletFont
	justify string

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []marker
	product      [][][]rune
	buffer       [][]rune
}

var errCharNotPrinted = errors.New("Width is not enough to print this character")
var errSmush = errors.New("smush failure")

func newBuilder(text []rune, font *FigletFont, dir string, width int, justify string) *builder {
	b := &builder{text: text, dir: dir, width: width, font: font, justify: justify}
	b.buffer = make([][]rune, font.height)
	return b
}

func copyBuf(buf [][]rune) [][]rune {
	c := make([][]rune, len(buf))
	copy(c, buf)
	return c
}

func (b *builder) curChar() ([][]rune, bool) {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return nil, false
	}
	c, ok := b.font.chars[int(b.text[b.iterator])]
	return c, ok
}

func (b *builder) addCharToProduct() error {
	cur, ok := b.curChar()
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}
	if !ok {
		return nil
	}
	w := b.font.width[int(b.text[b.iterator])]
	if b.width < w {
		return errCharNotPrinted
	}
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, cur)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			if err := b.addCurCharRowToBufferRow(cur, row); err != nil {
				return err
			}
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) smushRow(cur [][]rune, row int) ([]rune, []rune, error) {
	addLeft := b.buffer[row]
	addRight := cur[row]
	if b.dir == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := noChar
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		if i >= len(addRight) {
			return nil, nil, errSmush // IndexError in pyfiglet
		}
		right := addRight[i]
		smushed, ok := b.smushChars(left, right)
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if idx == len(addLeft) || !ok || smushed == noChar {
			return nil, nil, errSmush
		}
		nl := make([]rune, len(addLeft))
		copy(nl, addLeft)
		nl[idx] = smushed
		addLeft = nl
	}
	return addLeft, addRight, nil
}

func (b *builder) addCurCharRowToBufferRow(cur [][]rune, row int) error {
	addLeft, addRight, err := b.smushRow(cur, row)
	if err != nil {
		return err
	}
	var tail []rune
	if b.maxSmush < len(addRight) {
		tail = addRight[b.maxSmush:]
	}
	nr := make([]rune, 0, len(addLeft)+len(tail))
	nr = append(nr, addLeft...)
	nr = append(nr, tail...)
	b.buffer[row] = nr
	return nil
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([][]rune, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	// pyfiglet recomputes maxSmush here, but the value is always recomputed
	// before it is used again, so it has no visible effect.
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.product = append(b.product, m.buf)
		b.iterator = m.it
		b.cutBufferCommon()
	} else {
		b.product = append(b.product, b.buffer)
		b.iterator--
		b.cutBufferCommon()
	}
}

func (b *builder) justifyBuffer(buf [][]rune) {
	for row := 0; row < b.font.height; row++ {
		pad := 0
		switch b.justify {
		case "right":
			pad = b.width - len(buf[row]) - 1
		case "center":
			pad = (b.width - len(buf[row])) / 2
		default:
			continue
		}
		if pad > 0 {
			nr := make([]rune, 0, pad+len(buf[row]))
			for i := 0; i < pad; i++ {
				nr = append(nr, ' ')
			}
			buf[row] = append(nr, buf[row]...)
		}
	}
}

func (b *builder) returnProduct() string {
	if len(b.buffer[0]) != 0 {
		b.product = append(b.product, b.buffer)
	}
	var sb strings.Builder
	hb := string(b.font.hardBlank)
	for _, buf := range b.product {
		b.justifyBuffer(buf)
		parts := make([]string, len(buf))
		for i, r := range buf {
			parts[i] = string(r)
		}
		s := strings.Join(parts, "\n") + "\n"
		s = strings.ReplaceAll(s, hb, " ")
		sb.WriteString(s)
	}
	return sb.String()
}

func (b *builder) isHardBlank(c rune) bool {
	return len(b.font.hardBlank) == 1 && c != noChar && c == b.font.hardBlank[0]
}

func (b *builder) smushAmount(buffer [][]rune, cur [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		lineRight := cur[row]
		if b.dir == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}
		linebd := rstripSpaceLen(lineLeft) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := noChar
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}
		charbd := lstripSpaceCount(lineRight)
		ch2 := noChar
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
		}
		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == noChar || ch1 == ' ' {
			amt++
		} else if ch2 != noChar {
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

// in mirrors Python's `c in s` where c is a one-char (or empty) string.
func in(c rune, s string) bool {
	return c == noChar || strings.ContainsRune(s, c)
}

// smushChars returns (result, true) or (_, false) for Python's None.
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
	mode := b.font.smushMode
	if mode&smSmush == 0 {
		return 0, false
	}
	if mode&63 == 0 {
		if b.isHardBlank(left) {
			return right, true
		}
		if b.isHardBlank(right) {
			return left, true
		}
		if b.dir == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if mode&smHardblank != 0 {
		if b.isHardBlank(left) && b.isHardBlank(right) {
			return left, true
		}
	}
	if b.isHardBlank(left) || b.isHardBlank(right) {
		return 0, false
	}
	if mode&smEqual != 0 {
		if left == right {
			return left, true
		}
	}
	type rule struct{ a, b string }
	var smushes []rule
	if mode&smLowline != 0 {
		smushes = append(smushes, rule{"_", `|/\[]{}()<>`})
	}
	if mode&smHierarchy != 0 {
		smushes = append(smushes,
			rule{"|", `/\[]{}()<>`},
			rule{`\/`, "[]{}()<>"},
			rule{"[]", "{}()<>"},
			rule{"{}", "()<>"},
			rule{"()", "<>"},
		)
	}
	for _, r := range smushes {
		if in(left, r.a) && in(right, r.b) {
			return right, true
		}
		if in(right, r.a) && in(left, r.b) {
			return left, true
		}
	}
	if mode&smPair != 0 && left != noChar && right != noChar {
		for _, pair := range [][2]rune{{left, right}, {right, left}} {
			p := string(pair[:])
			if p == "[]" || p == "{}" || p == "()" {
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

// ---------------------------------------------------------------------------
// FigletString transforms
// ---------------------------------------------------------------------------

var reverseTable = buildTable(
	" !\"#$%&')(*+,-.\\" +
		"0123456789:;>=<?" +
		"@ABCDEFGHIJKLMNO" +
		"PQRSTUVWXYZ]/[^_" +
		"`abcdefghijklmno" +
		"pqrstuvwxyz}|{~\x7f")

var flipTable = buildTable(
	" !\"#$%&'()*+,-.\\" +
		"0123456789:;<=>?" +
		"@VBCDEFGHIJKLWNO" +
		"bQbSTUAMXYZ[/]v-" +
		"`aPcdefghijklwno" +
		"pqrstu^mxyz{|}~\x7f")

func buildTable(printable string) [256]rune {
	var t [256]rune
	for i := range t {
		t[i] = rune(i)
	}
	rs := []rune(printable)
	if len(rs) != 96 {
		panic("bad translation table")
	}
	for i, r := range rs {
		t[32+i] = r
	}
	return t
}

func translate(row []rune, t *[256]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if r >= 0 && r < 256 {
			out[i] = t[r]
		} else {
			out[i] = r
		}
	}
	return out
}

func newFromList(rows [][]rune) string {
	parts := make([]string, len(rows))
	for i, r := range rows {
		parts[i] = string(r)
	}
	return strings.Join(parts, "\n") + "\n"
}

func reverseString(s string) string {
	var out [][]rune
	for _, row := range splitlines([]rune(s)) {
		t := translate(row, &reverseTable)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		out = append(out, t)
	}
	return newFromList(out)
}

func flipString(s string) string {
	rows := splitlines([]rune(s))
	var out [][]rune
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translate(rows[i], &flipTable))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s string) string {
	var out [][]rune
	seen := false
	for _, row := range splitlines([]rune(s)) {
		if len(pyStrip(row)) != 0 || seen {
			seen = true
			out = append(out, row)
		}
	}
	return string(pyRstrip([]rune(newFromList(out))))
}

// ---------------------------------------------------------------------------
// Command line (optparse emulation)
// ---------------------------------------------------------------------------

type options struct {
	font, direction, justify, color, load string
	width                                 int
	reverse, flip, strip, normalize       bool
	listFonts, infoFont, help             bool
}

type optSpec struct {
	short, long string
	takesValue  bool
}

var optSpecs = []optSpec{
	{"h", "help", false},
	{"", "version", false},
	{"f", "font", true},
	{"D", "direction", true},
	{"j", "justify", true},
	{"w", "width", true},
	{"r", "reverse", false},
	{"n", "normalize-surrounding-newlines", false},
	{"s", "strip-surrounding-newlines", false},
	{"F", "flip", false},
	{"l", "list_fonts", false},
	{"i", "info_font", false},
	{"L", "load", true},
	{"c", "color", true},
}

func usageError(msg string) {
	fmt.Fprintf(os.Stderr, "Usage: figlet [options] [text..]\n\nfiglet: error: %s\n", msg)
	os.Exit(2)
}

// parseNum mirrors optparse's _parse_num for type int.
func parseNum(val string) (int, error) {
	v := val
	radix := 10
	lower := strings.ToLower(v)
	switch {
	case strings.HasPrefix(lower, "0x"):
		radix = 16
		v = v[2:]
	case strings.HasPrefix(lower, "0b"):
		radix = 2
		v = v[2:]
		if v == "" {
			v = "0"
		}
	case strings.HasPrefix(v, "0"):
		radix = 8
	}
	v = strings.TrimSpace(v)
	n, err := strconv.ParseInt(strings.ReplaceAll(v, "_", ""), radix, 64)
	return int(n), err
}

func applyOpt(o *options, long, value string) {
	switch long {
	case "help":
		o.help = true
	case "version":
		fmt.Println("1.0.4")
		os.Exit(0)
	case "font":
		o.font = value
	case "direction":
		if value != "auto" && value != "left-to-right" && value != "right-to-left" {
			usageError("option -D: invalid choice: '" + value + "'")
		}
		o.direction = value
	case "justify":
		if value != "auto" && value != "left" && value != "center" && value != "right" {
			usageError("option -j: invalid choice: '" + value + "'")
		}
		o.justify = value
	case "width":
		n, err := parseNum(value)
		if err != nil {
			usageError("option -w: invalid integer value: '" + value + "'")
		}
		o.width = n
	case "reverse":
		o.reverse = true
	case "normalize-surrounding-newlines":
		o.normalize = true
	case "strip-surrounding-newlines":
		o.strip = true
	case "flip":
		o.flip = true
	case "list_fonts":
		o.listFonts = true
	case "info_font":
		o.infoFont = true
	case "load":
		o.load = value
	case "color":
		o.color = value
	}
}

func parseArgs(argv []string) (*options, []string) {
	o := &options{font: "standard", direction: "auto", justify: "auto", width: 80, color: ":"}
	var rest []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			rest = append(rest, argv[i+1:]...)
			return o, rest
		case strings.HasPrefix(a, "--"):
			name, value, hasValue := strings.Cut(a[2:], "=")
			var spec *optSpec
			for k := range optSpecs {
				if optSpecs[k].long == name {
					spec = &optSpecs[k]
					break
				}
			}
			if spec == nil {
				var matches []*optSpec
				for k := range optSpecs {
					if strings.HasPrefix(optSpecs[k].long, name) {
						matches = append(matches, &optSpecs[k])
					}
				}
				if len(matches) != 1 {
					usageError("no such option: --" + name)
				}
				spec = matches[0]
			}
			if spec.takesValue {
				if !hasValue {
					if i+1 >= len(argv) {
						usageError("--" + spec.long + " option requires an argument")
					}
					i++
					value = argv[i]
				}
			} else if hasValue {
				usageError("--" + spec.long + " option does not take a value")
			}
			applyOpt(o, spec.long, value)
		case strings.HasPrefix(a, "-") && len(a) > 1:
			chars := []rune(a[1:])
			for j := 0; j < len(chars); j++ {
				var spec *optSpec
				for k := range optSpecs {
					if optSpecs[k].short == string(chars[j]) {
						spec = &optSpecs[k]
						break
					}
				}
				if spec == nil {
					usageError("no such option: -" + string(chars[j]))
				}
				if spec.takesValue {
					value := string(chars[j+1:])
					if value == "" {
						if i+1 >= len(argv) {
							usageError("-" + spec.short + " option requires an argument")
						}
						i++
						value = argv[i]
					}
					applyOpt(o, spec.long, value)
					break
				}
				applyOpt(o, spec.long, "")
			}
		default:
			rest = append(rest, a)
		}
	}
	return o, rest
}

func run() int {
	o, args := parseArgs(os.Args[1:])
	if o.help {
		fmt.Println("Usage: figlet [options] [text..]")
		return 0
	}
	if o.listFonts || o.infoFont || o.load != "" || o.color == "list" {
		return 0 // out of scope
	}
	if len(args) == 0 {
		fmt.Println("Usage: figlet [options] [text..]")
		return 1
	}
	text := []rune(strings.Join(args, " "))

	font, err := loadFont(o.font)
	if err != nil {
		if errors.Is(err, errFontNotFound) {
			fmt.Printf("pyfiglet error: requested font '%s' not found.\n", o.font)
			return 1
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	direction := o.direction
	if direction == "auto" {
		direction = "left-to-right"
		if font.hasPrintDir && font.printDirection == 1 {
			direction = "right-to-left"
		}
	}
	justify := o.justify
	if justify == "auto" {
		if direction == "left-to-right" {
			justify = "left"
		} else {
			justify = "right"
		}
	}

	b := newBuilder(text, font, direction, o.width, justify)
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		b.iterator++
	}
	r := b.returnProduct()

	if o.reverse {
		r = reverseString(r)
	}
	if o.flip {
		r = flipString(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = "\n" + stripSurroundingNewlines(r) + "\n"
	}
	os.Stdout.WriteString(r + "\n")
	return 0
}

func main() {
	os.Exit(run())
}
