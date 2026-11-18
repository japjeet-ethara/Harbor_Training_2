#!/bin/bash
# Oracle: a Go port of pyfiglet's FIGlet rendering, written into /workspace/repo.
set -euo pipefail
cd /workspace/repo

# Fonts: keep the .flf files inside the repository and embed them (go:embed), so any cwd works.
mkdir -p fonts
cp pyfiglet/fonts-standard/*.flf fonts/

cat > go.mod <<'EOF_0'
module figlet

go 1.22
EOF_0

cat > main.go <<'EOF_1'
// Command figlet is a Go port of pyfiglet (https://github.com/pwaller/pyfiglet,
// commit 255827efbc09d662875dad50fa0edc1e1263bf45, MIT licence).
package main

import (
	"embed"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var fontFS embed.FS

type options struct {
	font      string
	direction string
	justify   string
	width     int
	reverse   bool
	flip      bool
	strip     bool
	normalize bool
}

func usageError(msg string) {
	fmt.Fprintf(os.Stderr, "Usage: figlet [options] [text..]\n\nfiglet: error: %s\n", msg)
	os.Exit(2)
}

func choice(val string, allowed ...string) string {
	for _, a := range allowed {
		if val == a {
			return val
		}
	}
	usageError(fmt.Sprintf("invalid choice: '%s' (choose from '%s')", val, strings.Join(allowed, "', '")))
	return ""
}

// parseArgs follows optparse: options may be mixed with text; "--" ends the options.
func parseArgs(argv []string) (options, []string) {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var rest []string
	setValue := func(name, val string) {
		switch name {
		case "f", "font":
			o.font = val
		case "D", "direction":
			o.direction = choice(val, "auto", "left-to-right", "right-to-left")
		case "j", "justify":
			o.justify = choice(val, "auto", "left", "center", "right")
		case "w", "width":
			n, err := strconv.Atoi(val)
			if err != nil {
				usageError(fmt.Sprintf("option -w: invalid integer value: '%s'", val))
			}
			o.width = n
		}
	}
	setFlag := func(name string) bool {
		switch name {
		case "r", "reverse":
			o.reverse = true
		case "F", "flip":
			o.flip = true
		case "s", "strip-surrounding-newlines":
			o.strip = true
		case "n", "normalize-surrounding-newlines":
			o.normalize = true
		default:
			return false
		}
		return true
	}
	isValueOpt := func(name string) bool {
		switch name {
		case "f", "font", "D", "direction", "j", "justify", "w", "width":
			return true
		}
		return false
	}

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			rest = append(rest, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--") && len(a) > 2:
			name, val, hasVal := strings.Cut(a[2:], "=")
			if isValueOpt(name) {
				if !hasVal {
					i++
					if i >= len(argv) {
						usageError("option --" + name + " requires an argument")
					}
					val = argv[i]
				}
				setValue(name, val)
			} else if !setFlag(name) {
				usageError("no such option: --" + name)
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			chars := []rune(a[1:])
			for k := 0; k < len(chars); k++ {
				name := string(chars[k])
				if isValueOpt(name) {
					val := string(chars[k+1:])
					if val == "" {
						i++
						if i >= len(argv) {
							usageError("-" + name + " option requires an argument")
						}
						val = argv[i]
					}
					setValue(name, val)
					break
				}
				if !setFlag(name) {
					usageError("no such option: -" + name)
				}
			}
		default:
			rest = append(rest, a)
		}
	}
	return o, rest
}

func loadNamedFont(name string) (*Font, bool) {
	if name == "" || strings.ContainsAny(name, "/\\") {
		return nil, false
	}
	data, err := fontFS.ReadFile("fonts/" + name + ".flf")
	if err != nil {
		return nil, false
	}
	f, err := loadFont(name, string(data))
	if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet.FontError:", err)
		os.Exit(1)
	}
	return f, true
}

func translate(row string, m map[rune]rune) string {
	rs := []rune(row)
	for i, c := range rs {
		if t, ok := m[c]; ok {
			rs[i] = t
		}
	}
	return string(rs)
}

func newFromList(rows []string) string { return strings.Join(rows, "\n") + "\n" }

func reverseText(s string) string {
	var out []string
	for _, row := range pySplitLines(s) {
		rs := []rune(translate(row, reverseMap))
		for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
			rs[i], rs[j] = rs[j], rs[i]
		}
		out = append(out, string(rs))
	}
	return newFromList(out)
}

func flipText(s string) string {
	rows := pySplitLines(s)
	var out []string
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translate(rows[i], flipMap))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s string) string {
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

func main() {
	opts, args := parseArgs(os.Args[1:])
	if len(args) == 0 {
		fmt.Println("Usage: figlet [options] [text..]")
		os.Exit(1)
	}
	text := strings.Join(args, " ")

	font, ok := loadNamedFont(opts.font)
	if !ok {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", opts.font)
		os.Exit(1)
	}

	direction := opts.direction
	if direction == "auto" {
		if font.printDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}
	justify := opts.justify
	if justify == "auto" {
		if direction == "left-to-right" {
			justify = "left"
		} else {
			justify = "right"
		}
	}

	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(charNotPrinted); ok {
				fmt.Fprintln(os.Stderr, "pyfiglet.CharNotPrinted: Width is not enough to print this character")
			} else {
				fmt.Fprintln(os.Stderr, "figlet: internal error:", r)
			}
			os.Exit(1)
		}
	}()

	r := newBuilder(text, font, direction, opts.width, justify).render()
	if opts.reverse {
		r = reverseText(r)
	}
	if opts.flip {
		r = flipText(r)
	}
	if opts.strip {
		r = stripSurroundingNewlines(r)
	} else if opts.normalize {
		r = "\n" + stripSurroundingNewlines(r) + "\n"
	}
	os.Stdout.WriteString(r + "\n")
}
EOF_1

cat > font.go <<'EOF_2'
package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Font mirrors pyfiglet.FigletFont after loadFont().
type Font struct {
	height         int
	hardBlank      string
	printDirection int // -1 means "not given" (Python None)
	smushMode      int
	chars          map[rune][]string
	width          map[rune]int
}

// pySpace mimics Python's str.isspace() for one character.
func pySpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

func pyStrip(s string) string  { return strings.TrimFunc(s, pySpace) }
func pyRstrip(s string) string { return strings.TrimRightFunc(s, pySpace) }

func pyFields(s string) []string { return strings.FieldsFunc(s, pySpace) }

// pySplitLines mimics Python's str.splitlines().
func pySplitLines(s string) []string {
	var out []string
	rs := []rune(s)
	start := 0
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		isSep := c == '\n' || c == '\r' || c == 0x0b || c == 0x0c || c == 0x1c || c == 0x1d || c == 0x1e ||
			c == 0x85 || c == 0x2028 || c == 0x2029
		if !isSep {
			continue
		}
		out = append(out, string(rs[start:i]))
		if c == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
			i++
		}
		start = i + 1
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

func runeLen(s string) int { return len([]rune(s)) }

// endMarker finds the font's end-of-line marker: regex (.)\s*$ -> the last non-space char
// (or the first char if the line is all whitespace).
func endMarker(line string) (rune, bool) {
	rs := []rune(line)
	for i := len(rs) - 1; i >= 0; i-- {
		if !pySpace(rs[i]) {
			return rs[i], true
		}
	}
	if len(rs) > 0 {
		return rs[0], true
	}
	return 0, false
}

// stripEnd mimics  re.compile(re.escape(end) + r'{1,2}\s*$').sub('', line).
func stripEnd(line string, end rune) string {
	rs := []rune(line)
	n := len(rs)
	for n > 0 && pySpace(rs[n-1]) && rs[n-1] != end {
		n--
	}
	// rs[:n] is the line without trailing whitespace (unless the marker itself is whitespace)
	m := 0
	for m < n && rs[n-1-m] == end {
		m++
	}
	if m == 0 {
		return line
	}
	cut := 1
	if m >= 2 {
		cut = 2
	}
	return string(rs[:n-cut])
}

func loadFont(name, raw string) (*Font, error) {
	data := string([]rune(raw)) // invalid UTF-8 bytes -> U+FFFD, like decode('UTF-8','replace')
	data = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(data)
	lines := pySplitLines(data)

	fail := func(format string, a ...interface{}) (*Font, error) {
		return nil, fmt.Errorf("problem parsing %s font: %s", name, fmt.Sprintf(format, a...))
	}
	pop := func() (string, error) {
		if len(lines) == 0 {
			return "", errors.New("pop from empty list")
		}
		l := lines[0]
		lines = lines[1:]
		return l, nil
	}

	header, err := pop()
	if err != nil {
		return fail("%v", err)
	}
	hr := []rune(header)
	if len(hr) < 5 || (hr[0] != 't' && hr[0] != 'f') || hr[1] != 'l' || hr[2] != 'f' || hr[3] != '2' {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}
	fields := pyFields(string(hr[5:]))
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	f := &Font{hardBlank: fields[0], printDirection: -1, chars: map[rune][]string{}, width: map[rune]int{}}
	var nums [5]int
	for i := 0; i < 5; i++ {
		v, err := strconv.Atoi(fields[1+i])
		if err != nil {
			return fail("%v", err)
		}
		nums[i] = v
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	hasFull := false
	fullLayout := 0
	if len(fields) > 6 {
		v, err := strconv.Atoi(fields[6])
		if err != nil {
			return fail("%v", err)
		}
		f.printDirection = v
	}
	if len(fields) > 7 {
		v, err := strconv.Atoi(fields[7])
		if err != nil {
			return fail("%v", err)
		}
		fullLayout, hasFull = v, true
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
		if _, err := pop(); err != nil {
			return fail("%v", err)
		}
	}

	readChar := func() (int, []string, error) {
		var end rune
		haveEnd := false
		width := 0
		rows := make([]string, 0, height)
		for i := 0; i < height; i++ {
			line, err := pop()
			if err != nil {
				return 0, nil, err
			}
			if !haveEnd {
				e, ok := endMarker(line)
				if !ok {
					return 0, nil, errors.New("'NoneType' object has no attribute 'group'")
				}
				end, haveEnd = e, true
			}
			line = stripEnd(line, end)
			if l := runeLen(line); l > width {
				width = l
			}
			rows = append(rows, line)
		}
		return width, rows, nil
	}
	nonEmpty := func(rows []string) bool { return strings.Join(rows, "") != "" }

	// ASCII 32..126
	for i := 32; i < 127; i++ {
		w, rows, err := readChar()
		if err != nil {
			return fail("%v", err)
		}
		if i == 32 || nonEmpty(rows) {
			f.chars[rune(i)] = rows
			f.width[rune(i)] = w
		}
	}
	// German umlauts follow directly after character 126
	if len(lines) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, rows, err := readChar()
			if err != nil {
				return fail("%v", err)
			}
			if nonEmpty(rows) {
				f.chars[c] = rows
				f.width[c] = w
			}
		}
	}
	// code-tagged characters
	for len(lines) > 0 {
		line, _ := pop()
		line = pyStrip(line)
		tag := line
		if k := strings.Index(line, " "); k >= 0 {
			tag = line[:k]
		}
		if tag == "" {
			continue
		}
		if len(tag) >= 2 && (tag[:2] == "0x" || tag[:2] == "0X") {
			code, err := strconv.ParseInt(tag[2:], 16, 64)
			if err != nil {
				return fail("invalid literal for int() with base 16: '%s'", tag)
			}
			w, rows, err := readChar()
			if err != nil {
				return fail("%v", err)
			}
			if nonEmpty(rows) {
				f.chars[rune(code)] = rows
				f.width[rune(code)] = w
			}
		}
	}
	return f, nil
}
EOF_2

cat > render.go <<'EOF_3'
package main

import (
	"strings"
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

type marker struct {
	buffer   []string
	iterator int
}

// builder mirrors pyfiglet.FigletBuilder (+ FigletProduct).
type builder struct {
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

	blankMarkers []marker
	queue        [][]string
	buffer       []string
}

func newBuilder(text string, font *Font, direction string, width int, justify string) *builder {
	b := &builder{
		text: []rune(text), direction: direction, width: width, font: font, justify: justify,
	}
	b.buffer = make([]string, font.height)
	return b
}

// charNotPrinted is raised when the width cannot hold a glyph (pyfiglet.CharNotPrinted).
type charNotPrinted struct{}

func copyRows(rows []string) []string { return append([]string(nil), rows...) }

func (b *builder) getCharAt(i int) []string {
	if i < 0 || i >= len(b.text) {
		return nil
	}
	if g, ok := b.font.chars[b.text[i]]; ok {
		return g
	}
	return nil
}

func (b *builder) getCharWidthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	if w, ok := b.font.width[b.text[i]]; ok {
		return w, true
	}
	return 0, false
}

func (b *builder) getCurChar() []string { return b.getCharAt(b.iterator) }

func (b *builder) addCharToProduct() {
	curChar := b.getCurChar()

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyRows(b.buffer), b.iterator})
		b.handleNewLine()
		return
	}
	if curChar == nil {
		return
	}
	w, _ := b.getCharWidthAt(b.iterator)
	if b.width < w {
		panic(charNotPrinted{})
	}
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = runeLen(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyRows(b.buffer), b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}
	b.prevCharWidth = b.curCharWidth
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		// cutBufferAtLastBlank
		b.queue = append(b.queue, m.buffer)
		b.iterator = m.iterator
	} else {
		// cutBufferAtLastChar
		b.queue = append(b.queue, b.buffer)
		b.iterator--
	}
	b.cutBufferCommon()
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar := b.getCurChar()
	if curChar == nil {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

func (b *builder) rtl() bool { return b.direction == "right-to-left" }

func (b *builder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	r := []rune(addRight)
	rest := ""
	if b.maxSmush < len(r) {
		rest = string(r[b.maxSmush:])
	}
	b.buffer[row] = addLeft + rest
}

func (b *builder) smushRow(curChar []string, row int) (string, string) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.rtl() {
		addLeft, addRight = addRight, addLeft
	}
	left := []rune(addLeft)
	right := []rune(addRight)
	for i := 0; i < b.maxSmush; i++ {
		idx := len(left) - b.maxSmush + i
		l := ""
		if idx >= 0 && idx < len(left) {
			l = string(left[idx])
		}
		r := string(right[i]) // out of range panics, like Python's IndexError
		smushed, ok := b.smushChars(l, r)
		if idx < 0 || idx > len(left) {
			continue
		}
		if !ok {
			panic("TypeError: sequence item: expected str instance, NoneType found")
		}
		sr := []rune(smushed)
		left = append(append(append([]rune(nil), left[:idx]...), sr...), left[idx+1:]...)
	}
	return string(left), addRight
}

func (b *builder) smushAmount(buffer []string, curChar []string) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.rtl() {
			lineLeft, lineRight = lineRight, lineLeft
		}
		ll := []rune(lineLeft)
		lr := []rune(lineRight)

		// Only strip ASCII space to match figlet exactly.
		linebd := len([]rune(strings.TrimRight(lineLeft, " "))) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := ""
		if linebd < len(ll) {
			ch1 = string(ll[linebd])
		} else {
			linebd = 0
		}

		charbd := len(lr) - len([]rune(strings.TrimLeft(lineRight, " ")))
		ch2 := ""
		if charbd < len(lr) {
			ch2 = string(lr[charbd])
		} else {
			charbd = len(lr)
		}

		amt := charbd + len(ll) - 1 - linebd

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

// smushChars returns (result, true), or ("", false) for Python's None.
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
	if mode&smSmush == 0 {
		return "", false
	}
	hb := b.font.hardBlank
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
			pair{"()", "<>"},
		)
	}
	// Python's "x in y" is a substring test (and "" is in everything); strings.Contains matches that.
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

func (b *builder) justifyRows(buffer []string) []string {
	switch b.justify {
	case "right":
		for row := 0; row < b.font.height; row++ {
			n := b.width - runeLen(buffer[row]) - 1
			if n > 0 {
				buffer[row] = strings.Repeat(" ", n) + buffer[row]
			}
		}
	case "center":
		for row := 0; row < b.font.height; row++ {
			n := (b.width - runeLen(buffer[row])) / 2 // Go truncates toward zero, like int(x/2)
			if n > 0 {
				buffer[row] = strings.Repeat(" ", n) + buffer[row]
			}
		}
	}
	return buffer
}

func (b *builder) render() string {
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	if b.buffer[0] != "" {
		b.queue = append(b.queue, b.buffer)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		buf = b.justifyRows(buf)
		s := strings.Join(buf, "\n") + "\n"
		sb.WriteString(strings.ReplaceAll(s, b.font.hardBlank, " "))
	}
	return sb.String()
}
EOF_3

cat > maps.go <<'EOF_4'
package main

// Generated from pyfiglet's FigletString.__reverse_map__ / __flip_map__ (only the entries that differ from identity).
var reverseMap = map[rune]rune{
	0x28: 0x29, // '(' -> ')'
	0x29: 0x28, // ')' -> '('
	0x2f: 0x5c, // '/' -> '\\'
	0x3c: 0x3e, // '<' -> '>'
	0x3e: 0x3c, // '>' -> '<'
	0x5b: 0x5d, // '[' -> ']'
	0x5c: 0x2f, // '\\' -> '/'
	0x5d: 0x5b, // ']' -> '['
	0x7b: 0x7d, // '{' -> '}'
	0x7d: 0x7b, // '}' -> '{'
}

var flipMap = map[rune]rune{
	0x2f: 0x5c, // '/' -> '\\'
	0x41: 0x56, // 'A' -> 'V'
	0x4d: 0x57, // 'M' -> 'W'
	0x50: 0x62, // 'P' -> 'b'
	0x52: 0x62, // 'R' -> 'b'
	0x56: 0x41, // 'V' -> 'A'
	0x57: 0x4d, // 'W' -> 'M'
	0x5c: 0x2f, // '\\' -> '/'
	0x5e: 0x76, // '^' -> 'v'
	0x5f: 0x2d, // '_' -> '-'
	0x62: 0x50, // 'b' -> 'P'
	0x6d: 0x77, // 'm' -> 'w'
	0x76: 0x5e, // 'v' -> '^'
	0x77: 0x6d, // 'w' -> 'm'
}
EOF_4

# Remove the Python project (this also removes fonts-standard/__init__.py and setup.py).
rm -rf pyfiglet doc tools test_font.sh
find . -name '*.py' -delete

go build -o /tmp/figlet .
go build ./...
