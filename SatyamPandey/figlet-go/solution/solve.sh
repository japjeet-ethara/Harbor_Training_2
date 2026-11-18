#!/usr/bin/env bash
set -euo pipefail

# STEP A — Copy the fonts
mkdir -p /workspace/repo/fonts
cp /workspace/repo/pyfiglet/fonts/*.flf /workspace/repo/fonts/

# STEP B — Delete every .py file under /workspace/repo
find /workspace/repo -name "*.py" -delete

# STEP C — Write go.mod
cat <<'EOF' > /workspace/repo/go.mod
module figlet

go 1.23
EOF

# STEP D — Write the Go implementation
cat <<'EOF' > /workspace/repo/main.go
package main

import (
	"embed"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var fontFS embed.FS

const (
	SM_EQUAL     = 1
	SM_LOWLINE   = 2
	SM_HIERARCHY = 4
	SM_PAIR      = 8
	SM_BIGX      = 16
	SM_HARDBLANK = 32
	SM_KERN      = 64
	SM_SMUSH     = 128
)

type FigletFont struct {
	font           string
	height         int
	hardBlank      byte
	printDirection int
	smushMode      int
	chars          map[int][]string
	width          map[int]int
}

func loadFont(name string) (*FigletFont, error) {
	dataBytes, err := fontFS.ReadFile("fonts/" + name + ".flf")
	if err != nil {
		dataBytes, err = os.ReadFile("fonts/" + name + ".flf")
		if err != nil {
			return nil, fmt.Errorf("font %s not found: %w", name, err)
		}
	}

	dataStr := string(dataBytes)
	dataStr = regexp.MustCompile("[\u0085\u2028\u2029]").ReplaceAllString(dataStr, " ")
	lines := strings.Split(dataStr, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font file")
	}

	header := lines[0]
	lines = lines[1:]

	reMagic := regexp.MustCompile(`^[tf]lf2.`)
	if !reMagic.MatchString(header) {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}

	header = reMagic.ReplaceAllString(header, "")
	hFields := strings.Fields(header)
	if len(hFields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}

	hardBlank := hFields[0][0]
	height, _ := strconv.Atoi(hFields[1])
	oldLayout, _ := strconv.Atoi(hFields[4])
	commentLines, _ := strconv.Atoi(hFields[5])

	printDir := 0
	fullLayout := -1
	if len(hFields) > 6 {
		printDir, _ = strconv.Atoi(hFields[6])
	}
	if len(hFields) > 7 {
		fullLayout, _ = strconv.Atoi(hFields[7])
	}

	smushMode := fullLayout
	if fullLayout == -1 {
		if oldLayout == 0 {
			smushMode = 64
		} else if oldLayout < 0 {
			smushMode = 0
		} else {
			smushMode = (oldLayout & 31) | 128
		}
	}

	if len(lines) < commentLines {
		return nil, fmt.Errorf("not enough comment lines")
	}
	lines = lines[commentLines:]

	ff := &FigletFont{
		font:           name,
		height:         height,
		hardBlank:      hardBlank,
		printDirection: printDir,
		smushMode:      smushMode,
		chars:          make(map[int][]string),
		width:          make(map[int]int),
	}

	reEndMarker := regexp.MustCompile(`(.)\s*$`)

	readChar := func() (int, []string) {
		if len(lines) < height {
			return 0, nil
		}
		var charLines []string
		maxWidth := 0
		var reSub *regexp.Regexp

		for r := 0; r < height; r++ {
			line := lines[0]
			lines = lines[1:]

			if reSub == nil {
				m := reEndMarker.FindStringSubmatch(line)
				if len(m) > 1 {
					endChar := m[1]
					reSub = regexp.MustCompile(regexp.QuoteMeta(endChar) + `{1,2}\s*$`)
				}
			}

			if reSub != nil {
				line = reSub.ReplaceAllString(line, "")
			}

			if len(line) > maxWidth {
				maxWidth = len(line)
			}
			charLines = append(charLines, line)
		}
		return maxWidth, charLines
	}

	// ASCII 32 - 126
	for i := 32; i < 127; i++ {
		w, letter := readChar()
		if letter == nil {
			break
		}
		if i == 32 || strings.Join(letter, "") != "" {
			ff.chars[i] = letter
			ff.width[i] = w
		}
	}

	// German Umlaute
	german := []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'}
	for _, r := range german {
		if len(lines) < height {
			break
		}
		w, letter := readChar()
		if letter != nil && strings.Join(letter, "") != "" {
			ff.chars[int(r)] = letter
			ff.width[int(r)] = w
		}
	}

	// Extended characters
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		codeStr := f[0]
		if strings.HasPrefix(strings.ToLower(codeStr), "0x") {
			code, err := strconv.ParseInt(codeStr[2:], 16, 64)
			if err == nil {
				w, letter := readChar()
				if letter != nil && strings.Join(letter, "") != "" {
					ff.chars[int(code)] = letter
					ff.width[int(code)] = w
				}
			}
		}
	}

	return ff, nil
}

type blankMarker struct {
	buffer []string
	iter   int
}

type FigletBuilder struct {
	text              []int
	direction         string
	width             int
	font              *FigletFont
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

func newBuilder(text string, font *FigletFont, direction string, width int, justify string) *FigletBuilder {
	var textOrd []int
	for _, r := range text {
		textOrd = append(textOrd, int(r))
	}

	dir := direction
	if dir == "auto" {
		if font.printDirection == 1 {
			dir = "right-to-left"
		} else {
			dir = "left-to-right"
		}
	}

	jst := justify
	if jst == "auto" || jst == "" {
		if dir == "right-to-left" {
			jst = "right"
		} else {
			jst = "left"
		}
	}

	return &FigletBuilder{
		text:         textOrd,
		direction:    dir,
		width:        width,
		font:         font,
		justify:      jst,
		iterator:     0,
		buffer:       make([]string, font.height),
		blankMarkers: nil,
		product:      nil,
	}
}

func (b *FigletBuilder) getCharAt(i int) []string {
	if i < 0 || i >= len(b.text) {
		return nil
	}
	c := b.text[i]
	return b.font.chars[c]
}

func (b *FigletBuilder) getCharWidthAt(i int) int {
	if i < 0 || i >= len(b.text) {
		return 0
	}
	c := b.text[i]
	return b.font.width[c]
}

func (b *FigletBuilder) getCurChar() []string {
	return b.getCharAt(b.iterator)
}

func (b *FigletBuilder) getCurWidth() int {
	return b.getCharWidthAt(b.iterator)
}

func (b *FigletBuilder) smushChars(left, right byte) byte {
	if left == ' ' {
		return right
	}
	if right == ' ' {
		return left
	}

	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0
	}

	if (b.font.smushMode & SM_SMUSH) == 0 {
		return 0
	}

	if (b.font.smushMode & 63) == 0 {
		if left == b.font.hardBlank {
			return right
		}
		if right == b.font.hardBlank {
			return left
		}
		if b.direction == "right-to-left" {
			return left
		}
		return right
	}

	if (b.font.smushMode & SM_HARDBLANK) != 0 {
		if left == b.font.hardBlank && right == b.font.hardBlank {
			return left
		}
	}

	if left == b.font.hardBlank || right == b.font.hardBlank {
		return 0
	}

	if (b.font.smushMode & SM_EQUAL) != 0 {
		if left == right {
			return left
		}
	}

	if (b.font.smushMode & SM_LOWLINE) != 0 {
		if left == '_' && strings.IndexByte("|/\\[]{}()<>", right) != -1 {
			return right
		}
		if right == '_' && strings.IndexByte("|/\\[]{}()<>", left) != -1 {
			return left
		}
	}

	if (b.font.smushMode & SM_HIERARCHY) != 0 {
		type pair struct{ a, b string }
		hierarchy := []pair{
			{"|", "/\\[]{}()<>"},
			{"/\\", "[]{}()<>"},
			{"[]", "{}()<>"},
			{"{}", "()<>"},
			{"()", "<>"},
		}
		for _, h := range hierarchy {
			if strings.IndexByte(h.a, left) != -1 && strings.IndexByte(h.b, right) != -1 {
				return right
			}
			if strings.IndexByte(h.a, right) != -1 && strings.IndexByte(h.b, left) != -1 {
				return left
			}
		}
	}

	if (b.font.smushMode & SM_PAIR) != 0 {
		p1 := string([]byte{left, right})
		p2 := string([]byte{right, left})
		if p1 == "[]" || p1 == "{}" || p1 == "()" || p2 == "[]" || p2 == "{}" || p2 == "()" {
			return '|'
		}
	}

	if (b.font.smushMode & SM_BIGX) != 0 {
		if left == '/' && right == '\\' {
			return '|'
		}
		if right == '/' && left == '\\' {
			return 'Y'
		}
		if left == '>' && right == '<' {
			return 'X'
		}
	}

	return 0
}

func (b *FigletBuilder) smushAmount(curChar []string) int {
	if (b.font.smushMode & (SM_SMUSH | SM_KERN)) == 0 {
		return 0
	}

	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := b.buffer[row]
		lineRight := curChar[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		linebd := len(strings.TrimRight(lineLeft, " ")) - 1
		if linebd < 0 {
			linebd = 0
		}

		var ch1 byte
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		}

		rightTrimmed := strings.TrimLeft(lineRight, " ")
		charbd := len(lineRight) - len(rightTrimmed)
		var ch2 byte
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		}

		amt := charbd + len(lineLeft) - 1 - linebd

		if ch1 == 0 || ch1 == ' ' {
			amt++
		} else if ch2 != 0 && b.smushChars(ch1, ch2) != 0 {
			amt++
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

func (b *FigletBuilder) getLeftSmushedChar(i int, addLeft string) (byte, int) {
	idx := len(addLeft) - b.maxSmush + i
	if idx >= 0 && idx < len(addLeft) {
		return addLeft[idx], idx
	}
	return 0, idx
}

func (b *FigletBuilder) updateSmushedCharInLeftBuffer(addLeft string, idx int, smushed byte) string {
	l := []byte(addLeft)
	if idx < 0 || idx >= len(l) {
		return addLeft
	}
	l[idx] = smushed
	return string(l)
}

func (b *FigletBuilder) smushRow(curChar []string, row int) (string, string) {
	addLeft := b.buffer[row]
	addRight := curChar[row]

	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for i := 0; i < b.maxSmush; i++ {
		left, idx := b.getLeftSmushedChar(i, addLeft)
		var right byte
		if i < len(addRight) {
			right = addRight[i]
		}
		smushed := b.smushChars(left, right)
		addLeft = b.updateSmushedCharInLeftBuffer(addLeft, idx, smushed)
	}
	return addLeft, addRight
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	rightPart := ""
	if b.maxSmush < len(addRight) {
		rightPart = addRight[b.maxSmush:]
	}
	b.buffer[row] = addLeft + rightPart
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
	b.maxSmush = b.smushAmount(curChar)
}

func (b *FigletBuilder) cutBufferAtLastBlank(savedBuffer []string, savedIterator int) {
	b.product = append(b.product, savedBuffer)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *FigletBuilder) cutBufferAtLastChar() {
	b.product = append(b.product, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *FigletBuilder) getLastBlank() ([]string, int, bool) {
	if len(b.blankMarkers) == 0 {
		return nil, -1, false
	}
	last := b.blankMarkers[len(b.blankMarkers)-1]
	b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
	return last.buffer, last.iter, true
}

func (b *FigletBuilder) handleNewLine() {
	savedBuf, savedIter, ok := b.getLastBlank()
	if ok && savedIter != -1 {
		b.cutBufferAtLastBlank(savedBuf, savedIter)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *FigletBuilder) addCharToProduct() {
	curChar := b.getCurChar()

	if b.text[b.iterator] == '\n' {
		bufCopy := append([]string(nil), b.buffer...)
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: bufCopy, iter: b.iterator})
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
	b.maxSmush = b.smushAmount(curChar)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		bufCopy := append([]string(nil), b.buffer...)
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: bufCopy, iter: b.iterator})
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

func (b *FigletBuilder) formatProduct() string {
	if b.buffer[0] != "" {
		b.product = append(b.product, b.buffer)
	}

	var sb strings.Builder
	for _, buf := range b.product {
		justified := b.justifyString(b.justify, buf)
		sb.WriteString(b.replaceHardblanks(justified))
	}
	return sb.String()
}

func (b *FigletBuilder) justifyString(justify string, buf []string) []string {
	result := make([]string, b.font.height)
	copy(result, buf)

	if justify == "right" {
		for row := 0; row < b.font.height; row++ {
			pad := b.width - len(result[row]) - 1
			if pad > 0 {
				result[row] = strings.Repeat(" ", pad) + result[row]
			}
		}
	} else if justify == "center" {
		for row := 0; row < b.font.height; row++ {
			pad := (b.width - len(result[row])) / 2
			if pad > 0 {
				result[row] = strings.Repeat(" ", pad) + result[row]
			}
		}
	}
	return result
}

func (b *FigletBuilder) replaceHardblanks(buf []string) string {
	joined := strings.Join(buf, "\n") + "\n"
	return strings.ReplaceAll(joined, string(b.font.hardBlank), " ")
}

func renderText(text string, font *FigletFont, direction string, justify string, width int) string {
	builder := newBuilder(text, font, direction, width, justify)
	for builder.iterator < len(builder.text) {
		builder.addCharToProduct()
		builder.iterator++
	}
	return builder.formatProduct()
}

var reverseMap [256]byte
var flipMap [256]byte

func init() {
	for i := 0; i < 256; i++ {
		reverseMap[i] = byte(i)
		flipMap[i] = byte(i)
	}

	revDiffs := map[byte]byte{
		'(': ')', ')': '(',
		'/': '\\', '\\': '/',
		'<': '>', '>': '<',
		'[': ']', ']': '[',
		'{': '}', '}': '{',
	}
	for k, v := range revDiffs {
		reverseMap[k] = v
	}

	flipDiffs := map[byte]byte{
		'/': '\\', '\\': '/',
		'A': 'V', 'V': 'A',
		'M': 'W', 'W': 'M',
		'P': 'b', 'R': 'b', 'b': 'P',
		'^': 'v', 'v': '^',
		'_': '-',
		'm': 'w', 'w': 'm',
	}
	for k, v := range flipDiffs {
		flipMap[k] = v
	}
}

func applyReverse(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var out []string
	for _, row := range lines {
		b := []byte(row)
		rev := make([]byte, len(b))
		for i, c := range b {
			rev[len(b)-1-i] = reverseMap[c]
		}
		out = append(out, string(rev))
	}
	return strings.Join(out, "\n") + "\n"
}

func applyFlip(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		row := lines[i]
		b := []byte(row)
		for j, c := range b {
			b[j] = flipMap[c]
		}
		out = append(out, string(b))
	}
	return strings.Join(out, "\n") + "\n"
}

func stripSurroundingNewlines(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var out []string
	charsSeen := false
	for _, row := range lines {
		if strings.TrimSpace(row) != "" || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}

	res := strings.Join(out, "\n") + "\n"
	return strings.TrimRight(res, " \t\r\n")
}

func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}

func main() {
	fontName := "standard"
	width := 80
	justify := "auto"
	direction := "auto"
	doReverse := false
	doFlip := false
	doStrip := false
	doNormalize := false
	var textArgs []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-f" || a == "--font" {
			if i+1 < len(args) {
				fontName = args[i+1]
				i++
			}
		} else if a == "-w" || a == "--width" {
			if i+1 < len(args) {
				if w, err := strconv.Atoi(args[i+1]); err == nil {
					width = w
				}
				i++
			}
		} else if a == "-j" || a == "--justify" {
			if i+1 < len(args) {
				justify = args[i+1]
				i++
			}
		} else if a == "-D" || a == "--direction" {
			if i+1 < len(args) && (args[i+1] == "auto" || args[i+1] == "left-to-right" || args[i+1] == "right-to-left") {
				direction = args[i+1]
				i++
			} else {
				direction = "right-to-left"
			}
		} else if a == "-r" || a == "--reverse" {
			doReverse = true
		} else if a == "-F" || a == "--flip" {
			doFlip = true
		} else if a == "-s" || a == "--strip-surrounding-newlines" {
			doStrip = true
		} else if a == "-n" || a == "--normalize-surrounding-newlines" {
			doNormalize = true
		} else {
			textArgs = append(textArgs, a)
		}
	}

	if len(textArgs) == 0 {
		return
	}

	text := strings.Join(textArgs, " ")

	font, err := loadFont(fontName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: requested font %q not found.\n", fontName)
		os.Exit(1)
	}

	r := renderText(text, font, direction, justify, width)

	if doReverse {
		r = applyReverse(r)
	}
	if doFlip {
		r = applyFlip(r)
	}
	if doStrip {
		r = stripSurroundingNewlines(r)
	} else if doNormalize {
		r = normalizeSurroundingNewlines(r)
	}

	os.Stdout.WriteString(r)
	os.Stdout.WriteString("\n")
}
EOF
