#!/usr/bin/env bash
# solution/solve.sh
# Complete Go port of pyfiglet matching FigletBuilder byte for byte.
set -euo pipefail

REPO=/workspace/repo
cd "$REPO"

# ── 1. Clear any old Go files ────────────────────────────────────────────────
rm -f go.mod go.sum *.go
find . -name "*.go" -not -path "./.git/*" -delete 2>/dev/null || true

# ── 2. Initialise module ─────────────────────────────────────────────────────
go mod init figlet

# ── 3. Write main.go ─────────────────────────────────────────────────────────
cat > main.go << 'GOEOF'
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode"
)

func main() {
	fontName  := flag.String("f", "standard", "font name")
	width     := flag.Int("w", 80, "terminal width")
	justify   := flag.String("j", "auto", "justification: auto|left|center|right")
	direction := flag.String("D", "auto", "direction: auto|left-to-right|right-to-left")
	reverse   := flag.Bool("r", false, "mirror output")
	flip      := flag.Bool("F", false, "flip output")
	strip     := flag.Bool("s", false, "strip surrounding newlines")
	norm      := flag.Bool("n", false, "normalize surrounding newlines")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	text := strings.Join(args, " ")

	font, err := loadFont(*fontName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: requested font %q not found.\n", *fontName)
		os.Exit(1)
	}

	dir := resolveDirection(*direction, font.printDirection)
	just := resolveJustify(*justify, dir)

	builder := newFigletBuilder(text, font, dir, *width, just)
	for builder.isNotFinished() {
		builder.addCharToProduct()
		builder.goToNextChar()
	}
	result := builder.returnProduct()

	if *reverse {
		result = reverseArt(result)
	}
	if *flip {
		result = flipArt(result)
	}
	if *strip {
		result = stripSurroundingNewlines(result)
	} else if *norm {
		result = normalizeSurroundingNewlines(result)
	}

	os.Stdout.WriteString(result)
	os.Stdout.WriteString("\n")
}

func resolveDirection(opt string, fontDir int) string {
	if opt != "auto" {
		return opt
	}
	if fontDir == 1 {
		return "right-to-left"
	}
	return "left-to-right"
}

func resolveJustify(opt string, dir string) string {
	if opt != "auto" {
		return opt
	}
	if dir == "right-to-left" {
		return "right"
	}
	return "left"
}

// ── Reverse & Flip mappings (copied exactly from pyfiglet FigletString) ───────

var reverseDiffs = map[rune]rune{
	'(': ')',
	')': '(',
	'/': '\\',
	'<': '>',
	'>': '<',
	'[': ']',
	'\\': '/',
	']': '[',
	'{': '}',
	'}': '{',
}

func reverseArt(s string) string {
	// splitlines() in Python splits by \n without trailing newlines
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	var out []string
	for _, row := range lines {
		runes := []rune(row)
		rev := make([]rune, len(runes))
		for i, r := range runes {
			if m, ok := reverseDiffs[r]; ok {
				rev[len(runes)-1-i] = m
			} else {
				rev[len(runes)-1-i] = r
			}
		}
		out = append(out, string(rev))
	}
	return strings.Join(out, "\n") + "\n"
}

var flipDiffs = map[rune]rune{
	'/': '\\',
	'A': 'V',
	'M': 'W',
	'P': 'b',
	'R': 'b',
	'V': 'A',
	'W': 'M',
	'\\': '/',
	'^': 'v',
	'_': '-',
	'b': 'P',
	'm': 'w',
	'v': '^',
	'w': 'm',
}

func flipArt(s string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		row := lines[i]
		runes := []rune(row)
		for j, r := range runes {
			if m, ok := flipDiffs[r]; ok {
				runes[j] = m
			}
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

func stripSurroundingNewlines(s string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	var out []string
	charsSeen := false
	for _, row := range lines {
		if strings.TrimSpace(row) != "" || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}
	res := strings.Join(out, "\n") + "\n"
	return strings.TrimRightFunc(res, unicode.IsSpace)
}

func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}
GOEOF

# ── 4. Write figlet.go ───────────────────────────────────────────────────────
cat > figlet.go << 'GOEOF'
package main

import (
	"embed"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var fontFS embed.FS

// ── Font ──────────────────────────────────────────────────────────────────────

type FigletFont struct {
	hardBlank      string
	height         int
	baseline       int
	maxLength      int
	smushMode      int
	printDirection int
	chars          map[int][]string
	charWidths     map[int]int
}

var reMagic   = regexp.MustCompile(`^[tf]lf2.`)
var reEndChar = regexp.MustCompile(`(.)\s*$`)

func loadFont(name string) (*FigletFont, error) {
	for _, ext := range []string{"flf", "tlf"} {
		data, err := fontFS.ReadFile("fonts/" + name + "." + ext)
		if err == nil {
			return parseFont(string(data))
		}
	}
	return nil, fmt.Errorf("font not found: %s", name)
}

func parseFont(data string) (*FigletFont, error) {
	data = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(data)
	lines := strings.Split(data, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, "\r")
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font")
	}

	header := lines[0]
	lines = lines[1:]
	if !reMagic.MatchString(header) {
		return nil, fmt.Errorf("invalid magic")
	}
	header = reMagic.ReplaceAllString(header, "")
	fields := strings.Fields(header)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header")
	}

	hardBlank := string([]rune(fields[0])[0])
	height, _       := strconv.Atoi(fields[1])
	baseline, _     := strconv.Atoi(fields[2])
	maxLength, _    := strconv.Atoi(fields[3])
	oldLayout, _    := strconv.Atoi(fields[4])
	commentLines, _ := strconv.Atoi(fields[5])

	printDirection := 0
	fullLayout := -1
	if len(fields) > 6 {
		printDirection, _ = strconv.Atoi(fields[6])
	}
	if len(fields) > 7 {
		fullLayout, _ = strconv.Atoi(fields[7])
	}
	if fullLayout == -1 {
		if oldLayout == 0 {
			fullLayout = 64
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | 128
		}
	}

	font := &FigletFont{
		hardBlank:      hardBlank,
		height:         height,
		baseline:       baseline,
		maxLength:      maxLength,
		smushMode:      fullLayout,
		printDirection: printDirection,
		chars:          make(map[int][]string),
		charWidths:     make(map[int]int),
	}

	for i := 0; i < commentLines && len(lines) > 0; i++ {
		lines = lines[1:]
	}

	readChar := func() (int, []string, error) {
		if len(lines) < height {
			return 0, nil, fmt.Errorf("not enough lines")
		}
		var endRe *regexp.Regexp
		charLines := make([]string, height)
		width := 0
		for row := 0; row < height; row++ {
			line := lines[row]
			if endRe == nil {
				m := reEndChar.FindStringSubmatch(line)
				if m == nil {
					return 0, nil, fmt.Errorf("no end marker")
				}
				escaped := regexp.QuoteMeta(m[1])
				endRe = regexp.MustCompile(escaped + `{1,2}\s*$`)
			}
			line = endRe.ReplaceAllString(line, "")
			if len(line) > width {
				width = len(line)
			}
			charLines[row] = line
		}
		lines = lines[height:]
		return width, charLines, nil
	}

	// ASCII 32..126
	for i := 32; i < 127; i++ {
		w, ch, err := readChar()
		if err != nil {
			break
		}
		if i == 32 || strings.Join(ch, "") != "" {
			font.chars[i] = ch
			font.charWidths[i] = w
		}
	}

	// German umlauts
	for _, r := range "ÄÖÜäöüß" {
		if len(lines) == 0 {
			break
		}
		w, ch, err := readChar()
		if err != nil {
			break
		}
		if strings.Join(ch, "") != "" {
			font.chars[int(r)] = ch
			font.charWidths[int(r)] = w
		}
	}

	// Extended 0x characters
	reHex := regexp.MustCompile(`(?i)^0x`)
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if reHex.MatchString(parts[0]) {
			n, err := strconv.ParseInt(parts[0][2:], 16, 32)
			if err == nil {
				w, ch, err2 := readChar()
				if err2 == nil && strings.Join(ch, "") != "" {
					font.chars[int(n)] = ch
					font.charWidths[int(n)] = w
				}
			}
		}
	}

	return font, nil
}

// ── Smushing constants ────────────────────────────────────────────────────────

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

// ── FigletBuilder ─────────────────────────────────────────────────────────────

type blankMarker struct {
	buffer   []string
	iterator int
}

type FigletBuilder struct {
	text          []rune
	direction     string
	width         int
	font          *FigletFont
	justify       string

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []blankMarker
	queue        [][]string
	buffer       []string
}

func newFigletBuilder(text string, font *FigletFont, direction string, width int, justify string) *FigletBuilder {
	b := &FigletBuilder{
		text:      []rune(text),
		direction: direction,
		width:     width,
		font:      font,
		justify:   justify,
		buffer:    make([]string, font.height),
	}
	return b
}

func (b *FigletBuilder) isNotFinished() bool {
	return b.iterator < len(b.text)
}

func (b *FigletBuilder) goToNextChar() {
	b.iterator++
}

func (b *FigletBuilder) getCurChar() []string {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return nil
	}
	c := int(b.text[b.iterator])
	return b.font.chars[c]
}

func (b *FigletBuilder) getCurWidth() int {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return 0
	}
	c := int(b.text[b.iterator])
	return b.font.charWidths[c]
}

func (b *FigletBuilder) currentSmushAmount(curChar []string) int {
	return b.smushAmount(b.buffer, curChar)
}

func (b *FigletBuilder) smushChars(left, right string) (string, bool) {
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
	if b.font.smushMode&smEqual != 0 && left == right {
		return left, true
	}
	if b.font.smushMode&smLowline != 0 {
		const hiers = `|/\[]{}()<>`
		if left == "_" && strings.ContainsAny(right, hiers) {
			return right, true
		}
		if right == "_" && strings.ContainsAny(left, hiers) {
			return left, true
		}
	}
	if b.font.smushMode&smHierarchy != 0 {
		type pair struct{ a, b string }
		ranks := []pair{
			{"|", `/\[]{}()<>`},
			{`\/`, `[]{}()<>`},
			{"[]", `{}()<>`},
			{"{}", `()<>`},
			{"()", `<>`},
		}
		for _, p := range ranks {
			if strings.ContainsAny(left, p.a) && strings.ContainsAny(right, p.b) {
				return right, true
			}
			if strings.ContainsAny(right, p.a) && strings.ContainsAny(left, p.b) {
				return left, true
			}
		}
	}
	if b.font.smushMode&smPair != 0 {
		for _, combo := range []string{left + right, right + left} {
			if combo == "[]" || combo == "{}" || combo == "()" {
				return "|", true
			}
		}
	}
	if b.font.smushMode&smBigX != 0 {
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

func (b *FigletBuilder) smushAmount(buffer []string, curChar []string) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft  := buffer[row]
		lineRight := curChar[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}
		linebd := len(strings.TrimRight(lineLeft, " ")) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := ""
		if linebd < len(lineLeft) {
			ch1 = string(lineLeft[linebd])
		} else {
			linebd = 0
		}

		charbd := len(lineRight) - len(strings.TrimLeft(lineRight, " "))
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

func (b *FigletBuilder) smushRow(curChar []string, row int) (string, string) {
	addLeft  := b.buffer[row]
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	l := []byte(addLeft)
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := ""
		if idx >= 0 && idx < len(addLeft) {
			left = string(addLeft[idx])
		}
		right := ""
		if i < len(addRight) {
			right = string(addRight[i])
		}
		if smushed, ok := b.smushChars(left, right); ok && len(smushed) > 0 {
			if idx >= 0 && idx < len(l) {
				l[idx] = smushed[0]
			}
		}
	}
	return string(l), addRight
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	rightSlice := ""
	if b.maxSmush < len(addRight) {
		rightSlice = addRight[b.maxSmush:]
	}
	b.buffer[row] = addLeft + rightSlice
}

func (b *FigletBuilder) copyBuffer() []string {
	cp := make([]string, len(b.buffer))
	copy(cp, b.buffer)
	return cp
}

func (b *FigletBuilder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar := b.getCurChar()
	if curChar == nil {
		return
	}
	b.maxSmush = b.currentSmushAmount(curChar)
}

func (b *FigletBuilder) cutBufferAtLastBlank(savedBuffer []string, savedIterator int) {
	b.queue = append(b.queue, savedBuffer)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *FigletBuilder) cutBufferAtLastChar() {
	b.queue = append(b.queue, b.copyBuffer())
	b.iterator--
	b.cutBufferCommon()
}

func (b *FigletBuilder) handleNewLine() {
	if len(b.blankMarkers) > 0 {
		last := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.cutBufferAtLastBlank(last.buffer, last.iterator)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *FigletBuilder) addCharToProduct() {
	curChar := b.getCurChar()

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: b.copyBuffer(), iterator: b.iterator})
		b.handleNewLine()
		return
	}

	if curChar == nil {
		return
	}
	if b.width < b.getCurWidth() {
		return
	}
	b.curCharWidth = b.getCurWidth()
	b.maxSmush = b.currentSmushAmount(curChar)

	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: b.copyBuffer(), iterator: b.iterator})
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

func (b *FigletBuilder) returnProduct() string {
	if b.buffer[0] != "" {
		b.queue = append(b.queue, b.copyBuffer())
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		jBuf := b.justifyString(b.justify, buf)
		str := strings.Join(jBuf, "\n") + "\n"
		str = strings.ReplaceAll(str, b.font.hardBlank, " ")
		sb.WriteString(str)
	}
	return sb.String()
}

func (b *FigletBuilder) justifyString(justify string, buffer []string) []string {
	out := make([]string, len(buffer))
	copy(out, buffer)
	if justify == "right" {
		for row := 0; row < b.font.height; row++ {
			pad := b.width - len(out[row]) - 1
			if pad > 0 {
				out[row] = strings.Repeat(" ", pad) + out[row]
			}
		}
	} else if justify == "center" {
		for row := 0; row < b.font.height; row++ {
			pad := (b.width - len(out[row])) / 2
			if pad > 0 {
				out[row] = strings.Repeat(" ", pad) + out[row]
			}
		}
	}
	return out
}
GOEOF

# ── 5. Copy fonts ────────────────────────────────────────────────────────────
mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/ 2>/dev/null || true
cp pyfiglet/fonts/*.tlf fonts/ 2>/dev/null || true

# ── 6. Remove Python files ───────────────────────────────────────────────────
find /workspace/repo -name "*.py" -delete

# ── 7. Build ─────────────────────────────────────────────────────────────────
cd /workspace/repo
go build ./...
go build -o /tmp/figlet .
echo "Build OK"
