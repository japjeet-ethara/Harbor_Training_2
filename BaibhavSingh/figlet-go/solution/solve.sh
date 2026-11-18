#!/bin/bash
# Oracle solution for figlet-go: writes a Go port of pyfiglet into /workspace/repo,
# copies the standard .flf fonts into the repository (embedded with go:embed),
# then deletes every .py file. Standard library only. No Python is used.
set -euo pipefail
cd /workspace/repo

mkdir -p fonts
cp pyfiglet/fonts-standard/*.flf fonts/

cat > go.mod <<'GO_SOURCE_EOF'
module figlet

go 1.23
GO_SOURCE_EOF

cat > main.go <<'GO_SOURCE_EOF'
// Command figlet is a Go port of pyfiglet's command-line program
// (pwaller/pyfiglet @ 255827efbc09d662875dad50fa0edc1e1263bf45, MIT).
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

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

const usage = "Usage: figlet [options] [text..]\n"

// optparse-style parsing: short options with separate or attached values,
// long options with "=" or separate values, unambiguous long prefixes,
// clustered short flags, interspersed positional args and "--".
func parseArgs(argv []string) (options, []string, error) {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var pos []string

	valueShort := map[byte]string{'f': "font", 'D': "direction", 'j': "justify", 'w': "width", 'L': "load", 'c': "color"}
	flagShort := map[byte]string{'r': "reverse", 'F': "flip", 's': "strip-surrounding-newlines",
		'n': "normalize-surrounding-newlines", 'l': "list_fonts", 'i': "info_font", 'h': "help"}
	longs := []string{"font", "direction", "justify", "width", "reverse", "flip",
		"strip-surrounding-newlines", "normalize-surrounding-newlines", "list_fonts",
		"info_font", "load", "color", "help", "version"}
	takesValue := map[string]bool{"font": true, "direction": true, "justify": true, "width": true, "load": true, "color": true}

	apply := func(name, val string) error {
		switch name {
		case "font":
			o.font = val
		case "direction":
			if val != "auto" && val != "left-to-right" && val != "right-to-left" {
				return fmt.Errorf("option -D: invalid choice: %q", val)
			}
			o.direction = val
		case "justify":
			if val != "auto" && val != "left" && val != "center" && val != "right" {
				return fmt.Errorf("option -j: invalid choice: %q", val)
			}
			o.justify = val
		case "width":
			n, err := parseOptInt(val)
			if err != nil {
				return fmt.Errorf("option -w: invalid integer value: %q", val)
			}
			o.width = n
		case "reverse":
			o.reverse = true
		case "flip":
			o.flip = true
		case "strip-surrounding-newlines":
			o.strip = true
		case "normalize-surrounding-newlines":
			o.normalize = true
		case "help":
			fmt.Print(usage)
			os.Exit(0)
		case "version":
			fmt.Println("1.0.4")
			os.Exit(0)
		}
		// list_fonts, info_font, load, color: accepted, not part of the port.
		return nil
	}

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			pos = append(pos, argv[i+1:]...)
			return o, pos, nil
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a[2:], "=")
			full := ""
			for _, l := range longs {
				if l == name {
					full = l
					break
				}
				if strings.HasPrefix(l, name) {
					if full != "" {
						return o, nil, fmt.Errorf("ambiguous option: --%s", name)
					}
					full = l
				}
			}
			if full == "" {
				return o, nil, fmt.Errorf("no such option: --%s", name)
			}
			if takesValue[full] {
				if !hasVal {
					if i+1 >= len(argv) {
						return o, nil, fmt.Errorf("--%s option requires an argument", full)
					}
					i++
					val = argv[i]
				}
			} else if hasVal {
				return o, nil, fmt.Errorf("--%s option does not take a value", full)
			}
			if err := apply(full, val); err != nil {
				return o, nil, err
			}
		case len(a) > 1 && a[0] == '-':
			for k := 1; k < len(a); k++ {
				c := a[k]
				if name, ok := valueShort[c]; ok {
					val := a[k+1:]
					if val == "" {
						if i+1 >= len(argv) {
							return o, nil, fmt.Errorf("-%c option requires an argument", c)
						}
						i++
						val = argv[i]
					}
					if err := apply(name, val); err != nil {
						return o, nil, err
					}
					break
				}
				name, ok := flagShort[c]
				if !ok {
					return o, nil, fmt.Errorf("no such option: -%c", c)
				}
				if err := apply(name, ""); err != nil {
					return o, nil, err
				}
			}
		default:
			pos = append(pos, a)
		}
	}
	return o, pos, nil
}

// parseOptInt mirrors optparse's int conversion (0x.., 0b.., leading 0 = octal).
func parseOptInt(s string) (int, error) {
	t := strings.TrimSpace(s)
	sign := ""
	if strings.HasPrefix(t, "-") || strings.HasPrefix(t, "+") {
		sign, t = t[:1], t[1:]
	}
	base := 10
	switch {
	case len(t) > 2 && strings.EqualFold(t[:2], "0x"):
		base, t = 16, t[2:]
	case len(t) > 2 && strings.EqualFold(t[:2], "0b"):
		base, t = 2, t[2:]
	case len(t) > 1 && t[0] == '0':
		base, t = 8, t[1:]
	}
	n, err := strconv.ParseInt(sign+strings.ReplaceAll(t, "_", ""), base, 64)
	return int(n), err
}

func main() {
	opts, args, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprint(os.Stderr, usage)
		fmt.Fprintf(os.Stderr, "figlet: error: %v\n", err)
		os.Exit(2)
	}
	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(1)
	}
	text := []rune(strings.Join(args, " "))

	font, err := loadFont(opts.font)
	if err == errFontNotFound {
		fmt.Printf("pyfiglet error: requested font %s not found.\n", pyRepr(opts.font))
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet.FontError:", err)
		os.Exit(1)
	}

	r, err := renderText(text, font, opts.direction, opts.justify, opts.width)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet.CharNotPrinted:", err)
		os.Exit(1)
	}
	if opts.reverse {
		r = fsReverse(r)
	}
	if opts.flip {
		r = fsFlip(r)
	}
	if opts.strip {
		r = fsStripSurroundingNewlines(r)
	} else if opts.normalize {
		r = fsNormalizeSurroundingNewlines(r)
	}
	os.Stdout.WriteString(string(r) + "\n")
}

// pyRepr is a close-enough repr() for the font-not-found message.
func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}
GO_SOURCE_EOF

cat > font.go <<'GO_SOURCE_EOF'
package main

// Port of pyfiglet.FigletFont (loading and parsing only).

import (
	"archive/zip"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
)

//go:embed fonts
var fontFS embed.FS

var errFontNotFound = errors.New("font not found")

// pyWS is Python's `\s` for str patterns, as a regexp class.
const pyWS = `[\t\n\v\f\r\x1c-\x20\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

var (
	reMagicNumber = regexp.MustCompile(`^[tf]lf2.`)
	reEndMarker   = regexp.MustCompile(`(.)` + pyWS + `*$`)
)

type Font struct {
	name           string
	height         int
	hardBlank      string
	printDirection *int
	smushMode      int
	chars          map[int][][]rune
	width          map[int]int
}

func preloadFont(name string) ([]rune, error) {
	for _, ext := range []string{"tlf", "flf"} {
		data, err := fs.ReadFile(fontFS, "fonts/"+name+"."+ext)
		if err != nil {
			continue
		}
		if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil && len(zr.File) > 0 {
			f, err := zr.File[0].Open()
			if err != nil {
				return nil, err
			}
			data, err = io.ReadAll(f)
			f.Close()
			if err != nil {
				return nil, err
			}
		}
		if len(data) == 0 {
			break
		}
		return decodeUTF8Replace(data), nil
	}
	return nil, errFontNotFound
}

func pyInt(s string) (int, error) {
	s = strings.ReplaceAll(s, "_", "")
	return strconv.Atoi(s)
}

func loadFont(name string) (font *Font, err error) {
	raw, err := preloadFont(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			font, err = nil, fmt.Errorf("problem parsing %s font: %v", name, r)
		}
	}()

	for i, r := range raw {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			raw[i] = ' '
		}
	}
	data := pySplitlines(raw)
	pop := func() []rune {
		if len(data) == 0 {
			panic("pop from empty list")
		}
		l := data[0]
		data = data[1:]
		return l
	}

	header := string(pop())
	if !reMagicNumber.MatchString(header) {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}
	header = reMagicNumber.ReplaceAllString(header, "")
	fields := pyFields([]rune(header))
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}
	nums := make([]int, 5)
	for k := 0; k < 5; k++ {
		if nums[k], err = pyInt(fields[k+1]); err != nil {
			panic(err)
		}
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	f := &Font{name: name, height: height, hardBlank: fields[0],
		chars: map[int][][]rune{}, width: map[int]int{}}
	fullLayout := -1
	haveFull := false
	if len(fields) > 6 {
		d, err := pyInt(fields[6])
		if err != nil {
			panic(err)
		}
		f.printDirection = &d
	}
	if len(fields) > 7 {
		if fullLayout, err = pyInt(fields[7]); err != nil {
			panic(err)
		}
		haveFull = true
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

	for k := 0; k < commentLines; k++ {
		pop()
	}

	loadChar := func() (int, [][]rune) {
		var end *regexp.Regexp
		width := 0
		var rows [][]rune
		for k := 0; k < height; k++ {
			line := string(pop())
			if end == nil {
				m := reEndMarker.FindStringSubmatch(line)
				if m == nil {
					panic("no end marker")
				}
				end = regexp.MustCompile(regexp.QuoteMeta(m[1]) + `{1,2}` + pyWS + `*$`)
			}
			r := []rune(end.ReplaceAllString(line, ""))
			if len(r) > width {
				width = len(r)
			}
			rows = append(rows, r)
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
		w, letter := loadChar()
		if i == 32 || nonEmpty(letter) {
			f.chars[i] = letter
			f.width[i] = w
		}
	}
	if len(data) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, letter := loadChar()
			if nonEmpty(letter) {
				f.chars[int(c)] = letter
				f.width[int(c)] = w
			}
		}
	}
	for len(data) > 0 {
		line := pyStrip(pop())
		tok := strings.SplitN(string(line), " ", 2)[0]
		if tok == "" {
			continue
		}
		if len(tok) >= 2 && strings.EqualFold(tok[:2], "0x") {
			code, err := strconv.ParseInt(tok, 0, 64)
			if err != nil {
				code, err = strconv.ParseInt("0x"+tok[2:], 0, 64)
				if err != nil {
					panic(err)
				}
			}
			w, letter := loadChar()
			if nonEmpty(letter) {
				f.chars[int(code)] = letter
				f.width[int(code)] = w
			}
		}
	}
	return f, nil
}
GO_SOURCE_EOF

cat > render.go <<'GO_SOURCE_EOF'
package main

// Port of pyfiglet.FigletBuilder / FigletRenderingEngine / FigletString.
// The structure deliberately mirrors the Python, method by method, so that every
// quirk (including the ones that look like bugs) is reproduced.

import (
	"errors"
	"strings"
)

const (
	smEqual     = 1   // smush equal chars (not hardblanks)
	smLowline   = 2   // smush _ with any char in hierarchy
	smHierarchy = 4   // hierarchy: |, /\, [], {}, (), <>
	smPair      = 8   // hierarchy: [ + ] -> |, { + } -> |, ( + ) -> |
	smBigX      = 16  // / + \ -> |, \ + / -> Y, > + < -> X  (pyfiglet's version)
	smHardblank = 32  // hardblank + hardblank -> hardblank
	smKern      = 64  // kerning
	smSmush     = 128 // smushing
)

// noChar stands for Python's empty string ” where a character is expected.
const noChar rune = -1

var errCharNotPrinted = errors.New("Width is not enough to print this character")

type marker struct {
	buffer   [][]rune
	iterator int
}

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
	queue        [][][]rune
	buffer       [][]rune
}

func newBuilder(text []rune, font *Font, direction string, width int, justify string) *builder {
	b := &builder{text: text, direction: direction, width: width, font: font, justify: justify}
	b.buffer = emptyBuffer(font.height)
	return b
}

func emptyBuffer(h int) [][]rune {
	buf := make([][]rune, h)
	for i := range buf {
		buf[i] = []rune{}
	}
	return buf
}

func copyBuffer(buf [][]rune) [][]rune {
	out := make([][]rune, len(buf))
	copy(out, buf) // rows are never modified in place, so a shallow copy is a real copy
	return out
}

func concat(a, b []rune) []rune {
	out := make([]rune, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}

// ---- builder interface ----

func (b *builder) addCharToProduct() error {
	curChar := b.getCurChar()

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}

	if curChar == nil {
		return nil
	}
	w, _ := b.getCurWidth()
	if b.width < w {
		return errCharNotPrinted
	}
	b.curCharWidth = w
	b.maxSmush = b.currentSmushAmount(curChar)

	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, marker{copyBuffer(b.buffer), b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}

	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) goToNextChar() { b.iterator++ }

func (b *builder) isNotFinished() bool { return b.iterator < len(b.text) }

func (b *builder) returnProduct() []rune {
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	return b.formatProduct()
}

// ---- private ----

func (b *builder) formatProduct() []rune {
	var sb strings.Builder
	for _, buf := range b.queue {
		buf = b.justifyString(b.justify, copyBuffer(buf))
		sb.WriteString(b.replaceHardblanks(buf))
	}
	return []rune(sb.String())
}

func (b *builder) getCharAt(i int) [][]rune {
	if i < 0 || i >= len(b.text) {
		return nil
	}
	c, ok := b.font.chars[int(b.text[i])]
	if !ok {
		return nil
	}
	return c
}

func (b *builder) getCharWidthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	w, ok := b.font.width[int(b.text[i])]
	return w, ok
}

func (b *builder) getCurChar() [][]rune     { return b.getCharAt(b.iterator) }
func (b *builder) getCurWidth() (int, bool) { return b.getCharWidthAt(b.iterator) }

func (b *builder) getLeftSmushedChar(i int, addLeft []rune) (rune, int) {
	idx := len(addLeft) - b.maxSmush + i
	if idx >= 0 && idx < len(addLeft) {
		return addLeft[idx], idx
	}
	return noChar, idx
}

func (b *builder) currentSmushAmount(curChar [][]rune) int {
	return b.smushAmount(b.buffer, curChar)
}

func (b *builder) updateSmushedCharInLeftBuffer(addLeft []rune, idx int, smushed rune, ok bool) []rune {
	if idx < 0 || idx > len(addLeft) {
		return addLeft
	}
	if !ok {
		// Python stores None and then ''.join() raises TypeError.
		panic("sequence item: expected str instance, NoneType found")
	}
	l := make([]rune, len(addLeft))
	copy(l, addLeft)
	if smushed == noChar { // l[idx] = '' removes the character when joined
		return append(l[:idx], l[idx+1:]...)
	}
	l[idx] = smushed
	return l
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune) {
	addLeft := b.buffer[row]
	addRight := curChar[row]

	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for i := 0; i < b.maxSmush; i++ {
		left, idx := b.getLeftSmushedChar(i, addLeft)
		right := addRight[i] // IndexError in Python, panic here
		smushed, ok := b.smushChars(left, right)
		addLeft = b.updateSmushedCharInLeftBuffer(addLeft, idx, smushed, ok)
	}
	return addLeft, addRight
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	b.buffer[row] = concat(addLeft, addRight[b.maxSmush:])
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = emptyBuffer(b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar := b.getCurChar()
	if curChar == nil {
		return
	}
	b.maxSmush = b.currentSmushAmount(curChar)
}

func (b *builder) cutBufferAtLastBlank(saved [][]rune, savedIterator int) {
	b.queue = append(b.queue, saved)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *builder) cutBufferAtLastChar() {
	b.queue = append(b.queue, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.cutBufferAtLastBlank(m.buffer, m.iterator)
	} else {
		b.cutBufferAtLastChar()
	}
}

func spaces(n int) []rune {
	if n <= 0 {
		return nil
	}
	return []rune(strings.Repeat(" ", n))
}

func (b *builder) justifyString(justify string, buf [][]rune) [][]rune {
	switch justify {
	case "right":
		for row := 0; row < b.font.height; row++ {
			buf[row] = concat(spaces(b.width-len(buf[row])-1), buf[row])
		}
	case "center":
		for row := 0; row < b.font.height; row++ {
			// int((w - len) / 2): true division then truncation toward zero
			buf[row] = concat(spaces((b.width-len(buf[row]))/2), buf[row])
		}
	}
	return buf
}

func (b *builder) replaceHardblanks(buf [][]rune) string {
	parts := make([]string, len(buf))
	for i, r := range buf {
		parts[i] = string(r)
	}
	s := strings.Join(parts, "\n") + "\n"
	if b.font.hardBlank == "" {
		return s
	}
	return strings.ReplaceAll(s, b.font.hardBlank, " ")
}

func (b *builder) isHardBlank(r rune) bool {
	return r != noChar && string(r) == b.font.hardBlank
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

		linebd := len(rstripSpace(lineLeft)) - 1
		if linebd < 0 {
			linebd = 0
		}

		ch1 := noChar
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}

		charbd := len(lineRight) - len(lstripSpace(lineRight))
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

// inClass reproduces Python's `c in "chars"`, where ” is in every string.
func inClass(c rune, class string) bool {
	if c == noChar {
		return true
	}
	return strings.ContainsRune(class, c)
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
		if b.direction == "right-to-left" {
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
		if inClass(left, p.a) && inClass(right, p.b) {
			return right, true
		}
		if inClass(right, p.a) && inClass(left, p.b) {
			return left, true
		}
	}

	if mode&smPair != 0 && left != noChar && right != noChar {
		lr := string([]rune{left, right})
		rl := string([]rune{right, left})
		for _, s := range []string{lr, rl} {
			if s == "[]" || s == "{}" || s == "()" {
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

// ---- Figlet ----

func resolveDirection(opt string, f *Font) string {
	if opt != "auto" {
		return opt
	}
	if f.printDirection != nil && *f.printDirection == 1 {
		return "right-to-left"
	}
	return "left-to-right"
}

func resolveJustify(opt, direction string) string {
	if opt != "auto" {
		return opt
	}
	if direction == "right-to-left" {
		return "right"
	}
	return "left"
}

func renderText(text []rune, f *Font, direction, justify string, width int) ([]rune, error) {
	dir := resolveDirection(direction, f)
	b := newBuilder(text, f, dir, width, resolveJustify(justify, dir))
	for b.isNotFinished() {
		if err := b.addCharToProduct(); err != nil {
			return nil, err
		}
		b.goToNextChar()
	}
	return b.returnProduct(), nil
}

// ---- FigletString ----

var reverseMap = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '<': '>', '>': '<',
	'[': ']', '\\': '/', ']': '[', '{': '}', '}': '{',
}

var flipMap = map[rune]rune{
	'/': '\\', 'A': 'V', 'M': 'W', 'P': 'b', 'R': 'b', 'V': 'A', 'W': 'M',
	'\\': '/', '^': 'v', '_': '-', 'b': 'P', 'm': 'w', 'v': '^', 'w': 'm',
}

func translate(row []rune, m map[rune]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if t, ok := m[r]; ok {
			out[i] = t
		} else {
			out[i] = r
		}
	}
	return out
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

func fsReverse(s []rune) []rune {
	var rows [][]rune
	for _, row := range pySplitlines(s) {
		t := translate(row, reverseMap)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		rows = append(rows, t)
	}
	return newFromList(rows)
}

func fsFlip(s []rune) []rune {
	lines := pySplitlines(s)
	var rows [][]rune
	for i := len(lines) - 1; i >= 0; i-- {
		rows = append(rows, translate(lines[i], flipMap))
	}
	return newFromList(rows)
}

func fsStripSurroundingNewlines(s []rune) []rune {
	var rows [][]rune
	seen := false
	for _, row := range pySplitlines(s) {
		if len(pyStrip(row)) != 0 || seen {
			seen = true
			rows = append(rows, row)
		}
	}
	return pyRStrip(newFromList(rows))
}

func fsNormalizeSurroundingNewlines(s []rune) []rune {
	out := []rune{'\n'}
	out = append(out, fsStripSurroundingNewlines(s)...)
	return append(out, '\n')
}
GO_SOURCE_EOF

cat > pystr.go <<'GO_SOURCE_EOF'
package main

// Small helpers that reproduce the exact Python str behaviour pyfiglet relies on.

import (
	"unicode/utf8"
)

// pyIsSpace matches Python's str.isspace() (also what `\s` means in a str regex).
func pyIsSpace(r rune) bool {
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

// pyIsLineBreak matches the boundaries used by Python's str.splitlines().
func pyIsLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', 0x0b, 0x0c, 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pySplitlines reproduces str.splitlines() (keepends=False).
func pySplitlines(s []rune) [][]rune {
	var out [][]rune
	start := 0
	for i := 0; i < len(s); i++ {
		if !pyIsLineBreak(s[i]) {
			continue
		}
		out = append(out, s[start:i])
		if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			i++
		}
		start = i + 1
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// pyFields reproduces str.split() with no argument.
func pyFields(s []rune) []string {
	var out []string
	start := -1
	for i, r := range s {
		if pyIsSpace(r) {
			if start >= 0 {
				out = append(out, string(s[start:i]))
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, string(s[start:]))
	}
	return out
}

func pyStrip(s []rune) []rune {
	return pyLStrip(pyRStrip(s))
}

func pyRStrip(s []rune) []rune {
	e := len(s)
	for e > 0 && pyIsSpace(s[e-1]) {
		e--
	}
	return s[:e]
}

func pyLStrip(s []rune) []rune {
	b := 0
	for b < len(s) && pyIsSpace(s[b]) {
		b++
	}
	return s[b:]
}

// rstripSpace / lstripSpace strip only ASCII ' ' (pyfiglet's rstrip(' ')).
func rstripSpace(s []rune) []rune {
	e := len(s)
	for e > 0 && s[e-1] == ' ' {
		e--
	}
	return s[:e]
}

func lstripSpace(s []rune) []rune {
	b := 0
	for b < len(s) && s[b] == ' ' {
		b++
	}
	return s[b:]
}

// decodeUTF8Replace reproduces bytes.decode('utf-8', 'replace'): each maximal
// ill-formed subsequence becomes one U+FFFD (Go's []rune() would emit one per byte).
func decodeUTF8Replace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r != utf8.RuneError || size > 1 {
			out = append(out, r)
			i += size
			continue
		}
		out = append(out, utf8.RuneError)
		i += maximalSubpart(b[i:])
	}
	return out
}

// maximalSubpart returns the length of the maximal prefix of b that is the
// start of some well-formed sequence (at least 1).
func maximalSubpart(b []byte) int {
	c := b[0]
	var need int
	lo, hi := byte(0x80), byte(0xbf)
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
		return 1
	}
	n := 1
	for k := 0; k < need && n < len(b); k++ {
		x := b[n]
		if k == 0 {
			if x < lo || x > hi {
				return n
			}
		} else if x < 0x80 || x > 0xbf {
			return n
		}
		n++
	}
	return n
}
GO_SOURCE_EOF

find . -name '*.py' -type f -delete
find . -name '__pycache__' -type d -prune -exec rm -rf {} +

go build -o "$(mktemp -d)/" ./...   # checks every package without leaving a binary in the repo
go build -o /tmp/figlet .
echo "figlet port built: $(ls fonts | wc -l) fonts"
