#!/bin/bash
set -euo pipefail

cd /workspace/repo

# 1. Prepare fonts directory for go:embed
mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/

# 2. Write go.mod
cat << 'EOF' > go.mod
module figlet

go 1.23
EOF

# 3. Write main.go
cat << 'EOF' > main.go
package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

// ---------------------------------------------------------------------
// Smushing constants (lifted from figlet222 / pyfiglet)
// ---------------------------------------------------------------------
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

// ---------------------------------------------------------------------
// Translation maps for reverse and flip
// ---------------------------------------------------------------------
var reverseMap = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
	32, 33, 34, 35, 36, 37, 38, 39, 41, 40, 42, 43, 44, 45, 46, 92,
	48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 62, 61, 60, 63,
	64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
	80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 93, 47, 91, 94, 95,
	96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111,
	112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 125, 124, 123, 126, 127,
	128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143,
	144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159,
	160, 161, 162, 163, 164, 165, 166, 167, 168, 169, 170, 171, 172, 173, 174, 175,
	176, 177, 178, 179, 180, 181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191,
	192, 193, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207,
	208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223,
	224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239,
	240, 241, 242, 243, 244, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255,
}

var flipMap = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
	32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 92,
	48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63,
	64, 86, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 87, 78, 79,
	98, 81, 98, 83, 84, 85, 65, 77, 88, 89, 90, 91, 47, 93, 118, 45,
	96, 97, 80, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 119, 110, 111,
	112, 113, 114, 115, 116, 117, 94, 109, 120, 121, 122, 123, 124, 125, 126, 127,
	128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143,
	144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159,
	160, 161, 162, 163, 164, 165, 166, 167, 168, 169, 170, 171, 172, 173, 174, 175,
	176, 177, 178, 179, 180, 181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191,
	192, 193, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207,
	208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223,
	224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239,
	240, 241, 242, 243, 244, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255,
}

// ---------------------------------------------------------------------
// FigletFont
// ---------------------------------------------------------------------
type FigletFont struct {
	height         int
	hardBlank      byte
	printDirection int // 0 = LTR, 1 = RTL
	smushMode      int
	chars          map[rune][]string
	width          map[rune]int
}

func loadFont(name string) (*FigletFont, error) {
	var data []byte
	var err error

	fn := name + ".flf"
	data, err = embeddedFonts.ReadFile("fonts/" + fn)
	if err != nil {
		paths := []string{
			fn,
			filepath.Join("fonts", fn),
			filepath.Join("pyfiglet", "fonts", fn),
			filepath.Join("/workspace/repo/pyfiglet/fonts", fn),
			filepath.Join("/workspace/repo/fonts", fn),
		}
		for _, p := range paths {
			if d, e := os.ReadFile(p); e == nil {
				data = d
				err = nil
				break
			}
		}
	}
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("font %s not found", name)
	}

	content := string(data)
	content = strings.ReplaceAll(content, "\u0085", " ")
	content = strings.ReplaceAll(content, "\u2028", " ")
	content = strings.ReplaceAll(content, "\u2029", " ")

	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font")
	}

	headerLine := lines[0]
	lines = lines[1:]

	reMagic := regexp.MustCompile(`^[tf]lf2.`)
	if !reMagic.MatchString(headerLine) {
		return nil, fmt.Errorf("invalid magic in %s", name)
	}

	headerRest := reMagic.ReplaceAllString(headerLine, "")
	fields := strings.Fields(headerRest)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header in %s", name)
	}

	hardBlank := fields[0][0]
	height, _ := strconv.Atoi(fields[1])
	oldLayout, _ := strconv.Atoi(fields[4])
	commentLines, _ := strconv.Atoi(fields[5])

	printDir := 0
	if len(fields) > 6 {
		printDir, _ = strconv.Atoi(fields[6])
	}

	var fullLayout *int
	if len(fields) > 7 {
		v, _ := strconv.Atoi(fields[7])
		fullLayout = &v
	}

	var smushMode int
	if fullLayout != nil {
		smushMode = *fullLayout
	} else {
		if oldLayout == 0 {
			smushMode = 64
		} else if oldLayout < 0 {
			smushMode = 0
		} else {
			smushMode = (oldLayout & 31) | 128
		}
	}

	if len(lines) < commentLines {
		return nil, fmt.Errorf("not enough lines for comments in %s", name)
	}
	lines = lines[commentLines:]

	font := &FigletFont{
		height:         height,
		hardBlank:      hardBlank,
		printDirection: printDir,
		smushMode:      smushMode,
		chars:          make(map[rune][]string),
		width:          make(map[rune]int),
	}

	var endPattern *regexp.Regexp
	readChar := func() (int, []string, bool) {
		if len(lines) < height {
			return 0, nil, false
		}
		charRows := make([]string, height)
		maxWidth := 0
		for row := 0; row < height; row++ {
			line := lines[0]
			lines = lines[1:]
			if endPattern == nil {
				reEnd := regexp.MustCompile(`(.)\s*$`)
				m := reEnd.FindStringSubmatch(line)
				if len(m) > 1 {
					endChar := regexp.QuoteMeta(m[1])
					endPattern = regexp.MustCompile(endChar + `{1,2}\s*$`)
				}
			}
			if endPattern != nil {
				line = endPattern.ReplaceAllString(line, "")
			}
			w := len(line)
			if w > maxWidth {
				maxWidth = w
			}
			charRows[row] = line
		}
		return maxWidth, charRows, true
	}

	// 1. ASCII 32 to 126
	for i := 32; i < 127; i++ {
		w, letter, ok := readChar()
		if !ok {
			break
		}
		hasContent := false
		for _, r := range letter {
			if r != "" {
				hasContent = true
				break
			}
		}
		if i == 32 || hasContent {
			font.chars[rune(i)] = letter
			font.width[rune(i)] = w
		}
	}

	// 2. German Umlauts: ÄÖÜäöüß
	german := []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'}
	for _, gr := range german {
		if len(lines) < height {
			break
		}
		w, letter, ok := readChar()
		if !ok {
			break
		}
		hasContent := false
		for _, r := range letter {
			if r != "" {
				hasContent = true
				break
			}
		}
		if hasContent {
			font.chars[gr] = letter
			font.width[gr] = w
		}
	}

	// 3. Extended characters
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		codeStr := parts[0]
		if strings.HasPrefix(strings.ToLower(codeStr), "0x") {
			val, err := strconv.ParseInt(codeStr[2:], 16, 64)
			if err == nil {
				w, letter, ok := readChar()
				if ok {
					hasContent := false
					for _, r := range letter {
						if r != "" {
							hasContent = true
							break
						}
					}
					if hasContent {
						font.chars[rune(val)] = letter
						font.width[rune(val)] = w
					}
				}
			}
		}
	}

	return font, nil
}

// ---------------------------------------------------------------------
// FigletBuilder
// ---------------------------------------------------------------------
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
	iterator      int
	maxSmush      int
	curCharWidth  int
	prevCharWidth int
	totalWidth    int
	blankMarkers  []blankMarker
	queue         [][]string
	buffer        []string
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
	c := b.text[b.iterator]
	return b.font.chars[c]
}

func (b *FigletBuilder) getCurWidth() int {
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return 0
	}
	c := b.text[b.iterator]
	return b.font.width[c]
}

func (b *FigletBuilder) copyBuffer() []string {
	res := make([]string, len(b.buffer))
	copy(res, b.buffer)
	return res
}

func (b *FigletBuilder) addCharToProduct() {
	curChar := b.getCurChar()

	// Newline handling
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
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.totalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: b.copyBuffer(), iterator: b.iterator})
	}

	if b.totalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}

	b.prevCharWidth = b.curCharWidth
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

func (b *FigletBuilder) cutBufferCommon() {
	b.totalWidth = 0
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar := b.getCurChar()
	if curChar == nil {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
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

func (b *FigletBuilder) smushRow(curChar []string, row int) (string, string) {
	addLeft := b.buffer[row]
	addRight := curChar[row]

	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		var left byte
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		right := addRight[i]
		smushed := b.smushChars(left, right)
		if smushed != 0 && idx >= 0 && idx < len(addLeft) {
			bLeft := []byte(addLeft)
			bLeft[idx] = smushed
			addLeft = string(bLeft)
		}
	}
	return addLeft, addRight
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	b.buffer[row] = addLeft + addRight[b.maxSmush:]
}

func (b *FigletBuilder) smushAmount(buffer []string, curChar []string) int {
	if (b.font.smushMode & (SM_SMUSH | SM_KERN)) == 0 {
		return 0
	}

	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		rTrim := strings.TrimRight(lineLeft, " ")
		linebd := len(rTrim) - 1
		if linebd < 0 {
			linebd = 0
		}
		var ch1 byte
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
			ch1 = 0
		}

		lTrim := strings.TrimLeft(lineRight, " ")
		charbd := len(lineRight) - len(lTrim)
		var ch2 byte
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
			ch2 = 0
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

func (b *FigletBuilder) smushChars(left byte, right byte) byte {
	if left == ' ' {
		return right
	}
	if right == ' ' {
		return left
	}

	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0
	}

	// Kerning only
	if (b.font.smushMode & SM_SMUSH) == 0 {
		return 0
	}

	// Universal overlapping
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
		if left == '_' && strings.IndexByte(`|/\[]{}()<>`, right) != -1 {
			return right
		}
		if right == '_' && strings.IndexByte(`|/\[]{}()<>`, left) != -1 {
			return left
		}
	}

	if (b.font.smushMode & SM_HIERARCHY) != 0 {
		type pair struct{ a, b string }
		hierarchy := []pair{
			{"|", `/\[]{}()<>`},
			{`\/`, `[]{}()<>`},
			{"[]", `{}()<>`},
			{"{}", `()<>`},
			{"()", `<>`},
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

func (b *FigletBuilder) returnProduct() string {
	if len(b.buffer) > 0 && b.buffer[0] != "" {
		b.queue = append(b.queue, b.copyBuffer())
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		jBuf := b.justifyBuffer(buf)
		line := strings.Join(jBuf, "\n") + "\n"
		line = strings.ReplaceAll(line, string(b.font.hardBlank), " ")
		sb.WriteString(line)
	}
	return sb.String()
}

func (b *FigletBuilder) justifyBuffer(buf []string) []string {
	res := make([]string, len(buf))
	copy(res, buf)
	if b.justify == "right" {
		for r := 0; r < b.font.height; r++ {
			pad := b.width - len(res[r]) - 1
			if pad > 0 {
				res[r] = strings.Repeat(" ", pad) + res[r]
			}
		}
	} else if b.justify == "center" {
		for r := 0; r < b.font.height; r++ {
			pad := (b.width - len(res[r])) / 2
			if pad > 0 {
				res[r] = strings.Repeat(" ", pad) + res[r]
			}
		}
	}
	return res
}

// ---------------------------------------------------------------------
// Transformations: reverse, flip, strip, normalize
// ---------------------------------------------------------------------
func reverseString(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, len(lines))
	for idx, row := range lines {
		bRow := []byte(row)
		revRow := make([]byte, len(bRow))
		for i := 0; i < len(bRow); i++ {
			revRow[len(bRow)-1-i] = reverseMap[bRow[i]]
		}
		out[idx] = string(revRow)
	}
	return strings.Join(out, "\n") + "\n"
}

func flipString(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, len(lines))
	for idx := 0; idx < len(lines); idx++ {
		row := lines[len(lines)-1-idx]
		bRow := []byte(row)
		for i := 0; i < len(bRow); i++ {
			bRow[i] = flipMap[bRow[i]]
		}
		out[idx] = string(bRow)
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
	joined := strings.Join(out, "\n") + "\n"
	return strings.TrimRight(joined, " \t\r\n")
}

func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}

// ---------------------------------------------------------------------
// Render function
// ---------------------------------------------------------------------
func Render(text string, fontName, direction, justify string, width int, reverse, flip, strip, norm bool) (string, error) {
	font, err := loadFont(fontName)
	if err != nil {
		return "", err
	}

	actualDir := direction
	if actualDir == "auto" {
		if font.printDirection == 1 {
			actualDir = "right-to-left"
		} else {
			actualDir = "left-to-right"
		}
	}

	actualJust := justify
	if actualJust == "auto" {
		if actualDir == "right-to-left" {
			actualJust = "right"
		} else {
			actualJust = "left"
		}
	}

	builder := newFigletBuilder(text, font, actualDir, width, actualJust)
	for builder.isNotFinished() {
		builder.addCharToProduct()
		builder.goToNextChar()
	}

	product := builder.returnProduct()

	if reverse {
		product = reverseString(product)
	}
	if flip {
		product = flipString(product)
	}
	if strip {
		product = stripSurroundingNewlines(product)
	} else if norm {
		product = normalizeSurroundingNewlines(product)
	}

	return product + "\n", nil
}

// ---------------------------------------------------------------------
// CLI main
// ---------------------------------------------------------------------
func main() {
	var (
		font      string
		width     int
		justify   string
		direction string
		reverse   bool
		flip      bool
		strip     bool
		norm      bool
	)

	fs := flag.NewFlagSet("figlet", flag.ExitOnError)
	fs.StringVar(&font, "f", "standard", "font")
	fs.StringVar(&font, "font", "standard", "font")
	fs.IntVar(&width, "w", 80, "width")
	fs.IntVar(&width, "width", 80, "width")
	fs.StringVar(&justify, "j", "auto", "justify")
	fs.StringVar(&justify, "justify", "auto", "justify")
	fs.StringVar(&direction, "D", "auto", "direction")
	fs.StringVar(&direction, "direction", "auto", "direction")
	fs.BoolVar(&reverse, "r", false, "reverse")
	fs.BoolVar(&reverse, "reverse", false, "reverse")
	fs.BoolVar(&flip, "F", false, "flip")
	fs.BoolVar(&flip, "flip", false, "flip")
	fs.BoolVar(&strip, "s", false, "strip")
	fs.BoolVar(&strip, "strip-surrounding-newlines", false, "strip")
	fs.BoolVar(&norm, "n", false, "norm")
	fs.BoolVar(&norm, "normalize-surrounding-newlines", false, "norm")

	fs.SetOutput(io.Discard)
	_ = fs.Parse(os.Args[1:])

	args := fs.Args()
	if len(args) == 0 {
		return
	}
	text := strings.Join(args, " ")

	out, err := Render(text, font, direction, justify, width, reverse, flip, strip, norm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Stdout.WriteString(out)
}
EOF

# 4. Remove all Python files from the repository
find . -name "*.py" -delete
rm -rf pyfiglet/fonts-contrib pyfiglet/fonts-standard pyfiglet/fonts

# 5. Build to verify
go build -o /tmp/figlet .
echo "Solution applied and verified successfully."
