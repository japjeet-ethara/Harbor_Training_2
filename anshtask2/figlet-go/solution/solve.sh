#!/bin/bash
# Oracle: Go port of pyfiglet's FIGlet rendering (stdlib only; fonts embedded), then delete every .py.
# Local test: cp -r ../pyfiglet /tmp/work && REPO=/tmp/work bash solution/solve.sh
set -euo pipefail
REPO="${REPO:-/workspace/repo}"
cd "$REPO"

mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/

printf 'module figlet\n\ngo 1.23\n' > go.mod

cat > main.go <<'GO_EOF'
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

const none = rune(-1) // Python's '' (no character)

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
	hardblank rune
	height    int
	printDir  int
	smushMode int
	chars     map[rune][][]rune
	width     map[rune]int
}

func tagValue(line string) (int64, bool) {
	f := strings.Fields(line)
	if len(f) == 0 {
		return 0, false
	}
	v, err := strconv.ParseInt(f[0], 0, 64)
	return v, err == nil
}

func nonEmpty(rows [][]rune) bool {
	for _, r := range rows {
		if len(r) > 0 {
			return true
		}
	}
	return false
}

func loadFont(name string) (*font, error) {
	raw, err := fontFS.ReadFile("fonts/" + name + ".flf")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 || len(lines[0]) < 6 || !strings.HasPrefix(lines[0], "flf2") {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}
	hdr := strings.Fields(lines[0][5:])
	if len(hdr) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	n := make([]int, 5)
	for i := range n {
		v, err := strconv.Atoi(hdr[1+i])
		if err != nil {
			return nil, err
		}
		n[i] = v
	}
	f := &font{hardblank: []rune(hdr[0])[0], height: n[0], chars: map[rune][][]rune{}, width: map[rune]int{}}
	oldLayout, comments := n[3], n[4]
	if len(hdr) > 6 {
		f.printDir, _ = strconv.Atoi(hdr[6])
	}
	if len(hdr) > 7 {
		f.smushMode, _ = strconv.Atoi(hdr[7])
	} else if oldLayout == 0 {
		f.smushMode = 64
	} else if oldLayout < 0 {
		f.smushMode = 0
	} else {
		f.smushMode = (oldLayout & 31) | 128
	}
	var data []string
	if 1+comments <= len(lines) {
		data = lines[1+comments:]
	}
	pos := 0
	readChar := func() (int, [][]rune, bool) {
		if pos+f.height > len(data) {
			return 0, nil, false
		}
		var rows [][]rune
		w := 0
		var endc rune
		for j := 0; j < f.height; j++ {
			line := data[pos]
			pos++
			trimmed := []rune(strings.TrimRightFunc(line, unicode.IsSpace))
			if j == 0 {
				if len(trimmed) == 0 {
					return 0, nil, false
				}
				endc = trimmed[len(trimmed)-1]
			}
			rr := trimmed
			k := 0
			for k < 2 && len(rr) > 0 && rr[len(rr)-1] == endc {
				rr = rr[:len(rr)-1]
				k++
			}
			if k == 0 {
				rr = []rune(line)
			}
			if len(rr) > w {
				w = len(rr)
			}
			rows = append(rows, rr)
		}
		return w, rows, true
	}
	store := func(c rune, w int, rows [][]rune) {
		if nonEmpty(rows) {
			f.chars[c] = rows
			f.width[c] = w
		}
	}
	for c := rune(32); c < 127; c++ {
		w, rows, ok := readChar()
		if !ok {
			break
		}
		store(c, w, rows)
	}
	if pos < len(data) {
		if _, tag := tagValue(data[pos]); !tag {
			for _, c := range []rune("ÄÖÜäöüß") {
				w, rows, ok := readChar()
				if !ok {
					break
				}
				store(c, w, rows)
			}
		}
	}
	for pos < len(data) {
		line := data[pos]
		pos++
		v, ok := tagValue(line)
		if !ok {
			continue
		}
		w, rows, ok2 := readChar()
		if !ok2 {
			break
		}
		if v >= 0 {
			store(rune(v), w, rows)
		}
	}
	return f, nil
}

type marker struct {
	buf [][]rune
	it  int
}

type builder struct {
	text     []rune
	font     *font
	dir      string
	width    int
	justify  string
	iter     int
	maxSmush int
	curW     int
	prevW    int
	total    int
	buffer   [][]rune
	blanks   []marker
	queue    [][][]rune
}

func (b *builder) emptyBuf() [][]rune {
	return make([][]rune, b.font.height)
}

func copyBuf(buf [][]rune) [][]rune {
	out := make([][]rune, len(buf))
	for i, r := range buf {
		out[i] = append([]rune(nil), r...)
	}
	return out
}

func rstrip(r []rune) []rune {
	n := len(r)
	for n > 0 && unicode.IsSpace(r[n-1]) {
		n--
	}
	return r[:n]
}

func lstripLen(r []rune) int {
	n := 0
	for n < len(r) && unicode.IsSpace(r[n]) {
		n++
	}
	return n
}

func inStr(c rune, set string) bool {
	if c == none {
		return true // '' in "abc" is True in Python
	}
	return strings.ContainsRune(set, c)
}

func toStr(c rune) string {
	if c == none {
		return ""
	}
	return string(c)
}

func (b *builder) smushChars(left, right rune) (rune, bool) {
	if unicode.IsSpace(left) {
		return right, true
	}
	if unicode.IsSpace(right) {
		return left, true
	}
	if b.prevW < 2 || b.curW < 2 {
		return 0, false
	}
	m := b.font.smushMode
	if m&smSmush == 0 {
		return 0, false
	}
	hb := b.font.hardblank
	if m&63 == 0 {
		if left == hb {
			return right, true
		}
		if right == hb {
			return left, true
		}
		if b.dir == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if m&smHardblank != 0 && left == hb && right == hb {
		return left, true
	}
	if left == hb || right == hb {
		return 0, false
	}
	if m&smEqual != 0 && left == right {
		return left, true
	}
	type pair struct{ x, y string }
	var sm []pair
	if m&smLowline != 0 {
		sm = append(sm, pair{"_", `|/\[]{}()<>`})
	}
	if m&smHierarchy != 0 {
		sm = append(sm, pair{"|", `/\[]{}()<>`}, pair{`\/`, "[]{}()<>"}, pair{"[]", "{}()<>"}, pair{"{}", "()<>"}, pair{"()", "<>"})
	}
	for _, p := range sm {
		if inStr(left, p.x) && inStr(right, p.y) {
			return right, true
		}
		if inStr(right, p.x) && inStr(left, p.y) {
			return left, true
		}
	}
	if m&smPair != 0 {
		for _, pr := range []string{toStr(left) + toStr(right), toStr(right) + toStr(left)} {
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

func (b *builder) charAt(i int) [][]rune {
	if i < 0 || i >= len(b.text) {
		return nil
	}
	return b.font.chars[b.text[i]]
}

func (b *builder) smushAmount(cur [][]rune) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxS := b.curW
	for row := 0; row < b.font.height; row++ {
		left, right := b.buffer[row], cur[row]
		if b.dir == "right-to-left" {
			left, right = right, left
		}
		linebd := len(rstrip(left)) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := none
		if linebd < len(left) {
			ch1 = left[linebd]
		} else {
			linebd = 0
		}
		charbd := lstripLen(right)
		ch2 := none
		if charbd < len(right) {
			ch2 = right[charbd]
		} else {
			charbd = len(right)
		}
		amt := charbd + len(left) - 1 - linebd
		if ch1 == none || ch1 == ' ' {
			amt++
		} else if ch2 != none {
			if _, ok := b.smushChars(ch1, ch2); ok {
				amt++
			}
		}
		if amt < maxS {
			maxS = amt
		}
	}
	return maxS
}

func (b *builder) addRow(cur [][]rune, row int) {
	addLeft, addRight := b.buffer[row], cur[row]
	if b.dir == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	l := append([]rune(nil), addLeft...)
	for i := 0; i < b.maxSmush; i++ {
		idx := len(l) - b.maxSmush + i
		left := none
		if idx >= 0 && idx < len(l) {
			left = l[idx]
		}
		right := none
		if i < len(addRight) {
			right = addRight[i]
		}
		if sm, ok := b.smushChars(left, right); ok && idx >= 0 && idx < len(l) && sm != none {
			l[idx] = sm
		}
	}
	rest := addRight
	if b.maxSmush < len(rest) {
		rest = rest[b.maxSmush:]
	} else {
		rest = nil
	}
	b.buffer[row] = append(l, rest...)
}

func (b *builder) cutCommon() {
	b.total = 0
	b.buffer = b.emptyBuf()
	b.blanks = nil
	b.prevW = 0
	if b.charAt(b.iter) == nil {
		return
	}
	b.curW = b.font.width[b.text[b.iter]]
}

func (b *builder) add() error {
	cur := b.charAt(b.iter)
	if cur == nil {
		return nil
	}
	w := b.font.width[b.text[b.iter]]
	if b.width < w {
		return fmt.Errorf("width is not enough to print this character")
	}
	b.curW = w
	b.maxSmush = b.smushAmount(cur)
	b.total = len(b.buffer[0]) + w - b.maxSmush
	if b.text[b.iter] == ' ' {
		b.blanks = append(b.blanks, marker{copyBuf(b.buffer), b.iter})
	}
	if b.total >= b.width {
		if len(b.queue) > 200000 {
			return fmt.Errorf("cannot wrap")
		}
		if len(b.blanks) > 0 {
			m := b.blanks[len(b.blanks)-1]
			b.blanks = b.blanks[:len(b.blanks)-1]
			b.queue = append(b.queue, m.buf)
			b.iter = m.it
		} else {
			b.queue = append(b.queue, copyBuf(b.buffer))
			b.iter--
		}
		b.cutCommon()
		return nil
	}
	for row := 0; row < b.font.height; row++ {
		b.addRow(cur, row)
	}
	b.prevW = b.curW
	return nil
}

func render(text string, f *font, dir string, width int, justify string) (string, error) {
	b := &builder{text: []rune(text), font: f, dir: dir, width: width, justify: justify}
	b.buffer = b.emptyBuf()
	for b.iter < len(b.text) {
		if err := b.add(); err != nil {
			return "", err
		}
		b.iter++
	}
	if len(b.buffer[0]) > 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		rows := make([]string, len(buf))
		for r, row := range buf {
			pad := 0
			switch b.justify {
			case "right":
				pad = b.width - len(row) - 1
			case "center":
				pad = (b.width - len(row)) / 2
			}
			if pad < 0 {
				pad = 0
			}
			rows[r] = strings.Repeat(" ", pad) + string(row)
		}
		s := strings.Join(rows, "\n") + "\n"
		sb.WriteString(strings.ReplaceAll(s, string(f.hardblank), " "))
	}
	return sb.String(), nil
}

func splitRows(s string) []string {
	if s == "" {
		return nil
	}
	r := strings.Split(s, "\n")
	if r[len(r)-1] == "" {
		r = r[:len(r)-1]
	}
	return r
}

func fromRows(rows []string) string { return strings.Join(rows, "\n") + "\n" }

func translate(s string, m map[rune]rune) string {
	r := []rune(s)
	for i, c := range r {
		if t, ok := m[c]; ok {
			r[i] = t
		}
	}
	return string(r)
}

var reverseMap = map[rune]rune{'/': '\\', '\\': '/', '(': ')', ')': '(', '<': '>', '>': '<', '[': ']', ']': '[', '{': '}', '}': '{'}
var flipMap = map[rune]rune{'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M', '/': '\\', '\\': '/', '^': 'v', 'v': '^', '_': '-', '-': '_'}

func reverse(s string) string {
	rows := splitRows(s)
	for i, r := range rows {
		rr := []rune(translate(r, reverseMap))
		for a, z := 0, len(rr)-1; a < z; a, z = a+1, z-1 {
			rr[a], rr[z] = rr[z], rr[a]
		}
		rows[i] = string(rr)
	}
	return fromRows(rows)
}

func flip(s string) string {
	rows := splitRows(s)
	out := make([]string, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translate(rows[i], flipMap))
	}
	return fromRows(out)
}

func stripNL(s string) string {
	rows := splitRows(s)
	for len(rows) > 0 && strings.TrimSpace(rows[0]) == "" {
		rows = rows[1:]
	}
	for len(rows) > 0 && strings.TrimSpace(rows[len(rows)-1]) == "" {
		rows = rows[:len(rows)-1]
	}
	return fromRows(rows)
}

func main() {
	fontName, justify, direction, width := "standard", "auto", "auto", 80
	var rev, flp, strip, norm bool
	var text []string
	set := func(k, v string) {
		switch k {
		case "f", "font":
			fontName = v
		case "w", "width":
			if n, err := strconv.Atoi(v); err == nil {
				width = n
			}
		case "j", "justify":
			justify = v
		case "D", "direction":
			direction = v
		}
	}
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			text = append(text, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "--") {
			name, v, has := strings.Cut(a[2:], "=")
			switch name {
			case "font", "width", "justify", "direction":
				if !has && i+1 < len(args) {
					i++
					v = args[i]
				}
				set(name, v)
			case "reverse":
				rev = true
			case "flip":
				flp = true
			case "strip-surrounding-newlines":
				strip = true
			case "normalize-surrounding-newlines":
				norm = true
			}
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			for j := 1; j < len(a); j++ {
				switch c := a[j]; c {
				case 'r':
					rev = true
				case 'F':
					flp = true
				case 's':
					strip = true
				case 'n':
					norm = true
				case 'f', 'w', 'j', 'D':
					v := a[j+1:]
					if v == "" && i+1 < len(args) {
						i++
						v = args[i]
					}
					set(string(c), v)
					j = len(a)
				}
			}
			continue
		}
		text = append(text, a)
	}
	f, err := loadFont(fontName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "font error:", err)
		os.Exit(1)
	}
	dir := direction
	if dir == "auto" {
		if f.printDir == 1 {
			dir = "right-to-left"
		} else {
			dir = "left-to-right"
		}
	}
	just := justify
	if just == "auto" {
		if dir == "right-to-left" {
			just = "right"
		} else {
			just = "left"
		}
	}
	out, err := render(strings.Join(text, " "), f, dir, width, just)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if rev {
		out = reverse(out)
	}
	if flp {
		out = flip(out)
	}
	if strip {
		out = stripNL(out)
	}
	if norm {
		out = "\n" + stripNL(out) + "\n"
	}
	os.Stdout.WriteString(out + "\n")
}
GO_EOF

go build ./...
# the library, its tests and setup files must all go
rm -rf pyfiglet
find "$REPO" -name '*.py' -delete
find "$REPO" -name '__pycache__' -type d -prune -exec rm -rf {} + 2>/dev/null || true
