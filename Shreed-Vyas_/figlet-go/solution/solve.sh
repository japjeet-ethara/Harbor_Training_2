#!/bin/bash
# Oracle solution: writes the Go port of pyfiglet into /workspace/repo/figlet and builds it.
set -euo pipefail
mkdir -p /workspace/repo/figlet
cd /workspace/repo/figlet
cat > go.mod <<'GOMOD'
module figlet

go 1.23
GOMOD
cat > main.go <<'GOEOF'
// figlet: Go port of pyfiglet (FigletFont + FigletBuilder + FigletString). Standard library only.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
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

// ---------------------------------------------------------------- font

type figFont struct {
	hardBlank      string
	height         int
	printDirection int
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

func fontDirs() []string {
	var dirs []string
	if d := os.Getenv("FIGLET_FONTS"); d != "" {
		dirs = append(dirs, d)
	}
	return append(dirs, "pyfiglet/fonts", "/workspace/repo/pyfiglet/fonts")
}

func loadFont(name string) (*figFont, error) {
	name = strings.TrimSuffix(name, ".flf")
	for _, dir := range fontDirs() {
		for _, ext := range []string{".flf", ".tlf"} {
			data, err := os.ReadFile(filepath.Join(dir, name+ext))
			if err == nil {
				return parseFont(data)
			}
		}
	}
	return nil, fmt.Errorf("font %q not found", name)
}

func parseFont(data []byte) (*figFont, error) {
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(src, "\n")
	head := strings.Fields(lines[0])
	if len(head) < 6 || len([]rune(head[0])) < 6 ||
		!(strings.HasPrefix(head[0], "flf2") || strings.HasPrefix(head[0], "tlf2")) {
		return nil, fmt.Errorf("not a valid figlet font")
	}
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	f := &figFont{
		hardBlank: string([]rune(head[0])[5:]),
		chars:     map[int][][]rune{},
		width:     map[int]int{},
	}
	f.height = atoi(head[1])
	if f.height < 1 {
		return nil, fmt.Errorf("bad font height")
	}
	oldLayout := atoi(head[4])
	comments := atoi(head[5])
	if len(head) > 6 {
		f.printDirection = atoi(head[6])
	}
	if len(head) > 7 {
		f.smushMode = atoi(head[7])
	} else if oldLayout == 0 {
		f.smushMode = 64
	} else if oldLayout < 0 {
		f.smushMode = 0
	} else {
		f.smushMode = (oldLayout & 31) | 128
	}

	pos := 1 + comments
	readGlyph := func() ([][]rune, int, bool) {
		if pos+f.height > len(lines) {
			return nil, 0, false
		}
		rows := make([][]rune, f.height)
		w := 0
		var end rune
		for k := 0; k < f.height; k++ {
			line := []rune(lines[pos])
			pos++
			trimSpace := func() {
				for len(line) > 0 && unicode.IsSpace(line[len(line)-1]) {
					line = line[:len(line)-1]
				}
			}
			trimSpace()
			if k == 0 && len(line) > 0 {
				end = line[len(line)-1]
			}
			for count := 0; count < 2 && len(line) > 0 && line[len(line)-1] == end; count++ {
				line = line[:len(line)-1]
			}
			if len(line) > w {
				w = len(line)
			}
			rows[k] = line
		}
		return rows, w, true
	}

	required := []int{}
	for i := 32; i < 127; i++ {
		required = append(required, i)
	}
	required = append(required, 196, 214, 220, 228, 246, 252, 223)
	for _, code := range required {
		rows, w, ok := readGlyph()
		if !ok {
			return f, nil
		}
		if code == 32 || w > 0 {
			f.chars[code] = rows
			f.width[code] = w
		}
	}
	// code-tagged characters
	for pos < len(lines) {
		tag := strings.TrimSpace(lines[pos])
		pos++
		if tag == "" {
			continue
		}
		tok := strings.Fields(tag)[0]
		if !strings.HasPrefix(strings.ToLower(tok), "0x") {
			continue
		}
		code, err := strconv.ParseInt(tok[2:], 16, 64)
		rows, w, ok := readGlyph()
		if !ok {
			break
		}
		if err != nil {
			continue
		}
		if w > 0 {
			f.chars[int(code)] = rows
			f.width[int(code)] = w
		}
	}
	return f, nil
}

// ---------------------------------------------------------------- helpers

func isSpaceStr(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func rstripLen(r []rune) int {
	n := len(r)
	for n > 0 && r[n-1] == ' ' {
		n--
	}
	return n
}

func leadingSpaces(r []rune) int {
	i := 0
	for i < len(r) && r[i] == ' ' {
		i++
	}
	return i
}

func copyBuf(b [][]rune) [][]rune {
	out := make([][]rune, len(b))
	for i, r := range b {
		out[i] = append([]rune(nil), r...)
	}
	return out
}

// ---------------------------------------------------------------- builder

type marker struct {
	buf  [][]rune
	iter int
}

type builder struct {
	text              []rune
	direction         string
	width             int
	font              *figFont
	justify           string
	iter              int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []marker
	product           []string
	buffer            [][]rune
}

func (b *builder) emptyBuffer() [][]rune {
	return make([][]rune, b.font.height)
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
	sm := b.font.smushMode
	if sm&smSmush == 0 {
		return "", false
	}
	hb := b.font.hardBlank
	if sm&63 == 0 { // universal overlapping
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
	if sm&smHardblank != 0 && left == hb && right == hb {
		return left, true
	}
	if left == hb || right == hb {
		return "", false
	}
	if sm&smEqual != 0 && left == right {
		return left, true
	}
	type rule struct{ a, b string }
	var smushes []rule
	if sm&smLowline != 0 {
		smushes = append(smushes, rule{"_", "|/\\[]{}()<>"})
	}
	if sm&smHierarchy != 0 {
		smushes = append(smushes,
			rule{"|", "/\\[]{}()<>"},
			rule{"\\/", "[]{}()<>"},
			rule{"[]", "{}()<>"},
			rule{"{}", "()<>"},
			rule{"()", "<>"})
	}
	for _, r := range smushes {
		if strings.Contains(r.a, left) && strings.Contains(r.b, right) {
			return right, true
		}
		if strings.Contains(r.a, right) && strings.Contains(r.b, left) {
			return left, true
		}
	}
	if sm&smPair != 0 {
		for _, pair := range []string{left + right, right + left} {
			if pair == "[]" || pair == "{}" || pair == "()" {
				return "|", true
			}
		}
	}
	if sm&smBigX != 0 {
		if left == "/" && right == "\\" {
			return "|", true
		}
		if right == "/" && left == "\\" {
			return "Y", true
		}
		if left == ">" && right == "<" {
			return "X", true
		}
	}
	return "", false
}

func (b *builder) smushAmount(buf [][]rune, cur [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft, lineRight := buf[row], cur[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}
		linebd := rstripLen(lineLeft) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := ""
		if linebd < len(lineLeft) {
			ch1 = string(lineLeft[linebd])
		} else {
			linebd = 0
		}
		charbd := leadingSpaces(lineRight)
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

func (b *builder) smushRow(cur [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := cur[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	addLeft = append([]rune(nil), addLeft...)
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		if idx < 0 || idx >= len(addLeft) {
			continue
		}
		left := string(addLeft[idx])
		right := ""
		if i < len(addRight) {
			right = string(addRight[i])
		}
		if s, ok := b.smushChars(left, right); ok {
			if rs := []rune(s); len(rs) == 1 {
				addLeft[idx] = rs[0]
			}
		}
	}
	return addLeft, addRight
}

func (b *builder) addRow(cur [][]rune, row int) {
	left, right := b.smushRow(cur, row)
	n := b.maxSmush
	if n > len(right) {
		n = len(right)
	}
	if n < 0 {
		n = 0
	}
	b.buffer[row] = append(append([]rune(nil), left...), right[n:]...)
}

func (b *builder) block(buf [][]rune) string {
	rows := make([]string, len(buf))
	for i, r := range buf {
		row := string(r)
		switch b.justify {
		case "right":
			if p := b.width - len(r) - 1; p > 0 {
				row = strings.Repeat(" ", p) + row
			}
		case "center":
			if p := (b.width - len(r)) / 2; p > 0 {
				row = strings.Repeat(" ", p) + row
			}
		}
		if b.font.hardBlank != "" {
			row = strings.ReplaceAll(row, b.font.hardBlank, " ")
		}
		rows[i] = row
	}
	return strings.Join(rows, "\n") + "\n"
}

func (b *builder) cutCommon() {
	b.currentTotalWidth = 0
	b.buffer = b.emptyBuffer()
	b.blankMarkers = nil
	b.prevCharWidth = 0
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.product = append(b.product, b.block(m.buf))
		b.iter = m.iter
	} else {
		b.product = append(b.product, b.block(b.buffer))
		b.iter--
	}
	b.cutCommon()
}

func (b *builder) addChar() error {
	ch := b.text[b.iter]
	if ch == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iter})
		b.handleNewLine()
		return nil
	}
	cur, ok := b.font.chars[int(ch)]
	if !ok {
		return nil // character missing from the font: skipped
	}
	cw := b.font.width[int(ch)]
	if b.width < cw {
		return fmt.Errorf("character %q is wider than the output width", ch)
	}
	b.curCharWidth = cw
	b.maxSmush = b.smushAmount(b.buffer, cur)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush
	if ch == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuf(b.buffer), b.iter})
	}
	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
		b.prevCharWidth = b.curCharWidth
		return nil
	}
	for row := 0; row < b.font.height; row++ {
		b.addRow(cur, row)
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func renderText(f *figFont, text string, direction string, width int, justify string) (string, error) {
	b := &builder{text: []rune(text), direction: direction, width: width, font: f, justify: justify}
	b.buffer = b.emptyBuffer()
	for b.iter = 0; b.iter < len(b.text); {
		if err := b.addChar(); err != nil {
			return "", err
		}
		b.iter++
	}
	if len(b.buffer[0]) > 0 {
		b.product = append(b.product, b.block(b.buffer))
	}
	return strings.Join(b.product, ""), nil
}

// ---------------------------------------------------------------- FigletString.reverse / flip
// NOTE: reconstructed translation tables - verify against __reverse_map__ / __flip_map__ in pyfiglet/__init__.py.

func translationMap(ascii string) map[rune]rune {
	m := make(map[rune]rune, len(ascii))
	for i, r := range ascii {
		m[rune(i+32)] = r
	}
	return m
}

var reverseMap = translationMap(
	` !"#$%&')(*+,-.\` +
		`0123456789:;>=<?` +
		`@ABCDEFGHIJKLMNO` +
		`PQRSTUVWXYZ]/[^_` +
		"`abcdefghijklmno" +
		`pqrstuvwxyz}|{~` +
		"\x7f",
)

var flipMap = translationMap(
	` !"#$%&'()*+,-.\` +
		`0123456789:;<=>?` +
		`@VBCDEFGHIJKLWNO` +
		`bQbSTUAMXYZ[/]v-` +
		"`aPcdefghijklwno" +
		`pqrstu^mxyz{|}~` +
		"\x7f",
)

func translate(s string, m map[rune]rune) string {
	rs := []rune(s)
	for i, r := range rs {
		if t, ok := m[r]; ok {
			rs[i] = t
		}
	}
	return string(rs)
}

func reverseRunes(s string) string {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}
	return string(rs)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func doReverse(s string) string {
	lines := splitLines(s)
	for i, l := range lines {
		lines[i] = reverseRunes(translate(l, reverseMap))
	}
	return strings.Join(lines, "\n") + "\n"
}

func doFlip(s string) string {
	lines := splitLines(s)
	out := make([]string, len(lines))
	for i, l := range lines {
		out[len(lines)-1-i] = translate(l, flipMap)
	}
	return strings.Join(out, "\n") + "\n"
}

// ---------------------------------------------------------------- CLI

func usage(msg string) {
	fmt.Fprintln(os.Stderr, "figlet:", msg)
	os.Exit(2)
}

func main() {
	fontName, width := "standard", 80
	justify, direction := "auto", "auto"
	reverse, flip, noSmush := false, false, false

	args := os.Args[1:]
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if len(a) < 2 || a[0] != '-' {
			break
		}
		switch a {
		case "-r":
			reverse = true
		case "-F":
			flip = true
		case "-s":
			noSmush = false
		case "-n":
			noSmush = true
		case "-f", "-w", "-j", "-D":
			if i+1 >= len(args) {
				usage("option " + a + " needs a value")
			}
			v := args[i+1]
			i++
			switch a {
			case "-f":
				fontName = v
			case "-w":
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 {
					usage("invalid width " + v)
				}
				width = n
			case "-j":
				if v != "auto" && v != "left" && v != "center" && v != "right" {
					usage("invalid justification " + v)
				}
				justify = v
			case "-D":
				if v != "auto" && v != "left-to-right" && v != "right-to-left" {
					usage("invalid direction " + v)
				}
				direction = v
			}
		default:
			usage("unknown option " + a)
		}
		i++
	}

	var text string
	if rest := args[i:]; len(rest) > 0 {
		text = strings.Join(rest, " ")
	} else {
		data, _ := io.ReadAll(os.Stdin)
		text = strings.TrimSuffix(string(data), "\n")
	}

	f, err := loadFont(fontName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "figlet:", err)
		os.Exit(1)
	}
	if noSmush {
		f.smushMode = 0
	}
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
	out, err := renderText(f, text, dir, width, just)
	if err != nil {
		fmt.Fprintln(os.Stderr, "figlet:", err)
		os.Exit(1)
	}
	if reverse {
		out = doReverse(out)
	}
	if flip {
		out = doFlip(out)
	}
	fmt.Fprint(os.Stdout, out+"\n")
}
GOEOF
gofmt -l . >/dev/null 2>&1 || true
go build -o figlet .
echo "built /workspace/repo/figlet/figlet"
