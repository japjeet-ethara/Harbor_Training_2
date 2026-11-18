#!/bin/bash
set -e

cd /workspace/repo

# Embed the FIGlet fonts so the executable works from any directory.
mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/

# The submitted Go solution must not contain Python source files.
find . -type f -name "*.py" -delete

cat > go.mod <<'EOF'
module figlet

go 1.23
EOF

cat > main.go <<'GOEOF'
package main

import (
	"embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed fonts/*.flf
var fontFiles embed.FS

type Font struct {
	Height         int
	Hardblank      rune
	PrintDirection int
	SmushMode      int
	Chars          map[rune][]string
	Widths         map[rune]int
}

type blankMarker struct {
	buffer []string
	index  int
}

type Builder struct {
	text              []rune
	font              *Font
	direction         string
	width             int
	justify           string
	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int
	buffer            []string
	product           [][]string
	blankMarkers      []blankMarker
}

func loadFont(name string) (*Font, error) {
	name = strings.TrimSuffix(name, ".flf")
	name = strings.TrimSuffix(name, ".tlf")
	if name == "" {
		name = "standard"
	}
	data, err := fontFiles.ReadFile("fonts/" + name + ".flf")
	if err != nil {
		return nil, fmt.Errorf("font not found: %s", name)
	}
	return parseFont(name, string(data))
}

func parseFont(fontName, src string) (*Font, error) {
	src = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(src)
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("invalid font: %s", fontName)
	}
	headerLine := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(headerLine, "flf2") && !strings.HasPrefix(headerLine, "tlf2") {
		return nil, fmt.Errorf("invalid font: %s", fontName)
	}
	if len(headerLine) < 5 {
		return nil, fmt.Errorf("malformed font header: %s", fontName)
	}
	headerLine = headerLine[5:]
	parts := strings.Fields(headerLine)
	if len(parts) < 6 {
		return nil, fmt.Errorf("malformed font header: %s", fontName)
	}
	hardBlank := []rune(parts[0])[0]

	vals := make([]int, 6)
	for i := 0; i < 5; i++ {
		n, err := strconv.Atoi(parts[i+1])
		if err != nil {
			return nil, fmt.Errorf("malformed font header: %s", fontName)
		}
		vals[i] = n
	}
	height := vals[0]
	if height <= 0 {
		return nil, fmt.Errorf("malformed font dimensions: %s", fontName)
	}
	commentLines := vals[4]
	printDirection := 0
	fullLayout := 0
	if len(parts) > 6 {
		v, err := strconv.Atoi(parts[6])
		if err == nil {
			printDirection = v
		}
	}
	if len(parts) > 7 {
		v, err := strconv.Atoi(parts[7])
		if err == nil {
			fullLayout = v
		}
	} else {
		oldLayout := vals[3]
		if oldLayout == 0 {
			fullLayout = 64
		} else if oldLayout < 0 {
			fullLayout = 0
		} else {
			fullLayout = (oldLayout & 31) | 128
		}
	}

	data := lines[1:]
	if commentLines > 0 && commentLines <= len(data) {
		data = data[commentLines:]
	}

	f := &Font{
		Height:         height,
		Hardblank:      hardBlank,
		PrintDirection: printDirection,
		SmushMode:      fullLayout,
		Chars:          map[rune][]string{},
		Widths:         map[rune]int{},
	}

	parseGlyph := func() ([]string, int, error) {
		if len(data) < height {
			return nil, 0, fmt.Errorf("truncated glyph")
		}
		rows := make([]string, height)
		for i := 0; i < height; i++ {
			line := data[0]
			data = data[1:]
			r := []rune(line)
			j := len(r)
			for j > 0 && (r[j-1] == ' ' || r[j-1] == '\t' || r[j-1] == '\r') {
				j--
			}
			if j > 0 {
				endMarker := r[j-1]
				count := 0
				for j > 0 && r[j-1] == endMarker && count < 2 {
					j--
					count++
				}
				rows[i] = string(r[:j])
				continue
			}
			rows[i] = line
		}
		width := 0
		for _, row := range rows {
			if n := utf8.RuneCountInString(row); n > width {
				width = n
			}
		}
		return rows, width, nil
	}

	for code := 32; code < 127; code++ {
		rows, width, err := parseGlyph()
		if err != nil {
			return nil, fmt.Errorf("problem parsing %s: %w", fontName, err)
		}
		joined := strings.Join(rows, "")
		if code == 32 || joined != "" {
			f.Chars[rune(code)] = rows
			f.Widths[rune(code)] = width
		}
	}

	for _, ch := range []rune("ÄÖÜäöüß") {
		if len(data) < height {
			break
		}
		rows, width, err := parseGlyph()
		if err != nil {
			return nil, err
		}
		joined := strings.Join(rows, "")
		if joined != "" {
			f.Chars[ch] = rows
			f.Widths[ch] = width
		}
	}

	for len(data) > 0 {
		line := strings.TrimSpace(data[0])
		data = data[1:]
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(strings.ToLower(fields[0]), "0x") {
			continue
		}
		code, err := strconv.ParseInt(fields[0][2:], 16, 32)
		if err != nil {
			continue
		}
		rows, width, err := parseGlyph()
		if err != nil {
			return nil, err
		}
		joined := strings.Join(rows, "")
		if joined != "" {
			f.Chars[rune(code)] = rows
			f.Widths[rune(code)] = width
		}
	}

	return f, nil
}

func newBuilder(text string, f *Font, direction string, width int, justify string) *Builder {
	return &Builder{
		text:      []rune(text),
		font:      f,
		direction: direction,
		width:     width,
		justify:   justify,
		buffer:    make([]string, f.Height),
	}
}

func (b *Builder) charAt(i int) ([]string, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, false
	}
	g, ok := b.font.Chars[b.text[i]]
	return g, ok
}

func (b *Builder) charWidth(i int) int {
	if i < 0 || i >= len(b.text) {
		return 0
	}
	return b.font.Widths[b.text[i]]
}

func (b *Builder) getCurChar() []string {
	if c, ok := b.charAt(b.iterator); ok {
		return c
	}
	return nil
}

func (b *Builder) currentSmushAmount(cur []string) int {
	if (b.font.SmushMode & (128 | 64)) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.Height; row++ {
		left := b.buffer[row]
		right := cur[row]
		if b.direction == "right-to-left" {
			left, right = right, left
		}
		lrun := []rune(left)
		rrun := []rune(right)
		idx := len(lrun) - 1
		for idx >= 0 && lrun[idx] == ' ' {
			idx--
		}
		if idx < 0 {
			idx = 0
		}
		var ch1 rune
		if idx < len(lrun) {
			ch1 = lrun[idx]
		}
		j := 0
		for j < len(rrun) && rrun[j] == ' ' {
			j++
		}
		var ch2 rune
		if j < len(rrun) {
			ch2 = rrun[j]
		}
		amt := j + len(lrun) - 1 - idx
		if ch1 == 0 || ch1 == ' ' {
			amt++
		} else if ch2 != 0 {
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
	if (b.font.SmushMode & 128) == 0 {
		return 0, false
	}
	if (b.font.SmushMode & 63) == 0 {
		if left == b.font.Hardblank {
			return right, true
		}
		if right == b.font.Hardblank {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if (b.font.SmushMode&32) != 0 && left == b.font.Hardblank && right == b.font.Hardblank {
		return left, true
	}
	if left == b.font.Hardblank || right == b.font.Hardblank {
		return 0, false
	}
	if (b.font.SmushMode&1) != 0 && left == right {
		return left, true
	}
	if (b.font.SmushMode & 2) != 0 {
		if left == '_' && strings.ContainsRune("|/\\[]{}()<>", right) {
			return right, true
		}
		if right == '_' && strings.ContainsRune("|/\\[]{}()<>", left) {
			return left, true
		}
	}
	if (b.font.SmushMode & 4) != 0 {
		if strings.ContainsRune("|/\\[]{}()<>", left) && strings.ContainsRune("|/\\[]{}()<>", right) {
			ranks := map[rune]int{'|': 0, '/': 1, '\\': 1, '[': 2, ']': 2, '{': 3, '}': 3, '(': 4, ')': 4, '<': 5, '>': 5}
			la, lok := ranks[left]
			ra, rok := ranks[right]
			if lok && rok && la < ra {
				return right, true
			}
			if lok && rok && ra < la {
				return left, true
			}
		}
	}
	if (b.font.SmushMode & 8) != 0 {
		if (string(left)+string(right) == "[]") || (string(left)+string(right) == "{}") || (string(left)+string(right) == "()") ||
			(string(right)+string(left) == "[]") || (string(right)+string(left) == "{}") || (string(right)+string(left) == "()") {
			return '|', true
		}
	}
	if (b.font.SmushMode & 16) != 0 {
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

func (b *Builder) smushRow(curChar []string, row int) (string, string) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	leftRunes := []rune(addLeft)
	rightRunes := []rune(addRight)
	for i := 0; i < b.maxSmush; i++ {
		idx := len(leftRunes) - b.maxSmush + i
		var left rune
		if idx >= 0 && idx < len(leftRunes) {
			left = leftRunes[idx]
		}
		var right rune
		if i < len(rightRunes) {
			right = rightRunes[i]
		}
		if smushed, ok := b.smushChars(left, right); ok {
			if idx >= 0 && idx < len(leftRunes) {
				leftRunes[idx] = smushed
			}
		}
	}
	return string(leftRunes), addRight
}

func (b *Builder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	b.buffer[row] = addLeft + addRight[b.maxSmush:len(addRight)]
}

func (b *Builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([]string, b.font.Height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	if cur, ok := b.charAt(b.iterator); ok {
		b.curCharWidth = b.charWidth(b.iterator)
		b.maxSmush = b.currentSmushAmount(cur)
	}
}

func (b *Builder) cutBufferAtLastBlank(savedBuffer []string, savedIterator int) {
	b.product = append(b.product, savedBuffer)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *Builder) cutBufferAtLastChar() {
	b.product = append(b.product, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *Builder) getLastBlank() ([]string, int) {
	if len(b.blankMarkers) == 0 {
		return nil, -1
	}
	last := b.blankMarkers[len(b.blankMarkers)-1]
	b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
	return last.buffer, last.index
}

func (b *Builder) handleNewLine() {
	savedBuffer, savedIterator := b.getLastBlank()
	if savedIterator != -1 {
		b.cutBufferAtLastBlank(savedBuffer, savedIterator)
		return
	}
	b.cutBufferAtLastChar()
}

func cloneRows(rows []string) []string {
	out := make([]string, len(rows))
	copy(out, rows)
	return out
}

func (b *Builder) addCharToProduct() {
	if b.iterator >= len(b.text) {
		return
	}
	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: cloneRows(b.buffer), index: b.iterator})
		b.handleNewLine()
		return
	}
	curChar, ok := b.charAt(b.iterator)
	if !ok {
		return
	}
	b.curCharWidth = b.charWidth(b.iterator)
	if b.width < b.curCharWidth {
		return
	}
	b.maxSmush = b.currentSmushAmount(curChar)
	b.currentTotalWidth = utf8.RuneCountInString(b.buffer[0]) + b.curCharWidth - b.maxSmush
	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{buffer: cloneRows(b.buffer), index: b.iterator})
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
		b.product = append(b.product, cloneRows(b.buffer))
	}

	var out strings.Builder
	for _, buf := range b.product {
		fixed := make([]string, len(buf))
		copy(fixed, buf)
		for row := 0; row < len(fixed); row++ {
			if b.justify == "right" {
				pad := b.width - utf8.RuneCountInString(fixed[row]) - 1
				if pad > 0 {
					fixed[row] = strings.Repeat(" ", pad) + fixed[row]
				}
			} else if b.justify == "center" {
				pad := (b.width - utf8.RuneCountInString(fixed[row])) / 2
				if pad > 0 {
					fixed[row] = strings.Repeat(" ", pad) + fixed[row]
				}
			}
			fixed[row] = strings.ReplaceAll(fixed[row], string(b.font.Hardblank), " ")
		}
		out.WriteString(strings.Join(fixed, "\n"))
		out.WriteString("\n")
	}
	return out.String()
}

func translateASCII(r rune, mapping string) rune {
	if r >= 0 && r < 128 {
		rs := []rune(mapping)
		idx := int(r)
		if idx < len(rs) {
			return rs[idx]
		}
	}
	return r
}

const reverseMap = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f !\"#$%&')(*+,-.\\0123456789:;>=<?@ABCDEFGHIJKLMNOPQRSTUVWXYZ]/[^_`abcdefghijklmnopqrstuvwxyz}|{~\x7f\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"

const flipMap = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f !\"#$%&'()*+,-.\\0123456789:;<=>?@VBCDEFGHIJKLWNObQbSTUAMXYZ[/]v-`aPcdefghijklwnopqrstu^mxyz{|}~\x7f\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"

func reverseText(s string) string {
	rows := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, row := range rows {
		r := []rune(row)
		for j, ch := range r {
			r[j] = translateASCII(ch, reverseMap)
		}
		for a, z := 0, len(r)-1; a < z; a, z = a+1, z-1 {
			r[a], r[z] = r[z], r[a]
		}
		rows[i] = string(r)
	}
	return strings.Join(rows, "\n") + "\n"
}

func flipText(s string) string {
	rows := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for a, z := 0, len(rows)-1; a < z; a, z = a+1, z-1 {
		rows[a], rows[z] = rows[z], rows[a]
	}
	for i, row := range rows {
		r := []rune(row)
		for j, ch := range r {
			r[j] = translateASCII(ch, flipMap)
		}
		rows[i] = string(r)
	}
	return strings.Join(rows, "\n") + "\n"
}

func stripSurroundingNewlines(s string) string {
	rows := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	out := make([]string, 0, len(rows))
	seen := false
	for _, row := range rows {
		if strings.TrimSpace(row) != "" || seen {
			seen = true
			out = append(out, row)
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), " \t\r\n")
}

func main() {
	args := os.Args[1:]
	config := map[string]interface{}{"font": "standard", "width": 80, "justify": "auto", "direction": "auto", "reverse": false, "flip": false, "strip": false, "normalize": false}
	for len(args) > 0 {
		a := args[0]
		if a == "--" {
			args = args[1:]
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			break
		}
		args = args[1:]
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		getValue := func() string {
			if hasValue {
				return value
			}
			if len(args) == 0 {
				fmt.Fprintln(os.Stderr, "missing option value")
				os.Exit(2)
			}
			v := args[0]
			args = args[1:]
			return v
		}
		switch name {
		case "f", "font":
			config["font"] = getValue()
		case "w", "width":
			v, err := strconv.Atoi(getValue())
			if err != nil || v < 1 {
				fmt.Fprintln(os.Stderr, "invalid width")
				os.Exit(2)
			}
			config["width"] = v
		case "j", "justify":
			config["justify"] = getValue()
		case "D", "direction":
			config["direction"] = getValue()
		case "r", "reverse":
			config["reverse"] = true
		case "F", "flip":
			config["flip"] = true
		case "s", "strip-surrounding-newlines":
			config["strip"] = true
		case "n", "normalize-surrounding-newlines":
			config["normalize"] = true
		default:
			fmt.Fprintf(os.Stderr, "unknown option: %s\n", a)
			os.Exit(2)
		}
	}
	text := strings.Join(args, " ")
	fontName := config["font"].(string)
	width := config["width"].(int)
	justify := config["justify"].(string)
	direction := config["direction"].(string)

	font, err := loadFont(fontName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: requested font %q not found.\n", fontName)
		os.Exit(1)
	}
	if direction == "auto" {
		if font.PrintDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}
	if direction != "left-to-right" && direction != "right-to-left" {
		fmt.Fprintln(os.Stderr, "invalid direction")
		os.Exit(2)
	}
	if justify == "auto" {
		if direction == "right-to-left" {
			justify = "right"
		} else {
			justify = "left"
		}
	}
	if justify != "left" && justify != "right" && justify != "center" {
		fmt.Fprintln(os.Stderr, "invalid justification")
		os.Exit(2)
	}

	result := newBuilder(text, font, direction, width, justify).render()
	if config["reverse"].(bool) {
		result = reverseText(result)
	}
	if config["flip"].(bool) {
		result = flipText(result)
	}
	if config["strip"].(bool) {
		result = stripSurroundingNewlines(result)
	} else if config["normalize"].(bool) {
		result = "\n" + stripSurroundingNewlines(result) + "\n"
	}
	fmt.Print(result)
	fmt.Print("\n")
}
GOEOF

gofmt -w main.go
go build -o /tmp/figlet .
