#!/bin/bash
# Oracle solution: a Go port of pyfiglet's FIGlet rendering, standard library only.
# Writes go.mod + main.go into /workspace/repo, moves the .flf fonts into a folder
# inside the repository (embedded with go:embed, so the program works from any
# working directory), then deletes every .py file.
set -euo pipefail
REPO=/workspace/repo
cd "$REPO"

# 1. fonts: only the standard .flf fonts, in a folder that is part of the Go module
mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/

# 2. the Go module
cat > go.mod <<'GOMOD'
module figlet

go 1.21
GOMOD

# 3. the program
cat > main.go <<'GOEOF'
// A Go port of pyfiglet's FIGlet rendering (standard library only).
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

//go:embed fonts/*.flf
var fontFS embed.FS

var reverseMap = map[rune]rune{0x28: 0x29, 0x29: 0x28, 0x2f: 0x5c, 0x3c: 0x3e, 0x3e: 0x3c, 0x5b: 0x5d, 0x5c: 0x2f, 0x5d: 0x5b, 0x7b: 0x7d, 0x7d: 0x7b}
var flipMap = map[rune]rune{0x2f: 0x5c, 0x41: 0x56, 0x4d: 0x57, 0x50: 0x62, 0x52: 0x62, 0x56: 0x41, 0x57: 0x4d, 0x5c: 0x2f, 0x5e: 0x76, 0x5f: 0x2d, 0x62: 0x50, 0x6d: 0x77, 0x76: 0x5e, 0x77: 0x6d}

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

func fatal(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}

// pySpace mirrors Python's str.isspace().
func pySpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// pySplitlines mirrors Python's str.splitlines().
func pySplitlines(s string) []string {
	var out []string
	rs := []rune(s)
	start := 0
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, string(rs[start:i]))
			start = i + 1
		case '\r':
			out = append(out, string(rs[start:i]))
			if i+1 < len(rs) && rs[i+1] == '\n' {
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

func pyStrip(s string) string { return strings.TrimFunc(s, pySpace) }

// ---------------------------------------------------------------- font

type font struct {
	height         int
	hardBlank      string
	printDirection int
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

var reMagic = regexp.MustCompile(`^[tf]lf2.`)

func readFontData(name string) (string, bool) {
	var data []byte
	found := false
	dirs := []string{"./", "/usr/local/share/pyfiglet/", "/usr/local/share/figlet", "/usr/share/figlet"}
	for _, ext := range []string{"tlf", "flf"} {
		fn := name + "." + ext
		if b, err := fontFS.ReadFile("fonts/" + fn); err == nil && !strings.ContainsAny(name, "/\\") {
			data, found = b, true
			break
		}
		for _, loc := range dirs {
			p := filepath.Join(loc, fn)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				if b, err := os.ReadFile(p); err == nil {
					data, found = b, true
					break
				}
			}
		}
	}
	if !found {
		return "", false
	}
	if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil && len(zr.File) > 0 {
		rc, err := zr.File[0].Open()
		if err == nil {
			b, _ := io.ReadAll(rc)
			rc.Close()
			data = b
		}
	}
	return strings.ToValidUTF8(string(data), "\uFFFD"), true
}

func parseFont(name, raw string) *font {
	f := &font{chars: map[int][][]rune{}, width: map[int]int{}, printDirection: -1}
	fail := func(msg string) { fatal("problem parsing %s font: %s", name, msg) }

	raw = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(raw)
	lines := pySplitlines(raw)
	pos := 0
	pop := func() string {
		if pos >= len(lines) {
			fail("pop from empty list")
		}
		l := lines[pos]
		pos++
		return l
	}

	header := pop()
	if !reMagic.MatchString(header) {
		fatal("%s is not a valid figlet font", name)
	}
	header = reMagic.ReplaceAllString(header, "")
	h := strings.Fields(header)
	if len(h) < 6 {
		fatal("malformed header for %s", name)
	}
	f.hardBlank = h[0]
	var n [5]int
	for i := 0; i < 5; i++ {
		v, err := strconv.Atoi(h[1+i])
		if err != nil {
			fail("bad header number")
		}
		n[i] = v
	}
	f.height = n[0]
	oldLayout, commentLines := n[3], n[4]
	fullLayout := 0
	hasFull := false
	if len(h) > 6 {
		v, err := strconv.Atoi(h[6])
		if err != nil {
			fail("bad print direction")
		}
		f.printDirection = v
	}
	if len(h) > 7 {
		v, err := strconv.Atoi(h[7])
		if err != nil {
			fail("bad full layout")
		}
		fullLayout, hasFull = v, true
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
	f.smushMode = fullLayout

	for i := 0; i < commentLines; i++ {
		pop()
	}

	readChar := func() (int, [][]rune) {
		var end rune
		haveEnd := false
		width := 0
		var rows [][]rune
		for k := 0; k < f.height; k++ {
			line := []rune(pop())
			if !haveEnd {
				// regex (.)\s*$ : last non-space char, else first char
				tail := len(line)
				for tail > 0 && pySpace(line[tail-1]) {
					tail--
				}
				if tail > 0 {
					end = line[tail-1]
				} else if len(line) > 0 {
					end = line[0]
				} else {
					fail("empty line")
				}
				haveEnd = true
			}
			// regex: re.escape(end) + r'{1,2}\s*$'  replaced by ''
			tail := len(line)
			for tail > 0 && pySpace(line[tail-1]) {
				tail--
			}
			for p := 0; p < len(line); p++ {
				if line[p] != end {
					continue
				}
				if p+1 >= tail || (p+1 < len(line) && line[p+1] == end && p+2 >= tail) {
					line = line[:p]
					break
				}
			}
			if len(line) > width {
				width = len(line)
			}
			rows = append(rows, line)
		}
		return width, rows
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
		w, letter := readChar()
		if i == 32 || nonEmpty(letter) {
			f.chars[i] = letter
			f.width[i] = w
		}
	}
	if pos < len(lines) {
		for _, r := range "ÄÖÜäöüß" {
			w, letter := readChar()
			if nonEmpty(letter) {
				f.chars[int(r)] = letter
				f.width[int(r)] = w
			}
		}
	}
	for pos < len(lines) {
		line := pyStrip(pop())
		tok := line
		if k := strings.Index(line, " "); k >= 0 {
			tok = line[:k]
		}
		if tok == "" {
			continue
		}
		if len(tok) >= 2 && tok[0] == '0' && (tok[1] == 'x' || tok[1] == 'X') {
			code, err := strconv.ParseInt(tok[2:], 16, 64)
			if err != nil {
				fail("bad code tag " + tok)
			}
			w, letter := readChar()
			if nonEmpty(letter) {
				f.chars[int(code)] = letter
				f.width[int(code)] = w
			}
		}
	}
	return f
}

// ------------------------------------------------------------- builder

type marker struct {
	buf [][]rune
	it  int
}

type builder struct {
	text      []rune
	direction string
	width     int
	f         *font
	justify   string

	iterator, maxSmush                        int
	curCharWidth, prevCharWidth, currentTotal int
	blankMarkers                              []marker
	product                                   [][][]rune
	buffer                                    [][]rune
}

func (b *builder) newBuffer() [][]rune { return make([][]rune, b.f.height) }

func copyBuf(buf [][]rune) [][]rune {
	c := make([][]rune, len(buf))
	copy(c, buf)
	return c
}

func (b *builder) charAt(i int) ([][]rune, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	c, ok := b.f.chars[int(b.text[i])]
	return c, ok
}

func (b *builder) widthAt(i int) int { return b.f.width[int(b.text[i])] }

func (b *builder) addCharToProduct() {
	curChar, have := b.charAt(b.iterator)
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if !have {
		return
	}
	if b.width < b.widthAt(b.iterator) {
		fatal("Width is not enough to print this character")
	}
	b.curCharWidth = b.widthAt(b.iterator)
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotal = len(b.buffer[0]) + b.curCharWidth - b.maxSmush
	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iterator})
	}
	if b.currentTotal >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.f.height; row++ {
			l, r := b.smushRow(curChar, row)
			b.buffer[row] = append(append([]rune{}, l...), pySliceFrom(r, b.maxSmush)...)
		}
	}
	b.prevCharWidth = b.curCharWidth
}

func pySliceFrom(r []rune, n int) []rune {
	if n < 0 {
		n += len(r)
		if n < 0 {
			n = 0
		}
	}
	if n > len(r) {
		return nil
	}
	return r[n:]
}

func (b *builder) handleNewLine() {
	if len(b.blankMarkers) > 0 {
		m := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.product = append(b.product, m.buf)
		b.iterator = m.it
		b.cutCommon()
	} else {
		b.product = append(b.product, b.buffer)
		b.iterator--
		b.cutCommon()
	}
}

func (b *builder) cutCommon() {
	b.currentTotal = 0
	b.buffer = b.newBuffer()
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, have := b.charAt(b.iterator)
	if !have {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

func (b *builder) rtl() bool { return b.direction == "right-to-left" }

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.rtl() {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := ""
		if idx >= 0 && idx < len(addLeft) {
			left = string(addLeft[idx])
		}
		if i >= len(addRight) {
			fatal("string index out of range")
		}
		right := string(addRight[i])
		sm, ok := b.smushChars(left, right)
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if idx == len(addLeft) {
			fatal("list assignment index out of range")
		}
		if !ok {
			fatal("sequence item: expected str instance, NoneType found")
		}
		nl := make([]rune, 0, len(addLeft))
		nl = append(nl, addLeft[:idx]...)
		nl = append(nl, []rune(sm)...)
		nl = append(nl, addLeft[idx+1:]...)
		addLeft = nl
	}
	return addLeft, addRight
}

func rstripSpace(r []rune) int {
	n := len(r)
	for n > 0 && r[n-1] == ' ' {
		n--
	}
	return n
}

func (b *builder) smushAmount(buffer, curChar [][]rune) int {
	if b.f.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.f.height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.rtl() {
			lineLeft, lineRight = lineRight, lineLeft
		}
		linebd := rstripSpace(lineLeft) - 1
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
	mode := b.f.smushMode
	if mode&smSmush == 0 {
		return "", false
	}
	hb := b.f.hardBlank
	if mode&63 == 0 {
		if left == hb {
			return right, true
		}
		if right == hb {
			return left, true
		}
		if b.rtl() {
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
			pair{"()", "<>"})
	}
	for _, p := range smushes {
		if strings.Contains(p.a, left) && strings.Contains(p.b, right) {
			return right, true
		}
		if strings.Contains(p.a, right) && strings.Contains(p.b, left) {
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

func (b *builder) justifyBuffer(buf [][]rune) {
	switch b.justify {
	case "right":
		for row := 0; row < b.f.height; row++ {
			n := b.width - len(buf[row]) - 1
			if n < 0 {
				n = 0
			}
			buf[row] = append([]rune(strings.Repeat(" ", n)), buf[row]...)
		}
	case "center":
		for row := 0; row < b.f.height; row++ {
			n := (b.width - len(buf[row])) / 2
			if n < 0 {
				n = 0
			}
			buf[row] = append([]rune(strings.Repeat(" ", n)), buf[row]...)
		}
	}
}

func (b *builder) render() string {
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	if len(b.buffer[0]) != 0 {
		b.product = append(b.product, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.product {
		b.justifyBuffer(buf)
		var s strings.Builder
		for _, row := range buf {
			s.WriteString(string(row))
			s.WriteByte('\n')
		}
		sb.WriteString(strings.ReplaceAll(s.String(), b.f.hardBlank, " "))
	}
	return sb.String()
}

// ------------------------------------------------- post-processing

func newFromList(rows []string) string { return strings.Join(rows, "\n") + "\n" }

func translateRunes(s string, m map[rune]rune) string {
	rs := []rune(s)
	for i, r := range rs {
		if t, ok := m[r]; ok {
			rs[i] = t
		}
	}
	return string(rs)
}

func reverseText(s string) string {
	var out []string
	for _, row := range pySplitlines(s) {
		rs := []rune(translateRunes(row, reverseMap))
		for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
			rs[i], rs[j] = rs[j], rs[i]
		}
		out = append(out, string(rs))
	}
	return newFromList(out)
}

func flipText(s string) string {
	rows := pySplitlines(s)
	var out []string
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translateRunes(rows[i], flipMap))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s string) string {
	var out []string
	seen := false
	for _, row := range pySplitlines(s) {
		if pyStrip(row) != "" || seen {
			seen = true
			out = append(out, row)
		}
	}
	return strings.TrimRightFunc(newFromList(out), pySpace)
}

// -------------------------------------------------------------- main

func usageErr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "usage: figlet [options] [text..]\nfiglet: error: "+format+"\n", a...)
	os.Exit(2)
}

func main() {
	fontName, direction, justify := "standard", "auto", "auto"
	width := 80
	var doReverse, doNormalize, doStrip, doFlip bool
	var args []string

	longs := []struct {
		name   string
		short  byte
		hasArg bool
	}{
		{"font", 'f', true}, {"direction", 'D', true}, {"justify", 'j', true}, {"width", 'w', true},
		{"reverse", 'r', false}, {"normalize-surrounding-newlines", 'n', false},
		{"strip-surrounding-newlines", 's', false}, {"flip", 'F', false},
		{"list_fonts", 'l', false}, {"info_font", 'i', false}, {"load", 'L', true}, {"color", 'c', true},
	}
	apply := func(short byte, val string) {
		switch short {
		case 'f':
			fontName = val
		case 'D':
			if val != "auto" && val != "left-to-right" && val != "right-to-left" {
				usageErr("invalid direction: %q", val)
			}
			direction = val
		case 'j':
			if val != "auto" && val != "left" && val != "center" && val != "right" {
				usageErr("invalid justify: %q", val)
			}
			justify = val
		case 'w':
			v, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil {
				usageErr("option -w: invalid integer value: %q", val)
			}
			width = v
		case 'r':
			doReverse = true
		case 'n':
			doNormalize = true
		case 's':
			doStrip = true
		case 'F':
			doFlip = true
		}
	}

	argv := os.Args[1:]
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			args = append(args, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := a[2:], "", false
			if k := strings.Index(name, "="); k >= 0 {
				name, val, hasVal = name[:k], name[k+1:], true
			}
			matches := 0
			idx := -1
			for j, l := range longs {
				if l.name == name {
					matches, idx = 1, j
					break
				}
				if strings.HasPrefix(l.name, name) {
					matches++
					idx = j
				}
			}
			if matches != 1 {
				usageErr("no such option: --%s", name)
			}
			l := longs[idx]
			if l.hasArg && !hasVal {
				if i+1 >= len(argv) {
					usageErr("--%s option requires an argument", l.name)
				}
				i++
				val = argv[i]
			}
			apply(l.short, val)
		case len(a) > 1 && a[0] == '-':
			for k := 1; k < len(a); k++ {
				c := a[k]
				var l *struct {
					name   string
					short  byte
					hasArg bool
				}
				for j := range longs {
					if longs[j].short == c {
						l = &longs[j]
					}
				}
				if l == nil {
					usageErr("no such option: -%c", c)
				}
				if l.hasArg {
					val := a[k+1:]
					if val == "" {
						if i+1 >= len(argv) {
							usageErr("-%c option requires an argument", c)
						}
						i++
						val = argv[i]
					}
					apply(c, val)
					break
				}
				apply(c, "")
			}
		default:
			args = append(args, a)
		}
	}

	if len(args) == 0 {
		fmt.Println("usage: figlet [options] [text..]")
		os.Exit(1)
	}
	text := strings.Join(args, " ")

	raw, ok := readFontData(fontName)
	if !ok {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", fontName)
		os.Exit(1)
	}
	f := parseFont(fontName, raw)

	dir := direction
	if dir == "auto" {
		if f.printDirection == 1 {
			dir = "right-to-left"
		} else {
			dir = "left-to-right"
		}
	}
	just := justify
	if just == "auto" {
		if dir == "left-to-right" {
			just = "left"
		} else {
			just = "right"
		}
	}

	b := &builder{text: []rune(text), direction: dir, width: width, f: f, justify: just}
	b.buffer = b.newBuffer()
	r := b.render()
	if doReverse {
		r = reverseText(r)
	}
	if doFlip {
		r = flipText(r)
	}
	if doStrip {
		r = stripSurroundingNewlines(r)
	} else if doNormalize {
		r = "\n" + stripSurroundingNewlines(r) + "\n"
	}
	w := bufio.NewWriter(os.Stdout)
	w.WriteString(r)
	w.WriteString("\n")
	w.Flush()
}
GOEOF

# 4. the original is no longer needed: no Python source may remain anywhere
find "$REPO" -name '*.py' -type f -delete

# 5. prove it builds (the grader builds again, exactly like this)
go build ./...
go build -o /tmp/figlet .
echo "solve.sh: done"
