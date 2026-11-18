#!/bin/bash
# Oracle: a Go port of pyfiglet's FIGlet rendering (fonts, kerning/smushing rules,
# wrapping, justification, direction, -r -F -s -n), standard library only.
set -euo pipefail
cd /workspace/repo

# fonts: move the .flf files into a folder of our own and embed them with go:embed
mkdir -p fonts
mv pyfiglet/fonts/*.flf fonts/

cat > go.mod <<'GOEOF'
module figlet

go 1.21
GOEOF

cat > main.go <<'GOEOF'
// Go port of pyfiglet (commit 255827ef) rendering: fonts, smushing, wrapping,
// justification, direction, -r, -F, -s, -n. Standard library only.
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

//go:embed fonts
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

func fail(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

// ---------- Python-compatible string helpers ----------

func pyIsSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func pySplitlines(s []rune) [][]rune {
	var out [][]rune
	start := 0
	i := 0
	for i < len(s) {
		c := s[i]
		switch c {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029, '\r':
			out = append(out, s[start:i])
			if c == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			i++
			start = i
		default:
			i++
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func pyStrip(s []rune) []rune {
	a, b := 0, len(s)
	for a < b && pyIsSpace(s[a]) {
		a++
	}
	for b > a && pyIsSpace(s[b-1]) {
		b--
	}
	return s[a:b]
}

func pyRstrip(s []rune) []rune {
	b := len(s)
	for b > 0 && pyIsSpace(s[b-1]) {
		b--
	}
	return s[:b]
}

// pyDecodeUTF8 mimics bytes.decode('UTF-8', 'replace'): each maximal ill-formed
// subpart becomes one U+FFFD.
func pyDecodeUTF8(b []byte) []rune {
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
		case c == 0xED:
			need, hi = 2, 0x9F
		case c >= 0xE1 && c <= 0xEF:
			need = 2
		case c == 0xF0:
			need, lo = 3, 0x90
		case c == 0xF4:
			need, hi = 3, 0x8F
		case c >= 0xF1 && c <= 0xF3:
			need = 3
		default:
			out = append(out, 0xFFFD)
			i++
			continue
		}
		j := i + 1
		ok := true
		for k := 0; k < need; k++ {
			if j >= len(b) {
				ok = false
				break
			}
			cl, ch := byte(0x80), byte(0xBF)
			if k == 0 {
				cl, ch = lo, hi
			}
			if b[j] < cl || b[j] > ch {
				ok = false
				break
			}
			j++
		}
		if !ok {
			out = append(out, 0xFFFD)
			i = j
			continue
		}
		var r rune
		switch need {
		case 1:
			r = rune(c&0x1F)<<6 | rune(b[i+1]&0x3F)
		case 2:
			r = rune(c&0x0F)<<12 | rune(b[i+1]&0x3F)<<6 | rune(b[i+2]&0x3F)
		default:
			r = rune(c&0x07)<<18 | rune(b[i+1]&0x3F)<<12 | rune(b[i+2]&0x3F)<<6 | rune(b[i+3]&0x3F)
		}
		out = append(out, r)
		i = j
	}
	return out
}

// ---------- Font ----------

type font struct {
	name           string
	height         int
	hardBlank      string
	printDirection int // -1 = absent
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

func loadFontData(name string) []byte {
	for _, ext := range []string{"tlf", "flf"} {
		data, err := fontFS.ReadFile("fonts/" + name + "." + ext)
		if err != nil {
			continue
		}
		if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil && len(zr.File) > 0 {
			f, err := zr.File[0].Open()
			if err == nil {
				d, _ := io.ReadAll(f)
				f.Close()
				return d
			}
		}
		return data
	}
	return nil
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		fail("problem parsing font: bad number %q", s)
	}
	return n
}

type lineSource struct {
	lines [][]rune
	pos   int
}

func (l *lineSource) pop() []rune {
	if l.pos >= len(l.lines) {
		fail("problem parsing font: unexpected end of file")
	}
	s := l.lines[l.pos]
	l.pos++
	return s
}
func (l *lineSource) more() bool { return l.pos < len(l.lines) }

// stripEnd mimics re.sub(re.escape(end)+r'{1,2}\s*$', ”, line)
func stripEnd(line []rune, end rune) []rune {
	for p := 0; p < len(line); p++ {
		for m := 2; m >= 1; m-- {
			if p+m > len(line) {
				continue
			}
			ok := true
			for k := p; k < p+m; k++ {
				if line[k] != end {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			for k := p + m; k < len(line); k++ {
				if !pyIsSpace(line[k]) {
					ok = false
					break
				}
			}
			if ok {
				return line[:p]
			}
		}
	}
	return line
}

func readChar(src *lineSource, height int) (int, [][]rune) {
	var end rune
	haveEnd := false
	width := 0
	chars := make([][]rune, 0, height)
	for i := 0; i < height; i++ {
		line := src.pop()
		if !haveEnd {
			last := -1
			for k, r := range line {
				if !pyIsSpace(r) {
					last = k
				}
			}
			if last == -1 {
				if len(line) == 0 {
					fail("problem parsing font: empty line")
				}
				last = 0
			}
			end = line[last]
			haveEnd = true
		}
		line = stripEnd(line, end)
		if len(line) > width {
			width = len(line)
		}
		chars = append(chars, line)
	}
	return width, chars
}

func nonEmpty(rows [][]rune) bool {
	for _, r := range rows {
		if len(r) > 0 {
			return true
		}
	}
	return false
}

func loadFont(name string) *font {
	raw := loadFontData(name)
	if raw == nil {
		return nil
	}
	data := pyDecodeUTF8(raw)
	for i, r := range data {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			data[i] = ' '
		}
	}
	src := &lineSource{lines: pySplitlines(data)}
	header := src.pop()
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || header[1] != 'l' || header[2] != 'f' || header[3] != '2' || header[4] == '\n' {
		fail("%s is not a valid figlet font", name)
	}
	fields := strings.FieldsFunc(string(header[5:]), pyIsSpace)
	if len(fields) < 6 {
		fail("malformed header for %s", name)
	}
	f := &font{name: name, hardBlank: fields[0], printDirection: -1,
		chars: map[int][][]rune{}, width: map[int]int{}}
	height := atoi(fields[1])
	_ = atoi(fields[2]) // baseline
	_ = atoi(fields[3]) // max length
	oldLayout := atoi(fields[4])
	commentLines := atoi(fields[5])
	haveFull := false
	fullLayout := 0
	if len(fields) > 6 {
		f.printDirection = atoi(fields[6])
	}
	if len(fields) > 7 {
		fullLayout = atoi(fields[7])
		haveFull = true
	}
	if !haveFull {
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
	for i := 0; i < commentLines; i++ {
		src.pop()
	}
	for i := 32; i < 127; i++ {
		w, letter := readChar(src, height)
		if i == 32 || nonEmpty(letter) {
			f.chars[i] = letter
			f.width[i] = w
		}
	}
	if src.more() {
		for _, c := range "ÄÖÜäöüß" {
			w, letter := readChar(src, height)
			if nonEmpty(letter) {
				f.chars[int(c)] = letter
				f.width[int(c)] = w
			}
		}
	}
	for src.more() {
		line := pyStrip(src.pop())
		tok := line
		for k, r := range line {
			if r == ' ' {
				tok = line[:k]
				break
			}
		}
		if len(tok) == 0 {
			continue
		}
		if len(tok) >= 2 && tok[0] == '0' && (tok[1] == 'x' || tok[1] == 'X') {
			n, err := strconv.ParseInt(string(tok[2:]), 16, 64)
			if err != nil {
				fail("problem parsing font: bad code %q", string(tok))
			}
			w, letter := readChar(src, height)
			if nonEmpty(letter) {
				f.chars[int(n)] = letter
				f.width[int(n)] = w
			}
		}
	}
	return f
}

// ---------- Rendering engine ----------

type marker struct {
	buf [][]rune
	it  int
}

type builder struct {
	text              []int
	direction         string
	width             int
	font              *font
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

func copyRows(b [][]rune) [][]rune {
	c := make([][]rune, len(b))
	copy(c, b)
	return c
}

func (b *builder) emptyBuffer() [][]rune { return make([][]rune, b.font.height) }

func (b *builder) charAt(i int) ([][]rune, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	c, ok := b.font.chars[b.text[i]]
	return c, ok
}

func (b *builder) curWidth() int { return b.font.width[b.text[b.iterator]] }

func (b *builder) addChar() {
	curChar, has := b.charAt(b.iterator)
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyRows(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if !has {
		return
	}
	if b.width < b.curWidth() {
		fail("Width is not enough to print this character")
	}
	b.curCharWidth = b.curWidth()
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush
	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyRows(b.buffer), b.iterator})
	}
	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addCurCharRow(curChar, row)
		}
	}
	b.prevCharWidth = b.curCharWidth
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		if i >= len(addRight) {
			fail("IndexError: string index out of range")
		}
		right := addRight[i]
		if idx < 0 || idx >= len(addLeft) {
			continue // result would be discarded
		}
		left := addLeft[idx]
		smushed, ok := b.smushChars(left, right)
		if !ok {
			fail("TypeError: cannot join None")
		}
		nl := make([]rune, len(addLeft))
		copy(nl, addLeft)
		nl[idx] = smushed
		addLeft = nl
	}
	return addLeft, addRight
}

func (b *builder) addCurCharRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	var tail []rune
	if b.maxSmush < len(addRight) {
		tail = addRight[b.maxSmush:]
	}
	nr := make([]rune, 0, len(addLeft)+len(tail))
	nr = append(nr, addLeft...)
	nr = append(nr, tail...)
	b.buffer[row] = nr
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = b.emptyBuffer()
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, has := b.charAt(b.iterator)
	if !has {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.queue = append(b.queue, m.buf)
		b.iterator = m.it
		b.cutBufferCommon()
	} else {
		b.queue = append(b.queue, b.buffer)
		b.iterator--
		b.cutBufferCommon()
	}
}

func (b *builder) justifyRows(buf [][]rune) [][]rune {
	out := make([][]rune, len(buf))
	for row := range buf {
		n := 0
		switch b.justify {
		case "right":
			n = b.width - len(buf[row]) - 1
		case "center":
			n = (b.width - len(buf[row])) / 2 // Go truncates toward zero, like int()
		}
		if n > 0 {
			nr := make([]rune, 0, n+len(buf[row]))
			for k := 0; k < n; k++ {
				nr = append(nr, ' ')
			}
			out[row] = append(nr, buf[row]...)
		} else {
			out[row] = buf[row]
		}
	}
	return out
}

func (b *builder) result() string {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		buf = b.justifyRows(buf)
		var s strings.Builder
		for _, row := range buf {
			s.WriteString(string(row))
			s.WriteByte('\n')
		}
		sb.WriteString(strings.ReplaceAll(s.String(), b.font.hardBlank, " "))
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
		t := len(lineLeft)
		for t > 0 && lineLeft[t-1] == ' ' {
			t--
		}
		linebd := t - 1
		if linebd < 0 {
			linebd = 0
		}
		var ch1 rune
		has1 := false
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
			has1 = true
		} else {
			linebd = 0
		}
		charbd := 0
		for charbd < len(lineRight) && lineRight[charbd] == ' ' {
			charbd++
		}
		var ch2 rune
		has2 := false
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
			has2 = true
		}
		amt := charbd + len(lineLeft) - 1 - linebd
		if !has1 || ch1 == ' ' {
			amt++
		} else if has2 {
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
	m := b.font.smushMode
	if m&smSmush == 0 {
		return 0, false
	}
	hb := b.font.hardBlank
	l, r := string(left), string(right)
	if m&63 == 0 {
		if l == hb {
			return right, true
		}
		if r == hb {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if m&smHardblank != 0 {
		if l == hb && r == hb {
			return left, true
		}
	}
	if l == hb || r == hb {
		return 0, false
	}
	if m&smEqual != 0 && left == right {
		return left, true
	}
	type pair struct{ a, b string }
	var smushes []pair
	if m&smLowline != 0 {
		smushes = append(smushes, pair{"_", `|/\[]{}()<>`})
	}
	if m&smHierarchy != 0 {
		smushes = append(smushes,
			pair{"|", `/\[]{}()<>`},
			pair{`\/`, "[]{}()<>"},
			pair{"[]", "{}()<>"},
			pair{"{}", "()<>"},
			pair{"()", "<>"},
		)
	}
	for _, p := range smushes {
		if strings.ContainsRune(p.a, left) && strings.ContainsRune(p.b, right) {
			return right, true
		}
		if strings.ContainsRune(p.a, right) && strings.ContainsRune(p.b, left) {
			return left, true
		}
	}
	if m&smPair != 0 {
		for _, pr := range []string{l + r, r + l} {
			if pr == "[]" || pr == "{}" || pr == "()" {
				return '|', true
			}
		}
	}
	if m&smBigX != 0 {
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

func render(text []rune, f *font, direction string, width int, justify string) string {
	b := &builder{direction: direction, width: width, font: f, justify: justify}
	b.text = make([]int, len(text))
	for i, r := range text {
		b.text[i] = int(r)
	}
	b.buffer = b.emptyBuffer()
	for b.iterator < len(b.text) {
		b.addChar()
		b.iterator++
	}
	return b.result()
}

// ---------- Post-processing: -r, -F, -s, -n ----------

var reverseMap = map[rune]rune{'(': ')', ')': '(', '/': '\\', '<': '>', '>': '<', '[': ']', '\\': '/', ']': '[', '{': '}', '}': '{'}
var flipMap = map[rune]rune{'/': '\\', 'A': 'V', 'M': 'W', 'P': 'b', 'R': 'b', 'V': 'A', 'W': 'M', '\\': '/', '^': 'v', '_': '-', 'b': 'P', 'm': 'w', 'v': '^', 'w': 'm'}

func joinRowsPy(rows [][]rune) []rune {
	// '\n'.join(rows) + '\n'
	var out []rune
	for i, r := range rows {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, r...)
	}
	out = append(out, '\n')
	return out
}

func reverseStr(s []rune) []rune {
	rows := pySplitlines(s)
	out := make([][]rune, len(rows))
	for i, row := range rows {
		n := len(row)
		nr := make([]rune, n)
		for k, c := range row {
			if c < 256 {
				if m, ok := reverseMap[c]; ok {
					c = m
				}
			}
			nr[n-1-k] = c
		}
		out[i] = nr
	}
	return joinRowsPy(out)
}

func flipStr(s []rune) []rune {
	rows := pySplitlines(s)
	out := make([][]rune, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		nr := make([]rune, len(row))
		for k, c := range row {
			if c < 256 {
				if m, ok := flipMap[c]; ok {
					c = m
				}
			}
			nr[k] = c
		}
		out = append(out, nr)
	}
	return joinRowsPy(out)
}

func stripSurrounding(s []rune) []rune {
	var out [][]rune
	seen := false
	for _, row := range pySplitlines(s) {
		if len(pyStrip(row)) != 0 || seen {
			seen = true
			out = append(out, row)
		}
	}
	return pyRstrip(joinRowsPy(out))
}

// ---------- Command line (optparse-like) ----------

type options struct {
	font      string
	direction string
	justify   string
	width     int
	reverse   bool
	normalize bool
	strip     bool
	flip      bool
	args      []string
}

func usageErr(msg string) {
	fmt.Fprintf(os.Stderr, "Usage: figlet [options] [text..]\n\nfiglet: error: %s\n", msg)
	os.Exit(2)
}

func choice(name, v string, ok ...string) string {
	for _, o := range ok {
		if v == o {
			return v
		}
	}
	usageErr(fmt.Sprintf("option %s: invalid choice: '%s'", name, v))
	return ""
}

func parseArgs(argv []string) options {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	longs := map[string]string{"--font": "f", "--direction": "D", "--justify": "j", "--width": "w",
		"--reverse": "r", "--flip": "F", "--strip-surrounding-newlines": "s", "--normalize-surrounding-newlines": "n"}
	takesVal := map[string]bool{"f": true, "D": true, "j": true, "w": true}
	apply := func(k, v string) {
		switch k {
		case "f":
			o.font = v
		case "D":
			o.direction = choice("-D", v, "auto", "left-to-right", "right-to-left")
		case "j":
			o.justify = choice("-j", v, "auto", "left", "center", "right")
		case "w":
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				usageErr(fmt.Sprintf("option -w: invalid integer value: '%s'", v))
			}
			o.width = n
		case "r":
			o.reverse = true
		case "F":
			o.flip = true
		case "s":
			o.strip = true
		case "n":
			o.normalize = true
		}
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			o.args = append(o.args, argv[i+1:]...)
			return o
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := a, "", false
			if k := strings.IndexByte(a, '='); k >= 0 {
				name, val, hasVal = a[:k], a[k+1:], true
			}
			key, ok := longs[name]
			if !ok {
				usageErr("no such option: " + name)
			}
			if takesVal[key] {
				if !hasVal {
					if i+1 >= len(argv) {
						usageErr(name + " option requires an argument")
					}
					i++
					val = argv[i]
				}
				apply(key, val)
			} else {
				apply(key, "")
			}
		case len(a) > 1 && a[0] == '-':
			cl := a[1:]
			for k := 0; k < len(cl); k++ {
				key := string(cl[k])
				if key != "f" && key != "D" && key != "j" && key != "w" && key != "r" && key != "F" && key != "s" && key != "n" {
					usageErr("no such option: -" + key)
				}
				if takesVal[key] {
					if k+1 < len(cl) {
						apply(key, cl[k+1:])
					} else {
						if i+1 >= len(argv) {
							usageErr("-" + key + " option requires an argument")
						}
						i++
						apply(key, argv[i])
					}
					break
				}
				apply(key, "")
			}
		default:
			o.args = append(o.args, a)
		}
	}
	return o
}

func main() {
	o := parseArgs(os.Args[1:])
	if len(o.args) == 0 {
		fmt.Println("Usage: figlet [options] [text..]")
		os.Exit(1)
	}
	text := []rune(strings.Join(o.args, " "))
	f := loadFont(o.font)
	if f == nil {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", o.font)
		os.Exit(1)
	}
	direction := o.direction
	if direction == "auto" {
		if f.printDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
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
	r := []rune(render(text, f, direction, o.width, justify))
	if o.reverse {
		r = reverseStr(r)
	}
	if o.flip {
		r = flipStr(r)
	}
	if o.strip {
		r = stripSurrounding(r)
	} else if o.normalize {
		r = append(append([]rune{'\n'}, stripSurrounding(r)...), '\n')
	}
	w := bufio.NewWriter(os.Stdout)
	w.WriteString(string(r))
	w.WriteByte('\n')
	w.Flush()
}
GOEOF

gofmt -l . >/dev/null 2>&1 || true
go build -o /tmp/figlet .

# the original Python must be gone
find /workspace/repo -name '*.py' -delete
