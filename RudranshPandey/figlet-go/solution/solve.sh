#!/bin/bash
set -e

cd /workspace/repo

# Create fonts directory and copy fonts
mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/

# Write Go module file
cat <<'EOF' > go.mod
module figlet

go 1.23
EOF

# Write main.go
cat <<'EOF' > main.go
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
	"unicode"
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

const (
	SMEQUAL     = 1
	SMLOWLINE   = 2
	SMHIERARCHY = 4
	SMPAIR      = 8
	SMBIGX      = 16
	SMHARDBLANK = 32
	SMKERN      = 64
	SMSMUSH     = 128
)

type FigletFont struct {
	Name           string
	Height         int
	HardBlank      string
	PrintDirection int
	SmushMode      int
	Comment        string
	Chars          map[int][]string
	Width          map[int]int
}

var reMagic = regexp.MustCompile(`^[tf]lf2.`)
var reEndMarker = regexp.MustCompile(`(.)\s*$`)

func LoadFont(fontName string) (*FigletFont, error) {
	var raw []byte
	var err error

	embedPath := "fonts/" + fontName + ".flf"
	raw, err = embeddedFonts.ReadFile(embedPath)
	if err != nil {
		paths := []string{
			fontName + ".flf",
			filepath.Join("pyfiglet/fonts", fontName+".flf"),
			filepath.Join("/workspace/repo/pyfiglet/fonts", fontName+".flf"),
		}
		for _, p := range paths {
			if data, errRead := os.ReadFile(p); errRead == nil {
				raw = data
				err = nil
				break
			}
		}
	}
	if err != nil || len(raw) == 0 {
		return nil, fmt.Errorf("font %s not found", fontName)
	}

	content := string(raw)
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

	if !reMagic.MatchString(headerLine) {
		return nil, fmt.Errorf("invalid header")
	}
	headerClean := reMagic.ReplaceAllString(headerLine, "")
	fields := strings.Fields(headerClean)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header")
	}

	hardBlank := fields[0]
	height, _ := strconv.Atoi(fields[1])
	oldLayout, _ := strconv.Atoi(fields[4])
	commentLines, _ := strconv.Atoi(fields[5])

	printDir := 0
	if len(fields) > 6 {
		printDir, _ = strconv.Atoi(fields[6])
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
		Name:           fontName,
		Height:         height,
		HardBlank:      hardBlank,
		PrintDirection: printDir,
		SmushMode:      fullLayout,
		Chars:          make(map[int][]string),
		Width:          make(map[int]int),
	}

	for i := 0; i < commentLines && len(lines) > 0; i++ {
		font.Comment += lines[0]
		lines = lines[1:]
	}

	parseChar := func() (int, []string) {
		w := 0
		var charLines []string
		var endMarker string
		for r := 0; r < height && len(lines) > 0; r++ {
			line := lines[0]
			lines = lines[1:]
			if endMarker == "" {
				m := reEndMarker.FindStringSubmatch(line)
				if len(m) > 1 {
					endMarker = m[1]
				}
			}
			if endMarker != "" {
				reEnd := regexp.MustCompile(regexp.QuoteMeta(endMarker) + `{1,2}\s*$`)
				line = reEnd.ReplaceAllString(line, "")
			}
			if len(line) > w {
				w = len(line)
			}
			charLines = append(charLines, line)
		}
		return w, charLines
	}

	for i := 32; i < 127; i++ {
		w, letter := parseChar()
		if i == 32 || strings.Join(letter, "") != "" {
			font.Chars[i] = letter
			font.Width[i] = w
		}
	}

	// German Umlaute: ÄÖÜäöüß
	german := []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'}
	for _, r := range german {
		if len(lines) == 0 {
			break
		}
		w, letter := parseChar()
		if strings.Join(letter, "") != "" {
			font.Chars[int(r)] = letter
			font.Width[int(r)] = w
		}
	}

	// Extended characters
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		codeStr := parts[0]
		if strings.HasPrefix(strings.ToLower(codeStr), "0x") {
			code64, errHex := strconv.ParseInt(codeStr[2:], 16, 32)
			if errHex == nil {
				w, letter := parseChar()
				if strings.Join(letter, "") != "" {
					font.Chars[int(code64)] = letter
					font.Width[int(code64)] = w
				}
			}
		}
	}

	return font, nil
}

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
	Queue             [][]string
	Buffer            []string
}

func NewFigletBuilder(text string, font *FigletFont, direction string, width int, justify string) *FigletBuilder {
	return &FigletBuilder{
		Text:      []rune(text),
		Direction: direction,
		Width:     width,
		Font:      font,
		Justify:   justify,
		Buffer:    make([]string, font.Height),
	}
}

func (fb *FigletBuilder) getCurChar() []string {
	if fb.Iterator < 0 || fb.Iterator >= len(fb.Text) {
		return nil
	}
	r := fb.Text[fb.Iterator]
	return fb.Font.Chars[int(r)]
}

func (fb *FigletBuilder) getCurWidth() int {
	if fb.Iterator < 0 || fb.Iterator >= len(fb.Text) {
		return 0
	}
	r := fb.Text[fb.Iterator]
	return fb.Font.Width[int(r)]
}

func (fb *FigletBuilder) smushChars(left, right string) (string, bool) {
	if left == " " {
		return right, true
	}
	if right == " " {
		return left, true
	}
	if fb.PrevCharWidth < 2 || fb.CurCharWidth < 2 {
		return "", false
	}
	if (fb.Font.SmushMode & SMSMUSH) == 0 {
		return "", false
	}
	if (fb.Font.SmushMode & 63) == 0 {
		if left == fb.Font.HardBlank {
			return right, true
		}
		if right == fb.Font.HardBlank {
			return left, true
		}
		if fb.Direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if (fb.Font.SmushMode & SMHARDBLANK) != 0 {
		if left == fb.Font.HardBlank && right == fb.Font.HardBlank {
			return left, true
		}
	}
	if left == fb.Font.HardBlank || right == fb.Font.HardBlank {
		return "", false
	}
	if (fb.Font.SmushMode & SMEQUAL) != 0 {
		if left == right {
			return left, true
		}
	}
	if (fb.Font.SmushMode & SMLOWLINE) != 0 {
		if left == "_" && strings.ContainsRune("|/\\[]{}()<>", rune(right[0])) {
			return right, true
		}
		if right == "_" && strings.ContainsRune("|/\\[]{}()<>", rune(left[0])) {
			return left, true
		}
	}
	if (fb.Font.SmushMode & SMHIERARCHY) != 0 {
		hierarchy := []struct {
			a string
			b string
		}{
			{"|", "/\\[]{}()<>"},
			{"/\\", "[]{}()<>"},
			{"[]", "{}()<>"},
			{"{}", "()<>"},
			{"()", "<>"},
		}
		for _, h := range hierarchy {
			if strings.ContainsRune(h.a, rune(left[0])) && strings.ContainsRune(h.b, rune(right[0])) {
				return right, true
			}
			if strings.ContainsRune(h.a, rune(right[0])) && strings.ContainsRune(h.b, rune(left[0])) {
				return left, true
			}
		}
	}
	if (fb.Font.SmushMode & SMPAIR) != 0 {
		p1 := left + right
		p2 := right + left
		if p1 == "[]" || p1 == "{}" || p1 == "()" || p2 == "[]" || p2 == "{}" || p2 == "()" {
			return "|", true
		}
	}
	if (fb.Font.SmushMode & SMBIGX) != 0 {
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

func (fb *FigletBuilder) smushAmount(buffer []string, curChar []string) int {
	if (fb.Font.SmushMode & (SMSMUSH | SMKERN)) == 0 {
		return 0
	}
	maxSmush := fb.CurCharWidth
	for row := 0; row < fb.Font.Height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if fb.Direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		linebd := len(strings.TrimRight(lineLeft, " ")) - 1
		if linebd < 0 {
			linebd = 0
		}
		var ch1 string
		if linebd < len(lineLeft) {
			ch1 = string(lineLeft[linebd])
		}

		charbd := len(lineRight) - len(strings.TrimLeft(lineRight, " "))
		var ch2 string
		if charbd < len(lineRight) {
			ch2 = string(lineRight[charbd])
		}

		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == "" || ch1 == " " {
			amt += 1
		} else if ch2 != "" {
			if _, ok := fb.smushChars(ch1, ch2); ok {
				amt += 1
			}
		}
		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush
}

func (fb *FigletBuilder) smushRow(curChar []string, row int) (string, string) {
	addLeft := fb.Buffer[row]
	addRight := curChar[row]
	if fb.Direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < fb.MaxSmush; i++ {
		idx := len(addLeft) - fb.MaxSmush + i
		var left string
		if idx >= 0 && idx < len(addLeft) {
			left = string(addLeft[idx])
		}
		var right string
		if i < len(addRight) {
			right = string(addRight[i])
		}
		if smushed, ok := fb.smushChars(left, right); ok {
			runes := []rune(addLeft)
			if idx >= 0 && idx < len(runes) {
				runes[idx] = rune(smushed[0])
				addLeft = string(runes)
			}
		}
	}
	return addLeft, addRight
}

func (fb *FigletBuilder) cutBufferCommon() {
	fb.CurrentTotalWidth = 0
	fb.Buffer = make([]string, fb.Font.Height)
	fb.BlankMarkers = nil
	fb.PrevCharWidth = 0
	curChar := fb.getCurChar()
	if curChar != nil {
		fb.MaxSmush = fb.smushAmount(fb.Buffer, curChar)
	}
}

func (fb *FigletBuilder) handleNewLine() {
	if len(fb.BlankMarkers) > 0 {
		last := fb.BlankMarkers[len(fb.BlankMarkers)-1]
		fb.BlankMarkers = fb.BlankMarkers[:len(fb.BlankMarkers)-1]
		if last.Iterator != -1 {
			cp := make([]string, len(last.Buffer))
			copy(cp, last.Buffer)
			fb.Queue = append(fb.Queue, cp)
			fb.Iterator = last.Iterator
			fb.cutBufferCommon()
			return
		}
	}
	cp := make([]string, len(fb.Buffer))
	copy(cp, fb.Buffer)
	fb.Queue = append(fb.Queue, cp)
	fb.Iterator -= 1
	fb.cutBufferCommon()
}

func (fb *FigletBuilder) AddCharToProduct() {
	if fb.Text[fb.Iterator] == '\n' {
		cp := make([]string, len(fb.Buffer))
		copy(cp, fb.Buffer)
		fb.BlankMarkers = append(fb.BlankMarkers, BlankMarker{Buffer: cp, Iterator: fb.Iterator})
		fb.handleNewLine()
		return
	}
	curChar := fb.getCurChar()
	if curChar == nil {
		return
	}
	fb.CurCharWidth = fb.getCurWidth()
	fb.MaxSmush = fb.smushAmount(fb.Buffer, curChar)
	fb.CurrentTotalWidth = len(fb.Buffer[0]) + fb.CurCharWidth - fb.MaxSmush

	if fb.Text[fb.Iterator] == ' ' {
		cp := make([]string, len(fb.Buffer))
		copy(cp, fb.Buffer)
		fb.BlankMarkers = append(fb.BlankMarkers, BlankMarker{Buffer: cp, Iterator: fb.Iterator})
	}

	if fb.CurrentTotalWidth >= fb.Width {
		fb.handleNewLine()
	} else {
		for row := 0; row < fb.Font.Height; row++ {
			addLeft, addRight := fb.smushRow(curChar, row)
			if fb.MaxSmush < len(addRight) {
				fb.Buffer[row] = addLeft + addRight[fb.MaxSmush:]
			} else {
				fb.Buffer[row] = addLeft
			}
		}
	}
	fb.PrevCharWidth = fb.CurCharWidth
}

func (fb *FigletBuilder) Render() string {
	for fb.Iterator < len(fb.Text) {
		fb.AddCharToProduct()
		fb.Iterator++
	}
	if fb.Buffer[0] != "" {
		cp := make([]string, len(fb.Buffer))
		copy(cp, fb.Buffer)
		fb.Queue = append(fb.Queue, cp)
	}

	var sb strings.Builder
	for _, buf := range fb.Queue {
		jBuf := make([]string, len(buf))
		copy(jBuf, buf)
		if fb.Justify == "right" {
			for r := 0; r < fb.Font.Height; r++ {
				pad := fb.Width - len(jBuf[r]) - 1
				if pad > 0 {
					jBuf[r] = strings.Repeat(" ", pad) + jBuf[r]
				}
			}
		} else if fb.Justify == "center" {
			for r := 0; r < fb.Font.Height; r++ {
				pad := (fb.Width - len(jBuf[r])) / 2
				if pad > 0 {
					jBuf[r] = strings.Repeat(" ", pad) + jBuf[r]
				}
			}
		}
		for r := 0; r < fb.Font.Height; r++ {
			sb.WriteString(strings.ReplaceAll(jBuf[r], fb.Font.HardBlank, " "))
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func reverseMap(r rune) rune {
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

func flipMap(r rune) rune {
	switch r {
	case '/':
		return '\\'
	case '\\':
		return '/'
	case 'A':
		return 'V'
	case 'V':
		return 'A'
	case 'M':
		return 'W'
	case 'W':
		return 'M'
	case 'P':
		return 'b'
	case 'R':
		return 'b'
	case 'b':
		return 'P'
	case '^':
		return 'v'
	case 'v':
		return '^'
	case '_':
		return '-'
	case 'm':
		return 'w'
	case 'w':
		return 'm'
	}
	return r
}

func reverseString(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out []string
	for _, row := range lines {
		runes := []rune(row)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = reverseMap(runes[j]), reverseMap(runes[i])
		}
		if len(runes)%2 == 1 {
			mid := len(runes) / 2
			runes[mid] = reverseMap(runes[mid])
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

func flipString(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		runes := []rune(lines[i])
		for j := range runes {
			runes[j] = flipMap(runes[j])
		}
		out = append(out, string(runes))
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
	newFromList := strings.Join(out, "\n") + "\n"
	idx := len(newFromList)
	for idx > 0 && unicode.IsSpace(rune(newFromList[idx-1])) {
		idx--
	}
	return newFromList[:idx]
}

func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}

func main() {
	fontFlag := flag.String("f", "standard", "font")
	fontLongFlag := flag.String("font", "standard", "font")
	widthFlag := flag.Int("w", 80, "width")
	widthLongFlag := flag.Int("width", 80, "width")
	justifyFlag := flag.String("j", "auto", "justify")
	justifyLongFlag := flag.String("justify", "auto", "justify")
	directionFlag := flag.String("D", "auto", "direction")
	directionLongFlag := flag.String("direction", "auto", "direction")
	reverseFlag := flag.Bool("r", false, "reverse")
	reverseLongFlag := flag.Bool("reverse", false, "reverse")
	flipFlag := flag.Bool("F", false, "flip")
	flipLongFlag := flag.Bool("flip", false, "flip")
	stripFlag := flag.Bool("s", false, "strip")
	stripLongFlag := flag.Bool("strip-surrounding-newlines", false, "strip")
	normFlag := flag.Bool("n", false, "normalize")
	normLongFlag := flag.Bool("normalize-surrounding-newlines", false, "normalize")

	flag.Parse()

	fontName := *fontFlag
	if *fontLongFlag != "standard" {
		fontName = *fontLongFlag
	}
	width := *widthFlag
	if *widthLongFlag != 80 {
		width = *widthLongFlag
	}
	justify := *justifyFlag
	if *justifyLongFlag != "auto" {
		justify = *justifyLongFlag
	}
	direction := *directionFlag
	if *directionLongFlag != "auto" {
		direction = *directionLongFlag
	}
	isReverse := *reverseFlag || *reverseLongFlag
	isFlip := *flipFlag || *flipLongFlag
	isStrip := *stripFlag || *stripLongFlag
	isNorm := *normFlag || *normLongFlag

	text := strings.Join(flag.Args(), " ")

	font, err := LoadFont(fontName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	actualDirection := direction
	if actualDirection == "auto" {
		if font.PrintDirection == 1 {
			actualDirection = "right-to-left"
		} else {
			actualDirection = "left-to-right"
		}
	}

	actualJustify := justify
	if actualJustify == "auto" {
		if actualDirection == "right-to-left" {
			actualJustify = "right"
		} else {
			actualJustify = "left"
		}
	}

	builder := NewFigletBuilder(text, font, actualDirection, width, actualJustify)
	output := builder.Render()

	if isReverse {
		output = reverseString(output)
	}
	if isFlip {
		output = flipString(output)
	}
	if isStrip {
		output = stripSurroundingNewlines(output)
	} else if isNorm {
		output = normalizeSurroundingNewlines(output)
	}

	fmt.Println(output)
}

EOF

# Delete all .py files anywhere in the repo
find /workspace/repo -type f -name "*.py" -delete
rm -rf /workspace/repo/pyfiglet /workspace/repo/setup.py

# Build to verify
go build ./...
