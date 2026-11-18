#!/bin/bash
set -euo pipefail

REPO_DIR="/workspace/repo"
if [ ! -d "$REPO_DIR" ]; then
    REPO_DIR="$(pwd)"
fi
cd "$REPO_DIR"

# 1. Organize .flf fonts into a fonts directory
mkdir -p fonts
if [ -d "pyfiglet/fonts-standard" ]; then
    cp pyfiglet/fonts-standard/*.flf fonts/ 2>/dev/null || true
fi
if [ -d "pyfiglet/fonts" ]; then
    cp pyfiglet/fonts/*.flf fonts/ 2>/dev/null || true
fi

# 2. Delete all Python files from the repository
find . -type f -name "*.py" -delete
find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true

# 3. Create go.mod
cat << 'EOF' > go.mod
module figlet

go 1.23
EOF

# 4. Create string.go
cat << 'EOF' > string.go
package main

import (
	"strings"
)

var reverseTable = [256]byte{
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

var flipTable = [256]byte{
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

func pythonSplitlines(s string) []string {
	if s == "" {
		return []string{}
	}
	var lines []string
	var cur strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\r' {
			if i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
			lines = append(lines, cur.String())
			cur.Reset()
		} else if r == '\n' {
			lines = append(lines, cur.String())
			cur.Reset()
		} else {
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

func Reverse(s string) string {
	lines := pythonSplitlines(s)
	out := make([]string, 0, len(lines))
	for _, row := range lines {
		runes := []rune(row)
		for i, r := range runes {
			if r < 256 {
				runes[i] = rune(reverseTable[r])
			}
		}
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

func Flip(s string) string {
	lines := pythonSplitlines(s)
	out := make([]string, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		row := lines[i]
		runes := []rune(row)
		for j, r := range runes {
			if r < 256 {
				runes[j] = rune(flipTable[r])
			}
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

func StripSurroundingNewlines(s string) string {
	lines := pythonSplitlines(s)
	out := make([]string, 0, len(lines))
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

func NormalizeSurroundingNewlines(s string) string {
	return "\n" + StripSurroundingNewlines(s) + "\n"
}
EOF

# 5. Create font.go
cat << 'EOF' > font.go
package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

type FigletFont struct {
	Name           string
	Height         int
	HardBlank      string
	PrintDirection int
	SmushMode      int
	Chars          map[rune][]string
	Width          map[rune]int
}

var reMagic = regexp.MustCompile(`^[tf]lf2.`)
var reEndMarker = regexp.MustCompile(`(.)\s*$`)

func readChar(lines *[]string, height int) (int, []string) {
	width := 0
	chars := make([]string, 0, height)
	var reEnd *regexp.Regexp

	for i := 0; i < height; i++ {
		if len(*lines) == 0 {
			break
		}
		line := (*lines)[0]
		*lines = (*lines)[1:]

		if reEnd == nil {
			match := reEndMarker.FindStringSubmatch(line)
			if len(match) > 1 {
				endChar := match[1]
				reEnd = regexp.MustCompile(regexp.QuoteMeta(endChar) + `{1,2}\s*$`)
			}
		}

		if reEnd != nil {
			line = reEnd.ReplaceAllString(line, "")
		}

		if len(line) > width {
			width = len(line)
		}
		chars = append(chars, line)
	}

	return width, chars
}

func parseFont(name, data string) (*FigletFont, error) {
	reLineSplit := regexp.MustCompile("[\u0085\u2028\u2029]")
	data = reLineSplit.ReplaceAllString(data, " ")

	rawLines := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, l := range rawLines {
		sub := strings.Split(l, "\r")
		lines = append(lines, sub...)
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font data for %s", name)
	}

	headerLine := lines[0]
	lines = lines[1:]

	if !reMagic.MatchString(headerLine) {
		return nil, fmt.Errorf("%s is not a valid figlet font", name)
	}

	cleanHeader := reMagic.ReplaceAllString(headerLine, "")
	fields := strings.Fields(cleanHeader)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", name)
	}

	hardBlank := fields[0]
	height, _ := strconv.Atoi(fields[1])
	oldLayout, _ := strconv.Atoi(fields[4])
	commentLines, _ := strconv.Atoi(fields[5])

	printDirection := 0
	if len(fields) > 6 {
		printDirection, _ = strconv.Atoi(fields[6])
	}

	fullLayout := -1
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
		Name:           name,
		Height:         height,
		HardBlank:      hardBlank,
		PrintDirection: printDirection,
		SmushMode:      fullLayout,
		Chars:          make(map[rune][]string),
		Width:          make(map[rune]int),
	}

	if len(lines) >= commentLines {
		lines = lines[commentLines:]
	} else {
		lines = nil
	}

	for i := 32; i < 127; i++ {
		if len(lines) < height {
			break
		}
		width, letter := readChar(&lines, height)
		if i == 32 || strings.Join(letter, "") != "" {
			font.Chars[rune(i)] = letter
			font.Width[rune(i)] = width
		}
	}

	if len(lines) >= height {
		for _, ch := range []rune("ÄÖÜäöüß") {
			if len(lines) < height {
				break
			}
			width, letter := readChar(&lines, height)
			if strings.Join(letter, "") != "" {
				font.Chars[ch] = letter
				font.Width[ch] = width
			}
		}
	}

	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		codeStr := parts[0]
		if strings.HasPrefix(strings.ToLower(codeStr), "0x") {
			code64, err := strconv.ParseInt(codeStr[2:], 16, 32)
			if err == nil {
				width, letter := readChar(&lines, height)
				if strings.Join(letter, "") != "" {
					font.Chars[rune(code64)] = letter
					font.Width[rune(code64)] = width
				}
			}
		}
	}

	return font, nil
}

func LoadFont(name string) (*FigletFont, error) {
	baseName := strings.TrimSuffix(name, ".flf")
	flfName := baseName + ".flf"

	embeddedPath := "fonts/" + flfName
	data, err := embeddedFonts.ReadFile(embeddedPath)
	if err == nil {
		return parseFont(baseName, string(data))
	}

	candidates := []string{
		name,
		flfName,
		filepath.Join("fonts", flfName),
		filepath.Join("pyfiglet", "fonts-standard", flfName),
		filepath.Join("pyfiglet", "fonts", flfName),
		filepath.Join("/workspace/repo/fonts", flfName),
		filepath.Join("/workspace/repo/pyfiglet/fonts-standard", flfName),
	}

	for _, cand := range candidates {
		dataBytes, err := os.ReadFile(cand)
		if err == nil {
			return parseFont(baseName, string(dataBytes))
		}
	}

	return nil, fmt.Errorf("font not found: %s", name)
}
EOF

# 6. Create builder.go
cat << 'EOF' > builder.go
package main

import (
	"strings"
)

const (
	SmEqual     = 1
	SmLowline   = 2
	SmHierarchy = 4
	SmPair      = 8
	SmBigX      = 16
	SmHardblank = 32
	SmKern      = 64
	SmSmush     = 128
)

type BlankMarker struct {
	Buffer   []string
	Iterator int
}

type FigletBuilder struct {
	Text              []rune
	Direction         string
	Width             int
	Font              *FigletFont
	Justify           string
	Iterator          int
	MaxSmush          int
	CurCharWidth      int
	PrevCharWidth     int
	CurrentTotalWidth int
	BlankMarkers      []BlankMarker
	Buffer            []string
	ProductQueue      [][]string
}

func NewFigletBuilder(text string, font *FigletFont, direction string, width int, justify string) *FigletBuilder {
	b := &FigletBuilder{
		Text:         []rune(text),
		Direction:    direction,
		Width:        width,
		Font:         font,
		Justify:      justify,
		Iterator:     0,
		BlankMarkers: make([]BlankMarker, 0),
		Buffer:       make([]string, font.Height),
		ProductQueue: make([][]string, 0),
	}
	for i := range b.Buffer {
		b.Buffer[i] = ""
	}
	return b
}

func (b *FigletBuilder) isNotFinished() bool {
	return b.Iterator < len(b.Text)
}

func (b *FigletBuilder) goToNextChar() {
	b.Iterator++
}

func (b *FigletBuilder) getCurChar() ([]string, bool) {
	if b.Iterator < 0 || b.Iterator >= len(b.Text) {
		return nil, false
	}
	r := b.Text[b.Iterator]
	charRows, ok := b.Font.Chars[r]
	return charRows, ok
}

func (b *FigletBuilder) getCurWidth() int {
	if b.Iterator < 0 || b.Iterator >= len(b.Text) {
		return 0
	}
	r := b.Text[b.Iterator]
	return b.Font.Width[r]
}

func (b *FigletBuilder) addCharToProduct() {
	curChar, ok := b.getCurChar()

	if b.Iterator < len(b.Text) && b.Text[b.Iterator] == '\n' {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: bufCopy, Iterator: b.Iterator})
		b.handleNewLine()
		return
	}

	if !ok {
		return
	}

	curWidth := b.getCurWidth()
	if b.Width < curWidth {
		return
	}

	b.CurCharWidth = curWidth
	b.MaxSmush = b.smushAmount(b.Buffer, curChar)
	b.CurrentTotalWidth = len(b.Buffer[0]) + b.CurCharWidth - b.MaxSmush

	if b.Text[b.Iterator] == ' ' {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: bufCopy, Iterator: b.Iterator})
	}

	if b.CurrentTotalWidth >= b.Width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.Font.Height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}

	b.PrevCharWidth = b.CurCharWidth
}

func (b *FigletBuilder) cutBufferCommon() {
	b.CurrentTotalWidth = 0
	b.Buffer = make([]string, b.Font.Height)
	for i := range b.Buffer {
		b.Buffer[i] = ""
	}
	b.BlankMarkers = nil
	b.PrevCharWidth = 0
	curChar, ok := b.getCurChar()
	if !ok {
		return
	}
	b.MaxSmush = b.smushAmount(b.Buffer, curChar)
}

func (b *FigletBuilder) cutBufferAtLastBlank(savedBuffer []string, savedIterator int) {
	b.ProductQueue = append(b.ProductQueue, savedBuffer)
	b.Iterator = savedIterator
	b.cutBufferCommon()
}

func (b *FigletBuilder) cutBufferAtLastChar() {
	bufCopy := make([]string, len(b.Buffer))
	copy(bufCopy, b.Buffer)
	b.ProductQueue = append(b.ProductQueue, bufCopy)
	b.Iterator--
	b.cutBufferCommon()
}

func (b *FigletBuilder) getLastBlank() (BlankMarker, bool) {
	if len(b.BlankMarkers) == 0 {
		return BlankMarker{Iterator: -1}, false
	}
	bm := b.BlankMarkers[len(b.BlankMarkers)-1]
	b.BlankMarkers = b.BlankMarkers[:len(b.BlankMarkers)-1]
	return bm, true
}

func (b *FigletBuilder) handleNewLine() {
	bm, ok := b.getLastBlank()
	if ok && bm.Iterator != -1 {
		b.cutBufferAtLastBlank(bm.Buffer, bm.Iterator)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *FigletBuilder) smushRow(curChar []string, row int) (string, string) {
	addLeft := b.Buffer[row]
	addRight := curChar[row]

	if b.Direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for i := 0; i < b.MaxSmush; i++ {
		left, idx := b.getLeftSmushedChar(i, addLeft)
		right := ""
		if i < len(addRight) {
			right = string(addRight[i])
		}
		smushed := b.smushChars(left, right)
		if smushed != "" {
			addLeft = b.updateSmushedCharInLeftBuffer(addLeft, idx, smushed)
		}
	}
	return addLeft, addRight
}

func (b *FigletBuilder) getLeftSmushedChar(i int, addLeft string) (string, int) {
	idx := len(addLeft) - b.MaxSmush + i
	if idx >= 0 && idx < len(addLeft) {
		return string(addLeft[idx]), idx
	}
	return "", idx
}

func (b *FigletBuilder) updateSmushedCharInLeftBuffer(addLeft string, idx int, smushed string) string {
	if idx < 0 || idx >= len(addLeft) {
		return addLeft
	}
	bSlice := []byte(addLeft)
	bSlice[idx] = smushed[0]
	return string(bSlice)
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	rem := ""
	if b.MaxSmush < len(addRight) {
		rem = addRight[b.MaxSmush:]
	}
	b.Buffer[row] = addLeft + rem
}

func (b *FigletBuilder) smushAmount(buffer []string, curChar []string) int {
	if (b.Font.SmushMode & (SmSmush | SmKern)) == 0 {
		return 0
	}

	maxSmush := b.CurCharWidth
	for row := 0; row < b.Font.Height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if b.Direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		trimmedLeft := strings.TrimRight(lineLeft, " ")
		linebd := len(trimmedLeft) - 1
		if linebd < 0 {
			linebd = 0
		}

		ch1 := ""
		if linebd < len(lineLeft) {
			ch1 = string(lineLeft[linebd])
		} else {
			linebd = 0
			ch1 = ""
		}

		trimmedRight := strings.TrimLeft(lineRight, " ")
		charbd := len(lineRight) - len(trimmedRight)
		ch2 := ""
		if charbd < len(lineRight) {
			ch2 = string(lineRight[charbd])
		} else {
			charbd = len(lineRight)
			ch2 = ""
		}

		amt := charbd + len(lineLeft) - 1 - linebd

		if ch1 == "" || ch1 == " " {
			amt++
		} else if ch2 != "" && b.smushChars(ch1, ch2) != "" {
			amt++
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

func (b *FigletBuilder) smushChars(left, right string) string {
	if left == " " {
		return right
	}
	if right == " " {
		return left
	}

	if b.PrevCharWidth < 2 || b.CurCharWidth < 2 {
		return ""
	}

	if (b.Font.SmushMode & SmSmush) == 0 {
		return ""
	}

	if (b.Font.SmushMode & 63) == 0 {
		if left == b.Font.HardBlank {
			return right
		}
		if right == b.Font.HardBlank {
			return left
		}
		if b.Direction == "right-to-left" {
			return left
		}
		return right
	}

	if (b.Font.SmushMode & SmHardblank) != 0 {
		if left == b.Font.HardBlank && right == b.Font.HardBlank {
			return left
		}
	}

	if left == b.Font.HardBlank || right == b.Font.HardBlank {
		return ""
	}

	if (b.Font.SmushMode & SmEqual) != 0 {
		if left == right {
			return left
		}
	}

	type smushPair struct {
		a, b string
	}
	var smushes []smushPair

	if (b.Font.SmushMode & SmLowline) != 0 {
		smushes = append(smushes, smushPair{a: "_", b: `|/\[]{}()<>`})
	}

	if (b.Font.SmushMode & SmHierarchy) != 0 {
		smushes = append(smushes,
			smushPair{a: "|", b: `/\\[]{}()<>`},
			smushPair{a: `\/`, b: `[]{}()<>`},
			smushPair{a: "[]", b: "{}()<>"},
			smushPair{a: "{}", b: "()<>"},
			smushPair{a: "()", b: "<>"},
		)
	}

	for _, sp := range smushes {
		if strings.Contains(sp.a, left) && strings.Contains(sp.b, right) {
			return right
		}
		if strings.Contains(sp.b, left) && strings.Contains(sp.a, right) {
			return left
		}
	}

	if (b.Font.SmushMode & SmPair) != 0 {
		for _, pair := range []string{left + right, right + left} {
			if pair == "[]" || pair == "{}" || pair == "()" {
				return "|"
			}
		}
	}

	if (b.Font.SmushMode & SmBigX) != 0 {
		if left == "/" && right == "\\" {
			return "|"
		}
		if left == "\\" && right == "/" {
			return "Y"
		}
		if left == ">" && right == "<" {
			return "X"
		}
	}

	return ""
}

func (b *FigletBuilder) returnProduct() string {
	if len(b.Buffer) > 0 && b.Buffer[0] != "" {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.ProductQueue = append(b.ProductQueue, bufCopy)
	}

	var sb strings.Builder
	for _, buf := range b.ProductQueue {
		justified := b.justifyString(b.Justify, buf)
		line := strings.Join(justified, "\n") + "\n"
		line = strings.ReplaceAll(line, b.Font.HardBlank, " ")
		sb.WriteString(line)
	}
	return sb.String()
}

func (b *FigletBuilder) justifyString(justify string, buffer []string) []string {
	res := make([]string, len(buffer))
	copy(res, buffer)
	if justify == "right" {
		for row := 0; row < b.Font.Height; row++ {
			pad := b.Width - len(res[row]) - 1
			if pad > 0 {
				res[row] = strings.Repeat(" ", pad) + res[row]
			}
		}
	} else if justify == "center" {
		for row := 0; row < b.Font.Height; row++ {
			pad := (b.Width - len(res[row])) / 2
			if pad > 0 {
				res[row] = strings.Repeat(" ", pad) + res[row]
			}
		}
	}
	return res
}
EOF

# 7. Create main.go
cat << 'EOF' > main.go
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	var font = "standard"
	var width = 80
	var justify = "auto"
	var direction = "auto"
	var reverse = false
	var flip = false
	var strip = false
	var normalize = false
	var positional []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if arg == "-f" || arg == "--font" {
			if i+1 < len(args) {
				i++
				font = args[i]
			}
		} else if strings.HasPrefix(arg, "-f=") || strings.HasPrefix(arg, "--font=") {
			font = strings.SplitN(arg, "=", 2)[1]
		} else if arg == "-w" || arg == "--width" {
			if i+1 < len(args) {
				i++
				w, err := strconv.Atoi(args[i])
				if err == nil {
					width = w
				}
			}
		} else if strings.HasPrefix(arg, "-w=") || strings.HasPrefix(arg, "--width=") {
			w, err := strconv.Atoi(strings.SplitN(arg, "=", 2)[1])
			if err == nil {
				width = w
			}
		} else if arg == "-j" || arg == "--justify" {
			if i+1 < len(args) {
				i++
				justify = args[i]
			}
		} else if strings.HasPrefix(arg, "-j=") || strings.HasPrefix(arg, "--justify=") {
			justify = strings.SplitN(arg, "=", 2)[1]
		} else if arg == "-D" || arg == "--direction" {
			if i+1 < len(args) {
				i++
				direction = args[i]
			}
		} else if strings.HasPrefix(arg, "-D=") || strings.HasPrefix(arg, "--direction=") {
			direction = strings.SplitN(arg, "=", 2)[1]
		} else if arg == "-r" || arg == "--reverse" {
			reverse = true
		} else if arg == "-F" || arg == "--flip" {
			flip = true
		} else if arg == "-s" || arg == "--strip-surrounding-newlines" {
			strip = true
		} else if arg == "-n" || arg == "--normalize-surrounding-newlines" {
			normalize = true
		} else {
			positional = append(positional, arg)
		}
	}

	if len(positional) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [text..]\n", os.Args[0])
		os.Exit(1)
	}

	text := strings.Join(positional, " ")

	fontObj, err := LoadFont(font)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: requested font %q not found.\n", font)
		os.Exit(1)
	}

	dir := direction
	if dir == "auto" {
		if fontObj.PrintDirection == 1 {
			dir = "right-to-left"
		} else {
			dir = "left-to-right"
		}
	}

	jst := justify
	if jst == "auto" {
		if dir == "right-to-left" {
			jst = "right"
		} else {
			jst = "left"
		}
	}

	builder := NewFigletBuilder(text, fontObj, dir, width, jst)
	for builder.isNotFinished() {
		builder.addCharToProduct()
		builder.goToNextChar()
	}

	r := builder.returnProduct()
	if reverse {
		r = Reverse(r)
	}
	if flip {
		r = Flip(r)
	}
	if strip {
		r = StripSurroundingNewlines(r)
	} else if normalize {
		r = NormalizeSurroundingNewlines(r)
	}

	os.Stdout.WriteString(r)
	os.Stdout.WriteString("\n")
}
EOF

# 8. Build the binary to verify compilation
go build -o /tmp/figlet .
echo "Successfully built /tmp/figlet in $REPO_DIR"
