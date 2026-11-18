#!/bin/bash
set -euo pipefail

repo=/workspace/repo
cd "$repo"

# ---------------------------------------------------------------------------
# 1. Collect the FIGlet fonts inside the repository and embed them.
# ---------------------------------------------------------------------------
mkdir -p fonts
for src in pyfiglet/fonts/*.flf pyfiglet/fonts-standard/*.flf; do
    [ -f "$src" ] || continue
    cp -f "$src" fonts/
done

found=
for dst in fonts/*.flf; do
    [ -f "$dst" ] && found=1 && break
done
if [ -z "$found" ]; then
    echo "solve.sh: no .flf fonts found in the repository" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# 2. Go module and the standard-library-only Go port of pyfiglet.
# ---------------------------------------------------------------------------
cat > go.mod <<'GOMOD'
module figlet

go 1.23
GOMOD

cat > main.go <<'GOEOF'
package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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

	defaultFont  = "standard"
	defaultWidth = 80
)

var (
	errCharNotPrinted = errors.New("width is not enough to print this character")
	errFontNotFound   = errors.New("font not found")
)

// ---------------------------------------------------------------------------
// Options and main
// ---------------------------------------------------------------------------

type options struct {
	font      string
	direction string
	justify   string
	width     int
	reverse   bool
	flip      bool
	strip     bool
	normalize bool
	text      string
	listFonts bool
	infoFont  bool
	load      string
	color     string
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if opts.listFonts {
		entries, err := fs.ReadDir(fontFS, "fonts")
		if err == nil {
			var names []string
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				name = strings.TrimSuffix(name, ".flf")
				name = strings.TrimSuffix(name, ".tlf")
				names = append(names, name)
			}
			sortStrings(names)
			fmt.Println(strings.Join(names, "\n"))
		}
		return
	}

	if opts.infoFont {
		return
	}
	if opts.load != "" {
		return
	}

	fig, err := newFiglet(opts.font, opts.direction, opts.justify, opts.width)
	if err != nil {
		if errors.Is(err, errFontNotFound) {
			fmt.Printf("pyfiglet error: requested font %q not found.\n", opts.font)
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rendered, err := fig.renderText(opts.text)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if opts.reverse {
		rendered = rendered.reverse()
	}
	if opts.flip {
		rendered = rendered.flip()
	}

	var out string
	if opts.strip {
		out = rendered.stripSurroundingNewlines()
	} else if opts.normalize {
		out = reconstructNormalized(rendered)
	} else {
		out = string(rendered)
	}

	// pyfiglet writes the rendered bytes and then one extra newline.
	os.Stdout.WriteString(out)
	os.Stdout.WriteString("\n")
}

// reconstructNormalized mirrors pyfiglet's normalize_surrounding_newlines().
func reconstructNormalized(s figletString) string {
	return "\n" + s.stripSurroundingNewlines() + "\n"
}

func parseArgs(args []string) (*options, error) {
	o := &options{
		font:      defaultFont,
		direction: "auto",
		justify:   "auto",
		width:     defaultWidth,
		color:     ":",
	}

	var positional []string

	needValue := func(i *int, inline string, hasInline bool, name string) (string, error) {
		if hasInline {
			return inline, nil
		}
		*i++
		if *i >= len(args) {
			return "", fmt.Errorf("option %s requires an argument", name)
		}
		return args[*i], nil
	}

	for i := 0; i < len(args); i++ {
		a := args[i]

		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}

		if strings.HasPrefix(a, "--") {
			name, inline, hasInline := strings.Cut(a[2:], "=")
			switch name {
			case "font":
				v, err := needValue(&i, inline, hasInline, "--font")
				if err != nil {
					return nil, err
				}
				o.font = v
			case "width":
				v, err := needValue(&i, inline, hasInline, "--width")
				if err != nil {
					return nil, err
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("option --width: invalid integer value: %q", v)
				}
				o.width = n
			case "justify":
				v, err := needValue(&i, inline, hasInline, "--justify")
				if err != nil {
					return nil, err
				}
				o.justify = v
			case "direction":
				v, err := needValue(&i, inline, hasInline, "--direction")
				if err != nil {
					return nil, err
				}
				o.direction = v
			case "reverse":
				o.reverse = true
			case "flip":
				o.flip = true
			case "strip-surrounding-newlines":
				o.strip = true
			case "normalize-surrounding-newlines":
				o.normalize = true
			case "list_fonts":
				o.listFonts = true
			case "info_font":
				o.infoFont = true
			case "load":
				v, err := needValue(&i, inline, hasInline, "--load")
				if err != nil {
					return nil, err
				}
				o.load = v
			case "color":
				v, err := needValue(&i, inline, hasInline, "--color")
				if err != nil {
					return nil, err
				}
				o.color = v
			default:
				return nil, fmt.Errorf("no such option: --%s", name)
			}
			continue
		}

		if len(a) > 1 && a[0] == '-' {
			for j := 1; j < len(a); j++ {
				c := a[j]
				switch c {
				case 'r':
					o.reverse = true
				case 'F':
					o.flip = true
				case 's':
					o.strip = true
				case 'n':
					o.normalize = true
				case 'l':
					o.listFonts = true
				case 'i':
					o.infoFont = true
				case 'f', 'w', 'j', 'D', 'L', 'c':
					var v string
					if j+1 < len(a) {
						v = a[j+1:]
					} else {
						i++
						if i >= len(args) {
							return nil, fmt.Errorf("option -%c requires an argument", c)
						}
						v = args[i]
					}
					switch c {
					case 'f':
						o.font = v
					case 'w':
						n, err := strconv.Atoi(v)
						if err != nil {
							return nil, fmt.Errorf("option -w: invalid integer value: %q", v)
						}
						o.width = n
					case 'j':
						o.justify = v
					case 'D':
						o.direction = v
					case 'L':
						o.load = v
					case 'c':
						o.color = v
					}
					j = len(a)
				default:
					return nil, fmt.Errorf("no such option: -%c", c)
				}
			}
			continue
		}

		positional = append(positional, a)
	}

	switch o.justify {
	case "auto", "left", "center", "right":
	default:
		return nil, fmt.Errorf("option -j: invalid choice: %q", o.justify)
	}
	switch o.direction {
	case "auto", "left-to-right", "right-to-left":
	default:
		return nil, fmt.Errorf("option -D: invalid choice: %q", o.direction)
	}

	o.text = strings.Join(positional, " ")
	return o, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ---------------------------------------------------------------------------
// Python-compatible string helpers
// ---------------------------------------------------------------------------

// isPySpace mirrors Python's definition of whitespace, which differs slightly
// from Go's unicode.IsSpace for the C0 separators 0x1c-0x1f.
func isPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		'\x1c', '\x1d', '\x1e', '\x1f', '\x85', '\xa0':
		return true
	}
	return unicode.IsSpace(r)
}

// pySplitLines mirrors Python str.splitlines().
func pySplitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\n' || r == '\r' || r == '\v' || r == '\f' ||
			r == '\x1c' || r == '\x1d' || r == '\x1e' ||
			r == '\x85' || r == '\u2028' || r == '\u2029' {
			lines = append(lines, s[start:i])
			if r == '\r' && i+size < len(s) && s[i+size] == '\n' {
				i += size + 1
			} else {
				i += size
			}
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func runeAt(s string, idx int) string {
	if idx < 0 {
		return ""
	}
	i := 0
	for _, r := range s {
		if i == idx {
			return string(r)
		}
		i++
	}
	return ""
}

func runeStrings(s string) []string {
	out := make([]string, 0, utf8.RuneCountInString(s))
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
}

func dropRunes(s string, n int) string {
	if n <= 0 {
		return s
	}
	i := 0
	for pos := range s {
		if i == n {
			return s[pos:]
		}
		i++
	}
	return ""
}

func pyTrimSpace(s string) string {
	return strings.TrimFunc(s, isPySpace)
}

func pyTrimRightSpace(s string) string {
	return strings.TrimRightFunc(s, isPySpace)
}

// ---------------------------------------------------------------------------
// Font loading
// ---------------------------------------------------------------------------

type figletFont struct {
	name           string
	comment        string
	height         int
	hardBlank      string
	printDirection int
	smushMode      int
	chars          map[rune][]string
	width          map[rune]int
}

func loadFont(name string) (*figletFont, error) {
	var data []byte
	var err error
	found := false
	for _, ext := range []string{"tlf", "flf"} {
		data, err = fs.ReadFile(fontFS, "fonts/"+name+"."+ext)
		if err == nil {
			found = true
			break
		}
	}
	if !found {
		return nil, errFontNotFound
	}

	text := strings.ToValidUTF8(string(data), "\uFFFD")
	// Match pyfiglet's removal of unicode line separators before splitlines.
	text = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(text)

	lines := pySplitLines(text)
	font := &figletFont{
		name:  name,
		chars: make(map[rune][]string),
		width: make(map[rune]int),
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("problem parsing %s font: empty file", name)
	}

	header := lines[0]
	lines = lines[1:]
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || header[1:4] != "lf2" {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}
	header = header[5:]

	fields := strings.Fields(header)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}

	hardBlank := fields[0]
	height, err1 := strconv.Atoi(fields[1])
	baseLine, err2 := strconv.Atoi(fields[2])
	maxLength, err3 := strconv.Atoi(fields[3])
	oldLayout, err4 := strconv.Atoi(fields[4])
	commentLines, err5 := strconv.Atoi(fields[5])
	if err := firstError(err1, err2, err3, err4, err5); err != nil {
		return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
	}
	_ = baseLine
	_ = maxLength

	printDirection := 0
	fullLayout := 0
	haveFullLayout := false
	if len(fields) > 6 {
		v, err := strconv.Atoi(fields[6])
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
		}
		printDirection = v
	}
	if len(fields) > 7 {
		v, err := strconv.Atoi(fields[7])
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
		}
		fullLayout = v
		haveFullLayout = true
	}
	if !haveFullLayout {
		if oldLayout == 0 {
			fullLayout = smKern
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | smSmush
		}
	}

	font.height = height
	font.hardBlank = hardBlank
	font.printDirection = printDirection
	font.smushMode = fullLayout

	for i := 0; i < commentLines; i++ {
		if len(lines) == 0 {
			return nil, fmt.Errorf("problem parsing %s font: missing comment line", name)
		}
		font.comment += lines[0]
		lines = lines[1:]
	}

	endMarker := ""
	readChar := func() (int, []string, error) {
		width := 0
		rows := make([]string, 0, height)
		for row := 0; row < height; row++ {
			if len(lines) == 0 {
				return 0, nil, fmt.Errorf("problem parsing %s font: unexpected end of file", name)
			}
			line := lines[0]
			lines = lines[1:]

			if endMarker == "" {
				trimmed := pyTrimRightSpace(line)
				if trimmed == "" {
					if line == "" {
						return 0, nil, fmt.Errorf("problem parsing %s font: empty glyph row", name)
					}
					endMarker = string([]rune(line)[runeLen(line)-1])
				} else {
					endMarker = string([]rune(trimmed)[runeLen(trimmed)-1])
				}
			}

			trimmed := pyTrimRightSpace(line)
			remove := 0
			if strings.HasSuffix(trimmed, endMarker) {
				remove = 1
				if len(trimmed) >= 2*len(endMarker) && strings.HasSuffix(trimmed[:len(trimmed)-len(endMarker)], endMarker) {
					remove = 2
				}
			}
			if remove > 0 {
				line = trimmed[:len(trimmed)-remove*len(endMarker)]
			}
			// If there was no end marker we leave the line untouched, as the
			// regex substitution in pyfiglet would.

			widthHere := runeLen(line)
			if widthHere > width {
				width = widthHere
			}
			rows = append(rows, line)
		}
		return width, rows, nil
	}

	for c := rune(32); c < 127; c++ {
		w, rows, err := readChar()
		if err != nil {
			return nil, err
		}
		if c == 32 || strings.Join(rows, "") != "" {
			font.chars[c] = rows
			font.width[c] = w
		}
	}

	if len(lines) > 0 {
		for _, c := range []rune("ÄÖÜäöüß") {
			w, rows, err := readChar()
			if err != nil {
				return nil, err
			}
			if strings.Join(rows, "") != "" {
				font.chars[c] = rows
				font.width[c] = w
			}
		}
	}

	for len(lines) > 0 {
		line := pyTrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		first := strings.SplitN(line, " ", 2)[0]
		if strings.HasPrefix(strings.ToLower(first), "0x") {
			value, err := strconv.ParseInt(first, 0, 64)
			if err != nil {
				return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
			}
			w, rows, err := readChar()
			if err != nil {
				return nil, err
			}
			if strings.Join(rows, "") != "" {
				font.chars[rune(value)] = rows
				font.width[rune(value)] = w
			}
		}
	}

	return font, nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// FigletString
// ---------------------------------------------------------------------------

type figletString string

func newFromList(rows []string) figletString {
	return figletString(strings.Join(rows, "\n") + "\n")
}

// reverseRune mirrors pyfiglet's __reverse_map__.
func reverseRune(r rune) rune {
	switch r {
	case '(':
		return ')'
	case ')':
		return '('
	case '/':
		return '\\'
	case '\\':
		return '/'
	case '<':
		return '>'
	case '>':
		return '<'
	case '[':
		return ']'
	case ']':
		return '['
	case '{':
		return '}'
	case '}':
		return '{'
	}
	return r
}

// flipRune mirrors pyfiglet's __flip_map__.
func flipRune(r rune) rune {
	const flipTable = " !\"#$%&'()*+,-.\\" +
		"0123456789:;<=>?" +
		"@VBCDEFGHIJKLWNO" +
		"bQbSTUAMXYZ[/]v-" +
		"`aPcdefghijklwno" +
		"pqrstu^mxyz{|}~\x7f"
	if r < 0x20 || r >= 0x80 {
		return r
	}
	idx := int(r) - 0x20
	if idx < 0 || idx >= len(flipTable) {
		return r
	}
	return rune(flipTable[idx])
}

func (s figletString) reverse() figletString {
	rows := pySplitLines(string(s))
	out := make([]string, len(rows))
	for i, row := range rows {
		runes := []rune(row)
		for j := range runes {
			runes[j] = reverseRune(runes[j])
		}
		for l, h := 0, len(runes)-1; l < h; l, h = l+1, h-1 {
			runes[l], runes[h] = runes[h], runes[l]
		}
		out[i] = string(runes)
	}
	return newFromList(out)
}

func (s figletString) flip() figletString {
	rows := pySplitLines(string(s))
	out := make([]string, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		runes := []rune(rows[i])
		for j := range runes {
			runes[j] = flipRune(runes[j])
		}
		out = append(out, string(runes))
	}
	return newFromList(out)
}

func (s figletString) stripSurroundingNewlines() string {
	rows := pySplitLines(string(s))
	var out []string
	charsSeen := false
	for _, row := range rows {
		if pyTrimSpace(row) != "" || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}
	return pyTrimRightSpace(string(newFromList(out)))
}

// ---------------------------------------------------------------------------
// Figlet
// ---------------------------------------------------------------------------

type figlet struct {
	font      *figletFont
	direction string
	justify   string
	width     int
}

func newFiglet(fontName, directionOpt, justifyOpt string, width int) (*figlet, error) {
	font, err := loadFont(fontName)
	if err != nil {
		return nil, err
	}

	direction := directionOpt
	if direction == "auto" {
		if font.printDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}

	justify := justifyOpt
	if justify == "auto" {
		if direction == "left-to-right" {
			justify = "left"
		} else if direction == "right-to-left" {
			justify = "right"
		}
	}

	return &figlet{
		font:      font,
		direction: direction,
		justify:   justify,
		width:     width,
	}, nil
}

func (f *figlet) renderText(text string) (figletString, error) {
	b := &figletBuilder{
		text:         []rune(text),
		font:         f.font,
		direction:    f.direction,
		width:        f.width,
		justify:      f.justify,
		buffer:       make([]string, f.font.height),
		blankMarkers: nil,
	}

	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
		b.iterator++
	}
	return b.returnProduct()
}

// ---------------------------------------------------------------------------
// FigletBuilder
// ---------------------------------------------------------------------------

type blankMarker struct {
	rows     []string
	iterator int
}

type figletBuilder struct {
	text              []rune
	font              *figletFont
	direction         string
	width             int
	justify           string
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []blankMarker
	product           [][]string
	buffer            []string
}

func (b *figletBuilder) getCharAt(i int) ([]string, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	rows, ok := b.font.chars[b.text[i]]
	return rows, ok
}

func (b *figletBuilder) getCharWidthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	w, ok := b.font.width[b.text[i]]
	return w, ok
}

func (b *figletBuilder) addCharToProduct() error {
	curChar, curOK := b.getCharAt(b.iterator)

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{rows: append([]string(nil), b.buffer...), iterator: b.iterator})
		return b.handleNewLine()
	}

	if !curOK {
		return nil
	}

	curWidth, _ := b.getCharWidthAt(b.iterator)
	if b.width < curWidth {
		return errCharNotPrinted
	}
	b.curCharWidth = curWidth
	b.maxSmush = b.currentSmushAmount(curChar)
	b.currentTotalWidth = runeLen(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{rows: append([]string(nil), b.buffer...), iterator: b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		if err := b.handleNewLine(); err != nil {
			return err
		}
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

func (b *figletBuilder) returnProduct() (figletString, error) {
	if len(b.buffer) > 0 && b.buffer[0] != "" {
		b.product = append(b.product, b.buffer)
	}
	return b.formatProduct()
}

func (b *figletBuilder) formatProduct() (figletString, error) {
	var sb strings.Builder
	for _, buf := range b.product {
		b.justifyString(buf)
		sb.WriteString(b.replaceHardblanks(buf))
	}
	return figletString(sb.String()), nil
}

func (b *figletBuilder) justifyString(buffer []string) {
	switch b.justify {
	case "right":
		for row := range buffer {
			pad := b.width - runeLen(buffer[row]) - 1
			if pad < 0 {
				pad = 0
			}
			buffer[row] = strings.Repeat(" ", pad) + buffer[row]
		}
	case "center":
		for row := range buffer {
			diff := b.width - runeLen(buffer[row])
			pad := int(float64(diff) / 2.0)
			if pad < 0 {
				pad = 0
			}
			buffer[row] = strings.Repeat(" ", pad) + buffer[row]
		}
	}
}

func (b *figletBuilder) replaceHardblanks(buffer []string) string {
	string := strings.Join(buffer, "\n") + "\n"
	return strings.ReplaceAll(string, b.font.hardBlank, " ")
}

func (b *figletBuilder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, ok := b.getCharAt(b.iterator)
	if !ok {
		return
	}
	b.maxSmush = b.currentSmushAmount(curChar)
}

func (b *figletBuilder) cutBufferAtLastBlank(savedBuffer []string, savedIterator int) {
	b.product = append(b.product, savedBuffer)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *figletBuilder) cutBufferAtLastChar() {
	b.product = append(b.product, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *figletBuilder) handleNewLine() error {
	savedBuffer, savedIterator, ok := b.getLastBlank()
	if ok {
		b.cutBufferAtLastBlank(savedBuffer, savedIterator)
	} else {
		b.cutBufferAtLastChar()
	}
	return nil
}

func (b *figletBuilder) getLastBlank() ([]string, int, bool) {
	if len(b.blankMarkers) == 0 {
		return nil, -1, false
	}
	marker := b.blankMarkers[len(b.blankMarkers)-1]
	b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
	return marker.rows, marker.iterator, true
}

func (b *figletBuilder) currentSmushAmount(curChar []string) int {
	return b.smushAmount(b.buffer, curChar)
}

func (b *figletBuilder) smushAmount(buffer, curChar []string) int {
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

		linebd := runeLen(strings.TrimRight(lineLeft, " ")) - 1
		if linebd < 0 {
			linebd = 0
		}

		ch1 := ""
		if linebd < runeLen(lineLeft) {
			ch1 = runeAt(lineLeft, linebd)
		} else {
			linebd = 0
			ch1 = ""
		}

		charbd := runeLen(lineRight) - runeLen(strings.TrimLeft(lineRight, " "))
		ch2 := ""
		if charbd < runeLen(lineRight) {
			ch2 = runeAt(lineRight, charbd)
		} else {
			charbd = runeLen(lineRight)
			ch2 = ""
		}

		amt := charbd + runeLen(lineLeft) - 1 - linebd
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

func (b *figletBuilder) smushChars(left, right string) (string, bool) {
	if left == " " {
		return right, true
	}
	if right == " " {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return "", false
	}
	if b.font.smushMode&smSmush == 0 {
		return "", false
	}
	if b.font.smushMode&63 == 0 {
		if left == b.font.hardBlank {
			return right, true
		}
		if right == b.font.hardBlank {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if b.font.smushMode&smHardblank != 0 {
		if left == b.font.hardBlank && right == b.font.hardBlank {
			return left, true
		}
	}
	if left == b.font.hardBlank || right == b.font.hardBlank {
		return "", false
	}
	if b.font.smushMode&smEqual != 0 {
		if left == right {
			return left, true
		}
	}

	type smushPair struct{ a, b string }
	var smushes []smushPair
	if b.font.smushMode&smLowline != 0 {
		smushes = append(smushes, smushPair{"_", "|/\\[]{}()<>"})
	}
	if b.font.smushMode&smHierarchy != 0 {
		smushes = append(smushes,
			smushPair{"|", "/\\[]{}()<>"},
			smushPair{"\\/", "[]{}()<>"},
			smushPair{"[]", "{}()<>"},
			smushPair{"{}", "()<>"},
			smushPair{"()", "<>"},
		)
	}

	for _, pair := range smushes {
		if strings.Contains(pair.a, left) && strings.Contains(pair.b, right) {
			return right, true
		}
		if strings.Contains(pair.a, right) && strings.Contains(pair.b, left) {
			return left, true
		}
	}

	if b.font.smushMode&smPair != 0 {
		for _, pair := range []string{left + right, right + left} {
			if pair == "[]" || pair == "{}" || pair == "()" {
				return "|", true
			}
		}
	}

	if b.font.smushMode&smBigX != 0 {
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

func (b *figletBuilder) getLeftSmushedChar(i int, addLeft string) (string, int) {
	idx := runeLen(addLeft) - b.maxSmush + i
	if idx >= 0 && idx < runeLen(addLeft) {
		return runeAt(addLeft, idx), idx
	}
	return "", idx
}

func (b *figletBuilder) updateSmushedCharInLeftBuffer(addLeft string, idx int, smushed string) string {
	runes := runeStrings(addLeft)
	if idx < 0 || idx > len(runes) {
		return addLeft
	}
	if idx == len(runes) {
		return addLeft
	}
	runes[idx] = smushed
	return strings.Join(runes, "")
}

func (b *figletBuilder) smushRow(curChar []string, row int) (string, string, error) {
	addLeft := b.buffer[row]
	addRight := curChar[row]

	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for i := 0; i < b.maxSmush; i++ {
		if i >= runeLen(addRight) {
			return "", "", fmt.Errorf("smush index out of range")
		}
		left, idx := b.getLeftSmushedChar(i, addLeft)
		right := runeAt(addRight, i)
		smushed, ok := b.smushChars(left, right)
		if !ok {
			return "", "", fmt.Errorf("cannot smush %q with %q", left, right)
		}
		addLeft = b.updateSmushedCharInLeftBuffer(addLeft, idx, smushed)
	}
	return addLeft, addRight, nil
}

func (b *figletBuilder) addCurCharRowToBufferRow(curChar []string, row int) error {
	addLeft, addRight, err := b.smushRow(curChar, row)
	if err != nil {
		return err
	}
	b.buffer[row] = addLeft + dropRunes(addRight, b.maxSmush)
	return nil
}
GOEOF

# ---------------------------------------------------------------------------
# 3. Remove every Python source file from the repository.
# ---------------------------------------------------------------------------
find "$repo" -type f -name '*.py' -delete
find "$repo" -type d -name '__pycache__' -prune -exec rm -rf {} + 2>/dev/null || true

# ---------------------------------------------------------------------------
# 4. Smoke-test the build exactly the way the verifier does.
# ---------------------------------------------------------------------------
go build -o /tmp/figlet .
