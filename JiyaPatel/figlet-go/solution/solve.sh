#!/bin/bash
set -e

cd /workspace/repo

cat << 'EOF' > go.mod
module figlet

go 1.23
EOF

cat << 'EOF' > main.go
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

//go:embed pyfiglet/fonts/*.flf
var embeddedFonts embed.FS

type Font struct {
	HardBlank      rune
	Height         int
	PrintDirection int
	SmushMode      int
	Chars          map[rune][]string
	Width          map[rune]int
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func loadFont(name string) (*Font, error) {
	if !strings.HasSuffix(name, ".flf") {
		name = name + ".flf"
	}

	var raw []byte
	var err error

	searchDirs := []string{"pyfiglet/fonts", "/workspace/repo/pyfiglet/fonts", "."}
	for _, dir := range searchDirs {
		p := filepath.Join(dir, name)
		if data, e := os.ReadFile(p); e == nil {
			raw = data
			break
		}
	}
	if raw == nil {
		_ = filepath.WalkDir("/workspace/repo", func(path string, d fs.DirEntry, e error) error {
			if e == nil && !d.IsDir() && d.Name() == name {
				raw, _ = os.ReadFile(path)
				return filepath.SkipAll
			}
			return nil
		})
	}
	if raw == nil {
		raw, err = embeddedFonts.ReadFile("pyfiglet/fonts/" + name)
		if err != nil {
			return nil, fmt.Errorf("font not found: %s", name)
		}
	}

	strData := string(raw)
	strData = strings.ReplaceAll(strData, "\u0085", " ")
	strData = strings.ReplaceAll(strData, "\u2028", " ")
	strData = strings.ReplaceAll(strData, "\u2029", " ")

	lines := splitLines(strData)
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font")
	}

	reMagic := regexp.MustCompile(`^[ft]lf2.`)
	headerLine := lines[0]
	lines = lines[1:]
	if !reMagic.MatchString(headerLine) {
		return nil, fmt.Errorf("invalid header")
	}
	headerLine = reMagic.ReplaceAllString(headerLine, "")
	header := strings.Fields(headerLine)
	if len(header) < 6 {
		return nil, fmt.Errorf("malformed header")
	}

	hardBlank := []rune(header[0])[0]
	height, _ := strconv.Atoi(header[1])
	oldLayout, _ := strconv.Atoi(header[4])
	commentLines, _ := strconv.Atoi(header[5])

	printDirection := 0
	hasFullLayout := false
	fullLayout := 0

	if len(header) > 6 {
		printDirection, _ = strconv.Atoi(header[6])
	}
	if len(header) > 7 {
		fullLayout, _ = strconv.Atoi(header[7])
		hasFullLayout = true
	}

	if !hasFullLayout {
		if oldLayout == 0 {
			fullLayout = 64
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | 128
		}
	}

	if commentLines > len(lines) {
		commentLines = len(lines)
	}
	lines = lines[commentLines:]

	f := &Font{
		HardBlank:      hardBlank,
		Height:         height,
		PrintDirection: printDirection,
		SmushMode:      fullLayout,
		Chars:          make(map[rune][]string),
		Width:          make(map[rune]int),
	}

	reEndMarker := regexp.MustCompile(`(.)\s*$`)
	parseChar := func() (int, []string, bool) {
		if len(lines) < height {
			return 0, nil, false
		}
		var endRe *regexp.Regexp
		maxW := 0
		glyph := make([]string, height)
		for r := 0; r < height; r++ {
			line := lines[0]
			lines = lines[1:]
			if endRe == nil {
				m := reEndMarker.FindStringSubmatch(line)
				if len(m) > 1 {
					endRe = regexp.MustCompile(regexp.QuoteMeta(m[1]) + `{1,2}\s*$`)
				} else {
					endRe = regexp.MustCompile(`@\s*$`)
				}
			}
			line = endRe.ReplaceAllString(line, "")
			w := len([]rune(line))
			if w > maxW {
				maxW = w
			}
			glyph[r] = line
		}
		return maxW, glyph, true
	}

	for i := rune(32); i < 127; i++ {
		w, letter, ok := parseChar()
		if !ok {
			break
		}
		if i == 32 || strings.Join(letter, "") != "" {
			f.Chars[i] = letter
			f.Width[i] = w
		}
	}

	if len(lines) > 0 {
		for _, ch := range []rune{'Ä', 'Ö', 'Ü', 'ä', 'ö', 'ü', 'ß'} {
			w, letter, ok := parseChar()
			if !ok {
				break
			}
			if strings.Join(letter, "") != "" {
				f.Chars[ch] = letter
				f.Width[ch] = w
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
			code, err := strconv.ParseInt(codeStr[2:], 16, 32)
			if err == nil {
				w, letter, ok := parseChar()
				if ok && strings.Join(letter, "") != "" {
					f.Chars[rune(code)] = letter
					f.Width[rune(code)] = w
				}
			}
		}
	}

	return f, nil
}

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

type blankMarker struct {
	buffer   []string
	iterator int
}

type Builder struct {
	text              []rune
	direction         string
	width             int
	font              *Font
	justify           string
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	blankMarkers      []blankMarker
	productQueue      [][]string
	buffer            []string
}

func newBuilder(text string, font *Font, direction string, width int, justify string) *Builder {
	return &Builder{
		text:      []rune(text),
		direction: direction,
		width:     width,
		font:      font,
		justify:   justify,
		buffer:    make([]string, font.Height),
	}
}

func (b *Builder) getCharAt(i int) ([]string, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	g, ok := b.font.Chars[b.text[i]]
	return g, ok
}

func (b *Builder) getCharWidthAt(i int) (int, bool) {
	if i < 0 || i >= len(b.text) {
		return 0, false
	}
	w, ok := b.font.Width[b.text[i]]
	return w, ok
}

func (b *Builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}
	if (b.font.SmushMode & smSmush) == 0 {
		return 0, false
	}
	if (b.font.SmushMode & 63) == 0 {
		if left == b.font.HardBlank {
			return right, true
		}
		if right == b.font.HardBlank {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if (b.font.SmushMode & smHardblank) != 0 {
		if left == b.font.HardBlank && right == b.font.HardBlank {
			return left, true
		}
	}
	if left == b.font.HardBlank || right == b.font.HardBlank {
		return 0, false
	}
	if (b.font.SmushMode & smEqual) != 0 {
		if left == right {
			return left, true
		}
	}
	if (b.font.SmushMode & smLowline) != 0 {
		others := `|/\[]{}()<>`
		if left == '_' && strings.ContainsRune(others, right) {
			return right, true
		}
		if right == '_' && strings.ContainsRune(others, left) {
			return left, true
		}
	}
	if (b.font.SmushMode & smHierarchy) != 0 {
		pairs := [][2]string{
			{"|", `/\[]{}()<>`},
			{`\/`, `[]{}()<>`},
			{"[]", `{}()<>`},
			{"{}", `()<>`},
			{"()", `<>`},
		}
		for _, p := range pairs {
			if strings.ContainsRune(p[0], left) && strings.ContainsRune(p[1], right) {
				return right, true
			}
			if strings.ContainsRune(p[0], right) && strings.ContainsRune(p[1], left) {
				return left, true
			}
		}
	}
	if (b.font.SmushMode & smPair) != 0 {
		pair := string([]rune{left, right})
		rpair := string([]rune{right, left})
		if pair == "[]" || pair == "{}" || pair == "()" || rpair == "[]" || rpair == "{}" || rpair == "()" {
			return '|', true
		}
	}
	if (b.font.SmushMode & smBigX) != 0 {
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

func (b *Builder) smushAmount(buffer []string, curChar []string) int {
	if (b.font.SmushMode & (smSmush | smKern)) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.Height; row++ {
		lineLeft := []rune(buffer[row])
		lineRight := []rune(curChar[row])
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		// rstrip(' ')
		endLeft := len(lineLeft)
		for endLeft > 0 && lineLeft[endLeft-1] == ' ' {
			endLeft--
		}
		linebd := endLeft - 1
		if linebd < 0 {
			linebd = 0
		}

		var ch1 rune
		hasCh1 := false
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
			hasCh1 = true
		} else {
			linebd = 0
		}

		// lstrip(' ')
		startRight := 0
		for startRight < len(lineRight) && lineRight[startRight] == ' ' {
			startRight++
		}
		charbd := startRight
		var ch2 rune
		hasCh2 := false
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
			hasCh2 = true
		} else {
			charbd = len(lineRight)
		}

		amt := charbd + len(lineLeft) - 1 - linebd
		if !hasCh1 || ch1 == ' ' {
			amt++
		} else if hasCh2 {
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

func (b *Builder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft := []rune(b.buffer[row])
	addRight := []rune(curChar[row])
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		var left rune
		hasLeft := false
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
			hasLeft = true
		}
		right := addRight[i]
		if hasLeft {
			if smushed, ok := b.smushChars(left, right); ok {
				if idx >= 0 && idx < len(addLeft) {
					addLeft[idx] = smushed
				}
			}
		}
	}
	res := append(addLeft, addRight[b.maxSmush:]...)
	b.buffer[row] = string(res)
}

func (b *Builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.Height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, ok := b.getCharAt(b.iterator)
	if !ok {
		return
	}
	b.maxSmush = b.smushAmount(b.buffer, curChar)
}

func (b *Builder) handleNewLine() {
	if len(b.blankMarkers) > 0 {
		last := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.productQueue = append(b.productQueue, last.buffer)
		b.iterator = last.iterator
		b.cutBufferCommon()
	} else {
		b.productQueue = append(b.productQueue, b.buffer)
		b.iterator--
		b.cutBufferCommon()
	}
}

func (b *Builder) addCharToProduct() {
	curChar, ok := b.getCharAt(b.iterator)
	if b.text[b.iterator] == '\n' {
		cp := append([]string(nil), b.buffer...)
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: cp, iterator: b.iterator})
		b.handleNewLine()
		return
	}
	if !ok {
		return
	}
	w, _ := b.getCharWidthAt(b.iterator)
	b.curCharWidth = w
	b.maxSmush = b.smushAmount(b.buffer, curChar)
	b.currentTotalWidth = len([]rune(b.buffer[0])) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		cp := append([]string(nil), b.buffer...)
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: cp, iterator: b.iterator})
	}
	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.Height; row++ {
			b.addCurCharRowToBufferRow(curChar, row)
		}
	}
	b.prevCharWidth = b.curCharWidth
}

func (b *Builder) render() string {
	for b.iterator < len(b.text) {
		b.addCharToProduct()
		b.iterator++
	}
	if b.buffer[0] != "" {
		b.productQueue = append(b.productQueue, b.buffer)
	}
	var acc strings.Builder
	for _, buf := range b.productQueue {
		if b.justify == "right" {
			for row := 0; row < b.font.Height; row++ {
				pad := b.width - len([]rune(buf[row])) - 1
				if pad > 0 {
					buf[row] = strings.Repeat(" ", pad) + buf[row]
				}
			}
		} else if b.justify == "center" {
			for row := 0; row < b.font.Height; row++ {
				pad := (b.width - len([]rune(buf[row]))) / 2
				if pad > 0 {
					buf[row] = strings.Repeat(" ", pad) + buf[row]
				}
			}
		}
		s := strings.Join(buf, "\n") + "\n"
		s = strings.ReplaceAll(s, string(b.font.HardBlank), " ")
		acc.WriteString(s)
	}
	return acc.String()
}

var reverseMap = map[rune]rune{
	'(': ')', ')': '(',
	'/': '\\', '\\': '/',
	'<': '>', '>': '<',
	'[': ']', ']': '[',
	'{': '}', '}': '{',
}

var flipMap = map[rune]rune{
	'/': '\\', '\\': '/',
	'A': 'V', 'V': 'A',
	'M': 'W', 'W': 'M',
	'P': 'b', 'R': 'b', 'b': 'P',
	'T': 'u', 'u': 'T',
	'[': ']', ']': '[',
	'^': 'v', 'v': '^',
	'_': '-',
	'a': 'p', 'p': 'a',
	'w': 'm', 'm': 'w',
}

func reverseFiglet(s string) string {
	rows := splitLines(s)
	out := make([]string, len(rows))
	for i, row := range rows {
		runes := []rune(row)
		for j, r := range runes {
			if m, ok := reverseMap[r]; ok {
				runes[j] = m
			}
		}
		for l, r := 0, len(runes)-1; l < r; l, r = l+1, r-1 {
			runes[l], runes[r] = runes[r], runes[l]
		}
		out[i] = string(runes)
	}
	return strings.Join(out, "\n") + "\n"
}

func flipFiglet(s string) string {
	rows := splitLines(s)
	out := make([]string, len(rows))
	for i := range rows {
		src := []rune(rows[len(rows)-1-i])
		for j, r := range src {
			if m, ok := flipMap[r]; ok {
				src[j] = m
			}
		}
		out[i] = string(src)
	}
	return strings.Join(out, "\n") + "\n"
}

func stripSurroundingNewlines(s string) string {
	rows := splitLines(s)
	var out []string
	charsSeen := false
	for _, row := range rows {
		if strings.TrimSpace(row) != "" || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}
	res := strings.Join(out, "\n") + "\n"
	return strings.TrimRight(res, " \t\r\n")
}

func main() {
	fontName := "standard"
	width := 80
	justify := "auto"
	direction := "auto"
	reverse := false
	flip := false
	stripNL := false
	normNL := false
	var textParts []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			textParts = append(textParts, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			switch arg {
			case "-f", "--font":
				if i+1 < len(args) {
					fontName = args[i+1]
					i++
				}
			case "-w", "--width":
				if i+1 < len(args) {
					width, _ = strconv.Atoi(args[i+1])
					i++
				}
			case "-j", "--justify":
				if i+1 < len(args) {
					justify = args[i+1]
					i++
				}
			case "-D", "--direction":
				if i+1 < len(args) {
					direction = args[i+1]
					i++
				}
			case "-r", "--reverse":
				reverse = true
			case "-F", "--flip":
				flip = true
			case "-s", "--strip-surrounding-newlines":
				stripNL = true
			case "-n", "--normalize-surrounding-newlines":
				normNL = true
			default:
				if strings.HasPrefix(arg, "-f") {
					fontName = arg[2:]
				} else if strings.HasPrefix(arg, "-w") {
					width, _ = strconv.Atoi(arg[2:])
				} else if strings.HasPrefix(arg, "-j") {
					justify = arg[2:]
				} else if strings.HasPrefix(arg, "-D") {
					direction = arg[2:]
				}
			}
		} else {
			textParts = append(textParts, arg)
		}
	}

	f, err := loadFont(fontName)
	if err != nil {
		os.Exit(1)
	}

	dir := direction
	if dir == "auto" {
		if f.PrintDirection == 1 {
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

	text := strings.Join(textParts, " ")
	b := newBuilder(text, f, dir, width, just)
	r := b.render()
    if reverse {
        r = reverseFiglet(r)
    }
    if flip {
        r = flipFiglet(r)
    }
    if stripNL {
        r = stripSurroundingNewlines(r)
    } else if normNL {
        r = "\n" + stripSurroundingNewlines(r) + "\n"
    }

    fmt.Print(r)
}
EOF

find /workspace/repo -name "*.py" -delete
go build -o /tmp/figlet .
