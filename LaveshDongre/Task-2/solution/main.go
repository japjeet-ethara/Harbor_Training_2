package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed fonts/*.flf
var fontFiles embed.FS

const (
	smEqual      = 1
	smLowline    = 2
	smHierarchy  = 4
	smPair       = 8
	smBigX       = 16
	smHardblank  = 32
	smKern       = 64
	smSmush      = 128
)

type figFont struct {
	hardblank string
	height    int
	direction int
	smushMode int
	chars     map[rune][]string
	widths    map[rune]int
}

type blankMark struct {
	buffer []string
	index  int
}

type builder struct {
	text        []rune
	direction   string
	width       int
	justify     string
	font        *figFont
	index       int
	maxSmush    int
	prevWidth   int
	curWidth    int
	buffer      []string
	product     [][]string
	blankMarks  []blankMark
}

func loadFont(name string) (*figFont, error) {
	name = path.Base(name)
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".flf"), ".tlf")
	data, err := fs.ReadFile(fontFiles, path.Join("fonts", name+".flf"))
	if err != nil {
		return nil, err
	}
	return parseFont(string(data))
}

func parseFont(raw string) (*figFont, error) {
	raw = strings.NewReplacer("\r\n", "\n", "\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(raw)
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return nil, errors.New("empty font")
	}

	header := strings.Fields(lines[0])
	if len(header) < 6 || len(header[0]) < 6 ||
		!(strings.HasPrefix(header[0], "flf2") || strings.HasPrefix(header[0], "tlf2")) {
		return nil, errors.New("malformed font header")
	}
	hardblank := header[0][5:]
	height, err := strconv.Atoi(header[1])
	if err != nil {
		return nil, err
	}
	if _, err = strconv.Atoi(header[2]); err != nil {
		return nil, err
	}
	if _, err = strconv.Atoi(header[3]); err != nil {
		return nil, err
	}
	oldLayout, err := strconv.Atoi(header[4])
	if err != nil {
		return nil, err
	}
	commentLines, err := strconv.Atoi(header[5])
	if err != nil {
		return nil, err
	}
	direction := -1
	fullLayout := -1
	if len(header) > 6 {
		direction, _ = strconv.Atoi(header[6])
	}
	if len(header) > 7 {
		fullLayout, _ = strconv.Atoi(header[7])
	}
	if fullLayout == -1 {
		switch {
		case oldLayout == 0:
			fullLayout = 64
		case oldLayout < 0:
			fullLayout = 0
		default:
			fullLayout = (oldLayout & 31) | 128
		}
	}

	if height <= 0 || commentLines < 0 || len(lines) < 1+commentLines {
		return nil, errors.New("invalid font dimensions")
	}
	data := lines[1+commentLines:]
	font := &figFont{
		hardblank: hardblank,
		height:    height,
		direction: direction,
		smushMode: fullLayout,
		chars:     make(map[rune][]string),
		widths:    make(map[rune]int),
	}

	for ch := rune(32); ch < 127; ch++ {
		width, rows, rest, ok := readChar(data, height)
		if !ok {
			return nil, errors.New("truncated character definitions")
		}
		data = rest
		if ch == ' ' || strings.Join(rows, "") != "" {
			font.chars[ch] = rows
			font.widths[ch] = width
		}
	}
	for _, ch := range []rune("ÄÖÜäöüß") {
		if len(data) == 0 {
			break
		}
		width, rows, rest, ok := readChar(data, height)
		if !ok {
			return nil, errors.New("truncated extended character definitions")
		}
		data = rest
		if strings.Join(rows, "") != "" {
			font.chars[ch] = rows
			font.widths[ch] = width
		}
	}

	for len(data) > 0 {
		line := strings.TrimSpace(data[0])
		data = data[1:]
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		base := 10
		token := fields[0]
		if strings.HasPrefix(strings.ToLower(token), "0x") {
			base = 16
			token = token[2:]
		} else {
			continue
		}
		code, err := strconv.ParseInt(token, base, 32)
		if err != nil {
			continue
		}
		width, rows, rest, ok := readChar(data, height)
		if !ok {
			return nil, errors.New("truncated extended character definition")
		}
		data = rest
		if strings.Join(rows, "") != "" {
			font.chars[rune(code)] = rows
			font.widths[rune(code)] = width
		}
	}
	return font, nil
}

func readChar(data []string, height int) (int, []string, []string, bool) {
	if len(data) < height {
		return 0, nil, data, false
	}
	rows := make([]string, height)
	maxWidth := 0
	firstLine := strings.TrimRight(data[0], " \t")
	marker := ""
	if firstLine != "" {
		last, _ := utf8.DecodeLastRuneInString(firstLine)
		marker = string(last)
	}
	for i := 0; i < height; i++ {
		line := strings.TrimRight(data[i], " \t")
		if marker != "" && strings.HasSuffix(line, marker) {
			line = line[:len(line)-len(marker)]
			if strings.HasSuffix(line, marker) {
				line = line[:len(line)-len(marker)]
			}
		}
		rows[i] = line
		if n := utf8.RuneCountInString(line); n > maxWidth {
			maxWidth = n
		}
	}
	return maxWidth, rows, data[height:], true
}

func (b *builder) charAt(index int) ([]string, bool) {
	if index < 0 || index >= len(b.text) {
		return nil, false
	}
	rows, ok := b.font.chars[b.text[index]]
	return rows, ok
}

func (b *builder) widthAt(index int) int {
	if index < 0 || index >= len(b.text) {
		return 0
	}
	return b.font.widths[b.text[index]]
}

func (b *builder) currentSmushAmount(char []string) int {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := b.curWidth
	for row := 0; row < b.font.height; row++ {
		left, right := b.buffer[row], char[row]
		if b.direction == "right-to-left" {
			left, right = right, left
		}
		leftRunes, rightRunes := []rune(left), []rune(right)
		linebd := len([]rune(strings.TrimRight(left, " "))) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := ""
		if linebd < len(leftRunes) {
			ch1 = string(leftRunes[linebd])
		} else {
			linebd = 0
		}
		charbd := len(rightRunes) - len([]rune(strings.TrimLeft(right, " ")))
		ch2 := ""
		if charbd < len(rightRunes) {
			ch2 = string(rightRunes[charbd])
		} else {
			charbd = len(rightRunes)
		}
		amount := charbd + len(leftRunes) - 1 - linebd
		switch {
		case ch1 == "" || ch1 == " ":
			amount++
		case ch2 != "":
			if _, ok := b.smushChars(ch1, ch2); ok {
				amount++
			}
		}
		if amount < maxSmush {
			maxSmush = amount
		}
	}
	return maxSmush
}

func (b *builder) smushChars(left, right string) (string, bool) {
	if left == " " {
		return right, true
	}
	if right == " " {
		return left, true
	}
	if b.prevWidth < 2 || b.curWidth < 2 || b.font.smushMode&smSmush == 0 {
		return "", false
	}
	if b.font.smushMode&63 == 0 {
		if left == b.font.hardblank {
			return right, true
		}
		if right == b.font.hardblank {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if b.font.smushMode&smHardblank != 0 && left == b.font.hardblank && right == b.font.hardblank {
		return left, true
	}
	if left == b.font.hardblank || right == b.font.hardblank {
		return "", false
	}
	if b.font.smushMode&smEqual != 0 && left == right {
		return left, true
	}
	if b.font.smushMode&smLowline != 0 {
		if left == "_" && (right == "" || strings.ContainsRune("|/\\[]{}()<>", []rune(right)[0])) {
			return right, true
		}
		if right == "_" && (left == "" || strings.ContainsRune("|/\\[]{}()<>", []rune(left)[0])) {
			return left, true
		}
	}
	if b.font.smushMode&smHierarchy != 0 {
		groups := [][2]string{{"|", `/\[]{}()<>`}, {`/\`, "[]{}()<>"}, {"[]", "{}()<>"}, {"{}", "()<>"},
			{"()", "<>"}}
		for _, group := range groups {
			if strings.Contains(group[0], left) && strings.Contains(group[1], right) {
				return right, true
			}
			if strings.Contains(group[1], left) && strings.Contains(group[0], right) {
				return left, true
			}
		}
	}
	if b.font.smushMode&smPair != 0 {
		switch left + right {
		case "[]", "{}", "()":
			return "|", true
		}
		switch right + left {
		case "[]", "{}", "()":
			return "|", true
		}
	}
	if b.font.smushMode&smBigX != 0 {
		switch left + right {
		case `/\`:
			return "|", true
		case `\/`:
			return "Y", true
		case "><":
			return "X", true
		}
	}
	return "", false
}

func (b *builder) overlapLeft(addLeft string, i int) (string, int) {
	left := []rune(addLeft)
	index := len(left) - b.maxSmush + i
	if index >= 0 && index < len(left) {
		return string(left[index]), index
	}
	return "", index
}

func (b *builder) smushRow(char []string, row int) (string, string) {
	addLeft, addRight := b.buffer[row], char[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		left, index := b.overlapLeft(addLeft, i)
		rightRunes := []rune(addRight)
		right := ""
		if i < len(rightRunes) {
			right = string(rightRunes[i])
		}
		if smushed, ok := b.smushChars(left, right); ok {
			leftRunes := []rune(addLeft)
			if index >= 0 && index < len(leftRunes) {
				leftRunes[index] = []rune(smushed)[0]
				addLeft = string(leftRunes)
			}
		}
	}
	return addLeft, addRight
}

func (b *builder) addCharRows(char []string) {
	for row := 0; row < b.font.height; row++ {
		left, right := b.smushRow(char, row)
		rightRunes := []rune(right)
		if b.maxSmush > len(rightRunes) {
			b.buffer[row] = left
		} else {
			b.buffer[row] = left + string(rightRunes[b.maxSmush:])
		}
	}
}

func (b *builder) resetBuffer() {
	b.buffer = make([]string, b.font.height)
	b.blankMarks = nil
	b.prevWidth = 0
	if char, ok := b.charAt(b.index); ok {
		b.maxSmush = b.currentSmushAmount(char)
	}
}

func (b *builder) handleNewline() {
	if len(b.blankMarks) > 0 {
		last := b.blankMarks[len(b.blankMarks)-1]
		b.product = append(b.product, last.buffer)
		b.index = last.index
		b.resetBuffer()
		return
	}
	b.product = append(b.product, b.buffer)
	b.index--
	b.resetBuffer()
}

func (b *builder) addChar() error {
	if b.text[b.index] == '\n' {
		b.blankMarks = append(b.blankMarks, blankMark{append([]string(nil), b.buffer...), b.index})
		b.handleNewline()
		return nil
	}
	char, ok := b.charAt(b.index)
	if !ok {
		return nil
	}
	b.curWidth = b.widthAt(b.index)
	if b.width < b.curWidth {
		return errors.New("width is not enough to print a character")
	}
	b.maxSmush = b.currentSmushAmount(char)
	totalWidth := utf8.RuneCountInString(b.buffer[0]) + b.curWidth - b.maxSmush
	if b.text[b.index] == ' ' {
		b.blankMarks = append(b.blankMarks, blankMark{append([]string(nil), b.buffer...), b.index})
	}
	if totalWidth >= b.width {
		b.handleNewline()
	} else {
		b.addCharRows(char)
	}
	b.prevWidth = b.curWidth
	return nil
}

func (b *builder) render() (string, error) {
	for b.index < len(b.text) {
		if err := b.addChar(); err != nil {
			return "", err
		}
		b.index++
	}
	if b.buffer[0] != "" {
		b.product = append(b.product, b.buffer)
	}
	var result strings.Builder
	for _, buffer := range b.product {
		for row, line := range buffer {
			switch b.justify {
			case "right":
				padding := b.width - utf8.RuneCountInString(line) - 1
				if padding > 0 {
					line = strings.Repeat(" ", padding) + line
				}
			case "center":
				padding := (b.width - utf8.RuneCountInString(line)) / 2
				if padding > 0 {
					line = strings.Repeat(" ", padding) + line
				}
			}
			buffer[row] = strings.ReplaceAll(line, b.font.hardblank, " ")
		}
		result.WriteString(strings.Join(buffer, "\n"))
		result.WriteByte('\n')
	}
	return result.String(), nil
}

func translateRows(text string, mapping string, reverse bool) string {
	rows := strings.Split(text, "\n")
	if len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	for i, row := range rows {
		runes := []rune(row)
		for j, r := range runes {
			if r >= 32 && r <= 127 {
				runes[j] = rune(mapping[int(r)-32])
			}
		}
		if reverse {
			for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
				runes[left], runes[right] = runes[right], runes[left]
			}
		}
		rows[i] = string(runes)
	}
	return strings.Join(rows, "\n") + "\n"
}

func transformRows(text string, mapping string, reverse bool) string {
	rows := strings.Split(text, "\n")
	if len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	for i, row := range rows {
		runes := []rune(row)
		for j, r := range runes {
			if r >= 32 && r <= 127 {
				runes[j] = rune(mapping[int(r)-32])
			}
		}
		rows[i] = string(runes)
	}
	if reverse {
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
	}
	return strings.Join(rows, "\n") + "\n"
}

func trimSurroundingNewlines(text string) string {
	rows := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
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
	fontName := flag.String("f", "standard", "font")
	width := flag.Int("w", 80, "width")
	justify := flag.String("j", "auto", "justification")
	direction := flag.String("D", "auto", "direction")
	reverse := flag.Bool("r", false, "reverse output")
	flip := flag.Bool("F", false, "flip output")
	strip := flag.Bool("s", false, "strip surrounding newlines")
	normalize := flag.Bool("n", false, "normalize surrounding newlines")
	flag.Parse()
	if len(flag.Args()) == 0 {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: figlet [options] [text]")
		return
	}
	text := strings.Join(flag.Args(), " ")

	font, err := loadFont(*fontName)
	if err != nil {
		fmt.Fprintf(flag.CommandLine.Output(), "pyfiglet error: requested font %q not found.\n", *fontName)
		return
	}
	actualDirection := *direction
	if actualDirection == "auto" {
		actualDirection = "left-to-right"
		if font.direction == 1 {
			actualDirection = "right-to-left"
		}
	}
	actualJustify := *justify
	if actualJustify == "auto" {
		if actualDirection == "right-to-left" {
			actualJustify = "right"
		} else {
			actualJustify = "left"
		}
	}

	b := &builder{
		text:      []rune(text),
		direction: actualDirection,
		width:     *width,
		justify:   actualJustify,
		font:      font,
		buffer:    make([]string, font.height),
	}
	output, err := b.render()
	if err != nil {
		return
	}
	reverseMap := " !\"#$%&')(*+,-.\\" + "0123456789:;>=<?" +
		"@ABCDEFGHIJKLMNO" + "PQRSTUVWXYZ]/[^_" + "`abcdefghijklmno" + "pqrstuvwxyz}|{~\x7f"
	flipMap := " !\"#$%&'()*+,-.\\" + "0123456789:;<=>?" +
		"@VBCDEFGHIJKLWNO" + "bQbSTUAMXYZ[/]v-" + "`aPcdefghijklwno" + "pqrstu^mxyz{|}~\x7f"
	if *reverse {
		output = translateRows(output, reverseMap, true)
	}
	if *flip {
		output = transformRows(output, flipMap, true)
	}
	if *strip {
		output = trimSurroundingNewlines(output)
	} else if *normalize {
		output = "\n" + trimSurroundingNewlines(output) + "\n"
	}
	fmt.Print(output + "\n")
}
