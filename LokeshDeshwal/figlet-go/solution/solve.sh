#!/bin/bash
set -e

cd /workspace/repo

# 1. Prepare embedded fonts directory
mkdir -p /workspace/repo/fonts
if [ -d "/workspace/repo/pyfiglet/fonts" ]; then
    cp /workspace/repo/pyfiglet/fonts/*.flf /workspace/repo/fonts/
fi

# 2. Write go.mod
cat << 'EOF' > /workspace/repo/go.mod
module figlet

go 1.23
EOF

# 3. Write main.go (Go standard library FIGlet port)
cat << 'EOF' > /workspace/repo/main.go
package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

// Reverse translation map
var reverseMap = [256]byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
	' ', '!', '"', '#', '$', '%', '&', '\'', ')', '(', '*', '+', ',', '-', '.', '\\',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ':', ';', '>', '=', '<', '?',
	'@', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O',
	'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', ']', '/', '[', '^', '_',
	'`', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', '}', '|', '{', '~', 127,
}

// Flip translation map
var flipMap = [256]byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
	' ', '!', '"', '#', '$', '%', '&', '\'', '(', ')', '*', '+', ',', '-', '.', '\\',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ':', ';', '<', '=', '>', '?',
	'@', 'V', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'W', 'N', 'O',
	'b', 'Q', 'b', 'S', 'T', 'U', 'A', 'M', 'X', 'Y', 'Z', '[', '/', ']', 'v', '-',
	'`', 'a', 'P', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'w', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', '^', 'm', 'x', 'y', 'z', '{', '|', '}', '~', 127,
}

func init() {
	for i := 128; i < 256; i++ {
		reverseMap[i] = byte(i)
		flipMap[i] = byte(i)
	}
}

type FigletFont struct {
	Name           string
	Height         int
	HardBlank      string
	PrintDirection int
	SmushMode      int
	Chars          map[int][]string
	Width          map[int]int
}

var (
	reMagicNumber = regexp.MustCompile(`^[tf]lf2.`)
	reEndMarker   = regexp.MustCompile(`(.)\s*$`)
)

func LoadFont(name string) (*FigletFont, error) {
	filename := name
	if !strings.HasSuffix(filename, ".flf") && !strings.HasSuffix(filename, ".tlf") {
		filename += ".flf"
	}

	dataBytes, err := embeddedFonts.ReadFile("fonts/" + filename)
	if err != nil {
		dataBytes, err = os.ReadFile(filepath.Join("/workspace/repo/fonts", filename))
		if err != nil {
			dataBytes, err = os.ReadFile(filename)
			if err != nil {
				return nil, fmt.Errorf("font not found: %s", name)
			}
		}
	}

	dataStr := string(dataBytes)
	reLineSplit := regexp.MustCompile(`[\x{0085}\x{2028}\x{2029}]`)
	dataStr = reLineSplit.ReplaceAllString(dataStr, " ")

	lines := strings.Split(dataStr, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font file")
	}

	header := lines[0]
	lines = lines[1:]
	if !reMagicNumber.MatchString(header) {
		return nil, fmt.Errorf("invalid font magic number: %s", name)
	}

	header = reMagicNumber.ReplaceAllString(header, "")
	headerParts := strings.Fields(header)
	if len(headerParts) < 6 {
		return nil, fmt.Errorf("malformed header: %s", name)
	}

	hardBlank := headerParts[0]
	height, _ := strconv.Atoi(headerParts[1])
	oldLayout, _ := strconv.Atoi(headerParts[4])
	commentLines, _ := strconv.Atoi(headerParts[5])

	printDirection := 0
	if len(headerParts) > 6 {
		printDirection, _ = strconv.Atoi(headerParts[6])
	}

	var fullLayout *int
	if len(headerParts) > 7 {
		val, _ := strconv.Atoi(headerParts[7])
		fullLayout = &val
	}

	smushMode := 0
	if fullLayout == nil {
		if oldLayout == 0 {
			smushMode = 64
		} else if oldLayout < 0 {
			smushMode = 0
		} else {
			smushMode = (oldLayout & 31) | 128
		}
	} else {
		smushMode = *fullLayout
	}

	if len(lines) < commentLines {
		return nil, fmt.Errorf("font truncated in comments")
	}
	lines = lines[commentLines:]

	ff := &FigletFont{
		Name:           name,
		Height:         height,
		HardBlank:      hardBlank,
		PrintDirection: printDirection,
		SmushMode:      smushMode,
		Chars:          make(map[int][]string),
		Width:          make(map[int]int),
	}

	readChar := func() (int, []string) {
		if len(lines) < height {
			return 0, nil
		}
		var endStr string
		var endRe *regexp.Regexp
		maxW := 0
		charRows := make([]string, height)

		for row := 0; row < height; row++ {
			line := lines[0]
			lines = lines[1:]

			if endRe == nil {
				m := reEndMarker.FindStringSubmatch(line)
				if len(m) > 1 {
					endStr = m[1]
					endRe = regexp.MustCompile(regexp.QuoteMeta(endStr) + `{1,2}\s*$`)
				}
			}

			if endRe != nil {
				line = endRe.ReplaceAllString(line, "")
			}
			if len(line) > maxW {
				maxW = len(line)
			}
			charRows[row] = line
		}
		return maxW, charRows
	}

	// ASCII 32 to 126
	for i := 32; i <= 126; i++ {
		w, letter := readChar()
		allEmpty := true
		for _, r := range letter {
			if r != "" {
				allEmpty = false
				break
			}
		}
		if i == 32 || !allEmpty {
			ff.Chars[i] = letter
			ff.Width[i] = w
		}
	}

	// German Umlauts: Ä Ö Ü ä ö ü ß
	germanUmlauts := []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'}
	for _, ch := range germanUmlauts {
		if len(lines) < height {
			break
		}
		w, letter := readChar()
		allEmpty := true
		for _, r := range letter {
			if r != "" {
				allEmpty = false
				break
			}
		}
		if !allEmpty {
			ff.Chars[int(ch)] = letter
			ff.Width[int(ch)] = w
		}
	}

	// Extended character set
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		codeStr := parts[0]
		if strings.HasPrefix(strings.ToLower(codeStr), "0x") {
			code, err := strconv.ParseInt(codeStr[2:], 16, 64)
			if err == nil {
				w, letter := readChar()
				allEmpty := true
				for _, r := range letter {
					if r != "" {
						allEmpty = false
						break
					}
				}
				if !allEmpty {
					ff.Chars[int(code)] = letter
					ff.Width[int(code)] = w
				}
			}
		}
	}

	return ff, nil
}

// Smush mode constants
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

type BlankMarker struct {
	Buffer   []string
	Iterator int
}

type FigletBuilder struct {
	Text            []rune
	Font            *FigletFont
	Direction       string
	Width           int
	Justify         string
	Iterator        int
	MaxSmush        int
	CurCharWidth    int
	PrevCharWidth   int
	CurrentTotalW   int
	BlankMarkers    []BlankMarker
	Buffer          []string
	ProductQueue    [][]string
	HardBlank       byte
}

func NewFigletBuilder(text string, font *FigletFont, direction string, width int, justify string) *FigletBuilder {
	hb := byte('$')
	if len(font.HardBlank) > 0 {
		hb = font.HardBlank[0]
	}
	b := &FigletBuilder{
		Text:          []rune(text),
		Font:          font,
		Direction:     direction,
		Width:         width,
		Justify:       justify,
		Iterator:      0,
		BlankMarkers:  make([]BlankMarker, 0),
		Buffer:        make([]string, font.Height),
		ProductQueue:  make([][]string, 0),
		HardBlank:     hb,
	}
	return b
}

func (b *FigletBuilder) IsNotFinished() bool {
	return b.Iterator < len(b.Text)
}

func (b *FigletBuilder) GoToNextChar() {
	b.Iterator++
}

func (b *FigletBuilder) GetCurChar() []string {
	if b.Iterator < 0 || b.Iterator >= len(b.Text) {
		return nil
	}
	c := int(b.Text[b.Iterator])
	if rows, ok := b.Font.Chars[c]; ok {
		return rows
	}
	return nil
}

func (b *FigletBuilder) GetCurWidth() int {
	if b.Iterator < 0 || b.Iterator >= len(b.Text) {
		return 0
	}
	c := int(b.Text[b.Iterator])
	if w, ok := b.Font.Width[c]; ok {
		return w
	}
	return 0
}

func (b *FigletBuilder) SmushChars(left, right byte) (byte, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}

	if b.PrevCharWidth < 2 || b.CurCharWidth < 2 {
		return 0, false
	}

	if (b.Font.SmushMode & SM_SMUSH) == 0 {
		return 0, false
	}

	// Universal overlapping
	if (b.Font.SmushMode & 63) == 0 {
		if left == b.HardBlank {
			return right, true
		}
		if right == b.HardBlank {
			return left, true
		}
		if b.Direction == "right-to-left" {
			return left, true
		}
		return right, true
	}

	if (b.Font.SmushMode & SM_HARDBLANK) != 0 {
		if left == b.HardBlank && right == b.HardBlank {
			return left, true
		}
	}

	if left == b.HardBlank || right == b.HardBlank {
		return 0, false
	}

	// Equal characters
	if (b.Font.SmushMode & SM_EQUAL) != 0 {
		if left == right {
			return left, true
		}
	}

	// Underscore / lowline
	if (b.Font.SmushMode & SM_LOWLINE) != 0 {
		target := "|/\\[]{}()<>"
		if left == '_' && strings.IndexByte(target, right) >= 0 {
			return right, true
		}
		if right == '_' && strings.IndexByte(target, left) >= 0 {
			return left, true
		}
	}

	// Hierarchy
	if (b.Font.SmushMode & SM_HIERARCHY) != 0 {
		hierarchy := [6]string{
			"|",
			"/\\",
			"[]",
			"{}",
			"()",
			"<>",
		}
		getClass := func(c byte) int {
			for idx, set := range hierarchy {
				if strings.IndexByte(set, c) >= 0 {
					return idx
				}
			}
			return -1
		}
		c1 := getClass(left)
		c2 := getClass(right)
		if c1 >= 0 && c2 >= 0 && c1 != c2 {
			if c1 > c2 {
				return left, true
			}
			return right, true
		}
	}

	// Opposite pairs
	if (b.Font.SmushMode & SM_PAIR) != 0 {
		pair1 := string([]byte{left, right})
		pair2 := string([]byte{right, left})
		if pair1 == "[]" || pair1 == "{}" || pair1 == "()" ||
			pair2 == "[]" || pair2 == "{}" || pair2 == "()" {
			return '|', true
		}
	}

	// Big X
	if (b.Font.SmushMode & SM_BIGX) != 0 {
		if left == '/' && right == '\\' {
			return '|', true
		}
		if left == '\\' && right == '/' {
			return 'Y', true
		}
		if left == '>' && right == '<' {
			return 'X', true
		}
	}

	return 0, false
}

func (b *FigletBuilder) SmushAmount(curChar []string) int {
	if (b.Font.SmushMode & (SM_SMUSH | SM_KERN)) == 0 {
		return 0
	}

	maxSmush := b.CurCharWidth
	for row := 0; row < b.Font.Height; row++ {
		lineLeft := b.Buffer[row]
		lineRight := curChar[row]
		if b.Direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		trimmedLeft := strings.TrimRight(lineLeft, " ")
		linebd := len(trimmedLeft) - 1
		if linebd < 0 {
			linebd = 0
		}

		var ch1 byte = ' '
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
			ch1 = 0
		}

		trimmedRight := strings.TrimLeft(lineRight, " ")
		charbd := len(lineRight) - len(trimmedRight)
		var ch2 byte = ' '
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
			ch2 = 0
		}

		amt := charbd + len(lineLeft) - 1 - linebd

		if ch1 == 0 || ch1 == ' ' {
			amt++
		} else if ch2 != 0 {
			if _, ok := b.SmushChars(ch1, ch2); ok {
				amt++
			}
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}

	return maxSmush
}

func (b *FigletBuilder) SmushRow(curChar []string, row int) (string, string) {
	addLeft := b.Buffer[row]
	addRight := curChar[row]

	if b.Direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	addLeftBytes := []byte(addLeft)
	for i := 0; i < b.MaxSmush; i++ {
		idx := len(addLeft) - b.MaxSmush + i
		var leftChar byte = ' '
		if idx >= 0 && idx < len(addLeft) {
			leftChar = addLeft[idx]
		}
		var rightChar byte = ' '
		if i < len(addRight) {
			rightChar = addRight[i]
		}

		if smushed, ok := b.SmushChars(leftChar, rightChar); ok {
			if idx >= 0 && idx < len(addLeftBytes) {
				addLeftBytes[idx] = smushed
			}
		}
	}
	return string(addLeftBytes), addRight
}

func (b *FigletBuilder) AddCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.SmushRow(curChar, row)
	rightRem := ""
	if b.MaxSmush < len(addRight) {
		rightRem = addRight[b.MaxSmush:]
	}
	b.Buffer[row] = addLeft + rightRem
}

func (b *FigletBuilder) CutBufferCommon() {
	b.CurrentTotalW = 0
	b.Buffer = make([]string, b.Font.Height)
	b.BlankMarkers = make([]BlankMarker, 0)
	b.PrevCharWidth = 0
	curChar := b.GetCurChar()
	if curChar == nil {
		return
	}
	b.MaxSmush = b.SmushAmount(curChar)
}

func (b *FigletBuilder) CutBufferAtLastBlank(savedBuf []string, savedIterator int) {
	bufCopy := make([]string, len(savedBuf))
	copy(bufCopy, savedBuf)
	b.ProductQueue = append(b.ProductQueue, bufCopy)
	b.Iterator = savedIterator
	b.CutBufferCommon()
}

func (b *FigletBuilder) CutBufferAtLastChar() {
	bufCopy := make([]string, len(b.Buffer))
	copy(bufCopy, b.Buffer)
	b.ProductQueue = append(b.ProductQueue, bufCopy)
	b.Iterator--
	b.CutBufferCommon()
}

func (b *FigletBuilder) HandleNewLine() {
	if len(b.BlankMarkers) > 0 {
		last := b.BlankMarkers[len(b.BlankMarkers)-1]
		b.BlankMarkers = b.BlankMarkers[:len(b.BlankMarkers)-1]
		if last.Iterator != -1 {
			b.CutBufferAtLastBlank(last.Buffer, last.Iterator)
			return
		}
	}
	b.CutBufferAtLastChar()
}

func (b *FigletBuilder) AddCharToProduct() {
	if b.Text[b.Iterator] == '\n' {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: bufCopy, Iterator: b.Iterator})
		b.HandleNewLine()
		return
	}

	curChar := b.GetCurChar()
	if curChar == nil {
		return
	}

	b.CurCharWidth = b.GetCurWidth()
	b.MaxSmush = b.SmushAmount(curChar)
	b.CurrentTotalW = len(b.Buffer[0]) + b.CurCharWidth - b.MaxSmush

	if b.Text[b.Iterator] == ' ' {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.BlankMarkers = append(b.BlankMarkers, BlankMarker{Buffer: bufCopy, Iterator: b.Iterator})
	}

	if b.CurrentTotalW >= b.Width {
		b.HandleNewLine()
	} else {
		for row := 0; row < b.Font.Height; row++ {
			b.AddCurCharRowToBufferRow(curChar, row)
		}
	}

	b.PrevCharWidth = b.CurCharWidth
}

func (b *FigletBuilder) JustifyBuffer(buffer []string) []string {
	result := make([]string, len(buffer))
	copy(result, buffer)

	if b.Justify == "right" {
		for row := 0; row < b.Font.Height; row++ {
			pad := b.Width - len(result[row]) - 1
			if pad > 0 {
				result[row] = strings.Repeat(" ", pad) + result[row]
			}
		}
	} else if b.Justify == "center" {
		for row := 0; row < b.Font.Height; row++ {
			pad := (b.Width - len(result[row])) / 2
			if pad > 0 {
				result[row] = strings.Repeat(" ", pad) + result[row]
			}
		}
	}
	return result
}

func (b *FigletBuilder) ReplaceHardblanks(buffer []string) string {
	res := strings.Join(buffer, "\n") + "\n"
	if b.Font.HardBlank != "" {
		res = strings.ReplaceAll(res, b.Font.HardBlank, " ")
	}
	return res
}

func (b *FigletBuilder) ReturnProduct() string {
	if b.Buffer[0] != "" {
		bufCopy := make([]string, len(b.Buffer))
		copy(bufCopy, b.Buffer)
		b.ProductQueue = append(b.ProductQueue, bufCopy)
	}

	var acc strings.Builder
	for _, buf := range b.ProductQueue {
		justified := b.JustifyBuffer(buf)
		acc.WriteString(b.ReplaceHardblanks(justified))
	}
	return acc.String()
}

func ReverseFiglet(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out []string
	for _, row := range lines {
		rowBytes := []byte(row)
		for i, c := range rowBytes {
			rowBytes[i] = reverseMap[c]
		}
		for i, j := 0, len(rowBytes)-1; i < j; i, j = i+1, j-1 {
			rowBytes[i], rowBytes[j] = rowBytes[j], rowBytes[i]
		}
		out = append(out, string(rowBytes))
	}
	return strings.Join(out, "\n") + "\n"
}

func FlipFiglet(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		rowBytes := []byte(lines[i])
		for j, c := range rowBytes {
			rowBytes[j] = flipMap[c]
		}
		out = append(out, string(rowBytes))
	}
	return strings.Join(out, "\n") + "\n"
}

func StripSurroundingNewlines(s string) string {
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

func NormalizeSurroundingNewlines(s string) string {
	return "\n" + StripSurroundingNewlines(s) + "\n"
}

func Render(text string, fontName string, direction string, justify string, width int, reverse bool, flip bool, strip bool, norm bool) (string, error) {
	font, err := LoadFont(fontName)
	if err != nil {
		return "", err
	}

	actualDirection := direction
	if actualDirection == "auto" {
		if font.PrintDirection == 0 {
			actualDirection = "left-to-right"
		} else if font.PrintDirection == 1 {
			actualDirection = "right-to-left"
		} else {
			actualDirection = "left-to-right"
		}
	}

	actualJustify := justify
	if actualJustify == "auto" {
		if actualDirection == "left-to-right" {
			actualJustify = "left"
		} else if actualDirection == "right-to-left" {
			actualJustify = "right"
		}
	}

	builder := NewFigletBuilder(text, font, actualDirection, width, actualJustify)
	for builder.IsNotFinished() {
		builder.AddCharToProduct()
		builder.GoToNextChar()
	}

	r := builder.ReturnProduct()

	if reverse {
		r = ReverseFiglet(r)
	}
	if flip {
		r = FlipFiglet(r)
	}
	if strip {
		r = StripSurroundingNewlines(r)
	} else if norm {
		r = NormalizeSurroundingNewlines(r)
	}

	return r, nil
}

func main() {
	fontFlag := flag.String("f", "standard", "font")
	widthFlag := flag.Int("w", 80, "width")
	justifyFlag := flag.String("j", "auto", "justify")
	directionFlag := flag.String("D", "auto", "direction")
	reverseFlag := flag.Bool("r", false, "reverse")
	flipFlag := flag.Bool("F", false, "flip")
	stripFlag := flag.Bool("s", false, "strip")
	normFlag := flag.Bool("n", false, "norm")

	flag.Parse()

	text := strings.Join(flag.Args(), " ")
	out, err := Render(text, *fontFlag, *directionFlag, *justifyFlag, *widthFlag, *reverseFlag, *flipFlag, *stripFlag, *normFlag)
	if err != nil {
		os.Exit(1)
	}

	fmt.Print(out)
	fmt.Print("\n")
}
EOF

# 4. Remove all Python files from repository
find /workspace/repo -name "*.py" -delete
rm -rf /workspace/repo/pyfiglet
rm -rf /workspace/repo/tools /workspace/repo/doc /workspace/repo/.github

echo "Oracle solve complete: Go FIGlet twin ready, Python files deleted."
