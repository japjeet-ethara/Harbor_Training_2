#!/bin/bash
set -eu

ROOT=/workspace/repo
FONT_SOURCE="$ROOT/pyfiglet/fonts"

cd "$ROOT"
mkdir -p "$ROOT/fonts"
for font in standard slant small big mini banner shadow script; do
    source="$FONT_SOURCE/$font.flf"
    if [ ! -f "$source" ]; then
        printf 'Required FIGlet font is missing: %s\n' "$source" >&2
        exit 1
    fi
    cp "$source" "$ROOT/fonts/$font.flf"
done

cat > "$ROOT/go.mod" <<'EOF'
module figlet-go

go 1.23
EOF

cat > "$ROOT/main.go" <<'EOF'
package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

//go:embed fonts/*.flf
var embeddedFonts embed.FS

type glyph struct {
	rows  [][]rune
	width int
}

type font struct {
	hardBlank       rune
	height          int
	printDirection  *int
	smushMode       int
	glyphs          map[rune]glyph
}

type options struct {
	font      string
	width     int
	justify   string
	direction string
	reverse   bool
	flip      bool
	strip     bool
	normalize bool
	text      string
}

type blankMarker struct {
	rows     [][]rune
	iterator int
}

type builder struct {
	text             []rune
	font             *font
	direction        string
	width            int
	justify          string
	iterator         int
	maxSmush         int
	curCharWidth     int
	prevCharWidth    int
	currentTotalWidth int
	blankMarkers     []blankMarker
	product          [][][]rune
	buffer           [][]rune
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	f, err := loadFont(opts.font)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if opts.direction == "auto" {
		if f.printDirection != nil && *f.printDirection == 1 {
			opts.direction = "right-to-left"
		} else {
			opts.direction = "left-to-right"
		}
	}
	if opts.justify == "auto" {
		if opts.direction == "right-to-left" {
			opts.justify = "right"
		} else {
			opts.justify = "left"
		}
	}

	result, err := render(opts.text, f, opts.direction, opts.width, opts.justify)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.reverse {
		result = reverseRows(result)
	}
	if opts.flip {
		result = flipRows(result)
	}
	if opts.strip {
		result = stripSurroundingNewlines(result)
	} else if opts.normalize {
		result = normalizeSurroundingNewlines(result)
	}

	fmt.Print(result)
	fmt.Print("\n")
}

func parseOptions(args []string) (options, error) {
	opts := options{
		font:      "standard",
		width:     80,
		justify:   "auto",
		direction: "auto",
	}
	var text []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			text = append(text, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			text = append(text, arg)
			continue
		}
		if arg == "-r" || arg == "--reverse" {
			opts.reverse = true
			continue
		}
		if arg == "-F" || arg == "--flip" {
			opts.flip = true
			continue
		}
		if arg == "-s" || arg == "--strip-surrounding-newlines" {
			opts.strip = true
			continue
		}
		if arg == "-n" || arg == "--normalize-surrounding-newlines" {
			opts.normalize = true
			continue
		}

		key, value, hasValue := splitOption(arg)
		switch key {
		case "-f", "--font", "-w", "--width", "-j", "--justify", "-D", "--direction":
			if !hasValue {
				if i+1 >= len(args) {
					return opts, fmt.Errorf("option %s requires a value", key)
				}
				i++
				value = args[i]
			}
			switch key {
			case "-f", "--font":
				opts.font = value
			case "-w", "--width":
				width, err := strconv.Atoi(value)
				if err != nil {
					return opts, fmt.Errorf("invalid width %q: %w", value, err)
				}
				opts.width = width
			case "-j", "--justify":
				opts.justify = value
			case "-D", "--direction":
				opts.direction = value
			}
		default:
			return opts, fmt.Errorf("unknown option %q", arg)
		}
	}
	if len(text) == 0 {
		return opts, errors.New("text is required")
	}
	if opts.justify != "auto" && opts.justify != "left" && opts.justify != "center" && opts.justify != "right" {
		return opts, fmt.Errorf("invalid justification %q", opts.justify)
	}
	if opts.direction != "auto" && opts.direction != "left-to-right" && opts.direction != "right-to-left" {
		return opts, fmt.Errorf("invalid direction %q", opts.direction)
	}
	opts.text = strings.Join(text, " ")
	return opts, nil
}

func splitOption(arg string) (string, string, bool) {
	if i := strings.IndexByte(arg, '='); i >= 0 {
		return arg[:i], arg[i+1:], true
	}
	for _, key := range []string{"--font", "--width", "--justify", "--direction"} {
		if strings.HasPrefix(arg, key) && len(arg) > len(key) {
			return key, arg[len(key):], true
		}
	}
	if len(arg) > 2 {
		switch arg[:2] {
		case "-f", "-w", "-j", "-D":
			return arg[:2], arg[2:], true
		}
	}
	return arg, "", false
}

func loadFont(name string) (*font, error) {
	data, err := fs.ReadFile(embeddedFonts, "fonts/"+name+".flf")
	if err != nil {
		return nil, fmt.Errorf("font %q not found", name)
	}
	lines := splitFontLines(string(data))
	if len(lines) == 0 {
		return nil, fmt.Errorf("font %q is empty", name)
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for font %q", name)
	}
	magic := []rune(fields[0])
	if len(magic) < 6 || (magic[0] != 'f' && magic[0] != 't') || string(magic[1:4]) != "lf2" {
		return nil, fmt.Errorf("%q is not a valid FIGlet font", name)
	}

	lastHeaderField := len(fields) - 1
	if lastHeaderField > 7 {
		lastHeaderField = 7
	}
	numbers := make([]int, 0, lastHeaderField)
	for i := 1; i <= lastHeaderField; i++ {
		n, parseErr := strconv.Atoi(fields[i])
		if parseErr != nil {
			return nil, fmt.Errorf("malformed header for font %q: %w", name, parseErr)
		}
		numbers = append(numbers, n)
	}
	f := &font{
		hardBlank: magic[5],
		height:    numbers[0],
		glyphs:    make(map[rune]glyph),
	}
	if f.height < 1 {
		return nil, fmt.Errorf("invalid height in font %q", name)
	}
	oldLayout := numbers[3]
	commentLines := numbers[4]
	if len(numbers) > 5 {
		direction := numbers[5]
		f.printDirection = &direction
	}
	if len(numbers) > 6 {
		f.smushMode = numbers[6]
	} else if oldLayout == 0 {
		f.smushMode = 64
	} else if oldLayout < 0 {
		f.smushMode = 0
	} else {
		f.smushMode = (oldLayout & 31) | 128
	}
	if commentLines < 0 || 1+commentLines > len(lines) {
		return nil, fmt.Errorf("invalid comment count in font %q", name)
	}
	pos := 1 + commentLines
	for code := rune(32); code < 127; code++ {
		g, next, readErr := readGlyph(lines, pos, f.height)
		if readErr != nil {
			return nil, fmt.Errorf("font %q character %d: %w", name, code, readErr)
		}
		pos = next
		if code == ' ' || glyphHasContent(g) {
			f.glyphs[code] = g
		}
	}
	if pos < len(lines) {
		for _, code := range []rune("ÄÖÜäöüß") {
			g, next, readErr := readGlyph(lines, pos, f.height)
			if readErr != nil {
				return nil, fmt.Errorf("font %q character %U: %w", name, code, readErr)
			}
			pos = next
			if glyphHasContent(g) {
				f.glyphs[code] = g
			}
		}
	}
	for pos < len(lines) {
		line := strings.TrimSpace(lines[pos])
		pos++
		if line == "" {
			continue
		}
		token := strings.SplitN(line, " ", 2)[0]
		if !strings.HasPrefix(strings.ToLower(token), "0x") {
			continue
		}
		codeValue, parseErr := strconv.ParseInt(token[2:], 16, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("font %q has invalid character code %q", name, token)
		}
		g, next, readErr := readGlyph(lines, pos, f.height)
		if readErr != nil {
			return nil, fmt.Errorf("font %q character %q: %w", name, token, readErr)
		}
		pos = next
		if glyphHasContent(g) {
			f.glyphs[rune(codeValue)] = g
		}
	}
	return f, nil
}

func splitFontLines(data string) []string {
	data = strings.NewReplacer("\u0085", " ", "\u2028", " ", "\u2029", " ").Replace(data)
	runes := []rune(data)
	lines := make([]string, 0, len(runes)/32)
	start := 0
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\r' || r == '\n' || r == '\v' || r == '\f' || (r >= 0x1c && r <= 0x1e) {
			lines = append(lines, string(runes[start:i]))
			if r == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	if start < len(runes) {
		lines = append(lines, string(runes[start:]))
	}
	return lines
}

func readGlyph(lines []string, pos, height int) (glyph, int, error) {
	if pos+height > len(lines) {
		return glyph{}, pos, errors.New("font ended before glyph was complete")
	}
	rows := make([][]rune, height)
	var marker rune
	markerPattern := (*regexp.Regexp)(nil)
	width := 0
	for row := 0; row < height; row++ {
		line := lines[pos+row]
		if markerPattern == nil {
			trimmed := strings.TrimRightFunc(line, unicode.IsSpace)
			if trimmed == "" {
				return glyph{}, pos, errors.New("glyph row has no end marker")
			}
			markerRunes := []rune(trimmed)
			marker = markerRunes[len(markerRunes)-1]
			markerPattern = regexp.MustCompile(regexp.QuoteMeta(string(marker)) + `{1,2}\s*$`)
		}
		line = markerPattern.ReplaceAllString(line, "")
		rows[row] = []rune(line)
		if len(rows[row]) > width {
			width = len(rows[row])
		}
	}
	return glyph{rows: rows, width: width}, pos + height, nil
}

func glyphHasContent(g glyph) bool {
	for _, row := range g.rows {
		if len(row) != 0 {
			return true
		}
	}
	return false
}

func render(text string, f *font, direction string, width int, justify string) (string, error) {
	b := &builder{
		text:      []rune(text),
		font:      f,
		direction: direction,
		width:     width,
		justify:   justify,
		buffer:    make([][]rune, f.height),
	}
	maxSteps := len(b.text)*4 + 100
	for steps := 0; b.iterator < len(b.text); steps++ {
		if steps >= maxSteps {
			return "", errors.New("rendering made no progress at this width")
		}
		if err := b.addCharToProduct(); err != nil {
			return "", err
		}
		b.iterator++
	}
	if len(b.buffer) > 0 && len(b.buffer[0]) > 0 {
		b.product = append(b.product, b.buffer)
	}
	var output strings.Builder
	for _, rows := range b.product {
		for row := range rows {
			switch justify {
			case "right":
				padding := width - len(rows[row]) - 1
				if padding > 0 {
					rows[row] = append(make([]rune, padding), rows[row]...)
					for i := 0; i < padding; i++ {
						rows[row][i] = ' '
					}
				}
			case "center":
				padding := (width - len(rows[row])) / 2
				if padding > 0 {
					left := make([]rune, padding)
					for i := range left {
						left[i] = ' '
					}
					rows[row] = append(left, rows[row]...)
				}
			}
			for _, r := range rows[row] {
				if r == f.hardBlank {
					r = ' '
				}
				output.WriteRune(r)
			}
			output.WriteByte('\n')
		}
	}
	return output.String(), nil
}

func (b *builder) addCharToProduct() error {
	currentCode := b.text[b.iterator]
	if currentCode == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{rows: copyRows(b.buffer), iterator: b.iterator})
		b.handleNewLine()
		return nil
	}
	current, ok := b.font.glyphs[currentCode]
	if !ok {
		return nil
	}
	if b.width < current.width {
		return fmt.Errorf("width is not enough to print character %q", currentCode)
	}
	b.curCharWidth = current.width
	b.maxSmush = b.smushAmount(b.buffer, current)
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush
	if currentCode == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{rows: copyRows(b.buffer), iterator: b.iterator})
	}
	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			if err := b.addCurrentRow(current, row); err != nil {
				return err
			}
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) addCurrentRow(current glyph, row int) error {
	left := append([]rune(nil), b.buffer[row]...)
	right := current.rows[row]
	if b.direction == "right-to-left" {
		left, right = right, left
		left = append([]rune(nil), left...)
	}
	for i := 0; i < b.maxSmush; i++ {
		if i >= len(right) {
			return errors.New("font row is shorter than its smush amount")
		}
		index := len(left) - b.maxSmush + i
		if index < 0 || index > len(left) {
			continue
		}
		if index == len(left) {
			return errors.New("smush index is past the end of the left row")
		}
		leftChar := left[index]
		smushed, ok := b.smushChars(leftChar, right[i])
		if !ok {
			return errors.New("font layout requires an invalid smush")
		}
		left[index] = smushed
	}
	b.buffer[row] = append(left, right[b.maxSmush:]...)
	return nil
}

func (b *builder) handleNewLine() {
	if len(b.blankMarkers) > 0 {
		last := b.blankMarkers[len(b.blankMarkers)-1]
		b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
		b.product = append(b.product, last.rows)
		b.iterator = last.iterator
		b.cutBufferCommon()
		return
	}
	b.product = append(b.product, b.buffer)
	b.iterator--
	b.cutBufferCommon()
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = make([][]rune, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	if b.iterator < 0 || b.iterator >= len(b.text) {
		return
	}
	current, ok := b.font.glyphs[b.text[b.iterator]]
	if ok {
		b.maxSmush = b.smushAmount(b.buffer, current)
	}
}

func (b *builder) smushAmount(buffer [][]rune, current glyph) int {
	if b.font.smushMode&(128|64) == 0 {
		return 0
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		left := buffer[row]
		right := current.rows[row]
		if b.direction == "right-to-left" {
			left, right = right, left
		}
		lineBoundary := len(trimRightSpaces(left)) - 1
		if lineBoundary < 0 {
			lineBoundary = 0
		}
		var ch1 rune
		if lineBoundary < len(left) {
			ch1 = left[lineBoundary]
		} else {
			lineBoundary = 0
		}
		charBoundary := 0
		for charBoundary < len(right) && right[charBoundary] == ' ' {
			charBoundary++
		}
		var ch2 rune
		if charBoundary < len(right) {
			ch2 = right[charBoundary]
		} else {
			charBoundary = len(right)
		}
		amount := charBoundary + len(left) - 1 - lineBoundary
		if ch1 == 0 || ch1 == ' ' {
			amount++
		} else if ch2 != 0 {
			if _, canSmush := b.smushChars(ch1, ch2); canSmush {
				amount++
			}
		}
		if amount < maxSmush {
			maxSmush = amount
		}
	}
	return maxSmush
}

func trimRightSpaces(value []rune) []rune {
	end := len(value)
	for end > 0 && value[end-1] == ' ' {
		end--
	}
	return value[:end]
}

func (b *builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 || b.font.smushMode&128 == 0 {
		return 0, false
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
	if b.font.smushMode&32 != 0 && left == b.font.hardBlank && right == b.font.hardBlank {
		return left, true
	}
	if left == b.font.hardBlank || right == b.font.hardBlank {
		return 0, false
	}
	if b.font.smushMode&1 != 0 && left == right {
		return left, true
	}
	if b.font.smushMode&2 != 0 {
		if left == '_' && strings.ContainsRune("|/\\[]{}()<>", right) {
			return right, true
		}
		if right == '_' && strings.ContainsRune("|/\\[]{}()<>", left) {
			return left, true
		}
	}
	if b.font.smushMode&4 != 0 {
		hierarchy := [][2]string{
			{"|", `/\[]{}()<>`},
			{`/\`, "[]{}()<>"},
			{"[]", "{}()<>"},
			{"{}", "()<>"},
			{"()", "<>"},
		}
		for _, pair := range hierarchy {
			if strings.ContainsRune(pair[0], left) && strings.ContainsRune(pair[1], right) {
				return right, true
			}
			if strings.ContainsRune(pair[0], right) && strings.ContainsRune(pair[1], left) {
				return left, true
			}
		}
	}
	if b.font.smushMode&8 != 0 {
		if (left == '[' && right == ']') || (left == ']' && right == '[') ||
			(left == '{' && right == '}') || (left == '}' && right == '{') ||
			(left == '(' && right == ')') || (left == ')' && right == '(') {
			return '|', true
		}
	}
	if b.font.smushMode&16 != 0 {
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

func copyRows(rows [][]rune) [][]rune {
	result := make([][]rune, len(rows))
	for i := range rows {
		result[i] = append([]rune(nil), rows[i]...)
	}
	return result
}

func splitRenderedLines(value string) []string {
	if value == "" {
		return nil
	}
	lines := strings.Split(value, "\n")
	if strings.HasSuffix(value, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func reverseRows(value string) string {
	lines := splitRenderedLines(value)
	for i, line := range lines {
		runes := []rune(line)
		for j := range runes {
			runes[j] = reverseChar(runes[j])
		}
		for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
			runes[left], runes[right] = runes[right], runes[left]
		}
		lines[i] = string(runes)
	}
	return strings.Join(lines, "\n") + "\n"
}

func reverseChar(r rune) rune {
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
	default:
		return r
	}
}

func flipRows(value string) string {
	lines := splitRenderedLines(value)
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	for i, line := range lines {
		runes := []rune(line)
		for j, r := range runes {
			runes[j] = flipChar(r)
		}
		lines[i] = string(runes)
	}
	return strings.Join(lines, "\n") + "\n"
}

func flipChar(r rune) rune {
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
	case 'P', 'R':
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
	default:
		return r
	}
}

func stripSurroundingNewlines(value string) string {
	lines := splitRenderedLines(value)
	output := make([]string, 0, len(lines))
	charsSeen := false
	for _, line := range lines {
		if strings.TrimSpace(line) != "" || charsSeen {
			charsSeen = true
			output = append(output, line)
		}
	}
	joined := strings.Join(output, "\n")
	if len(output) > 0 {
		joined += "\n"
	}
	return strings.TrimRightFunc(joined, unicode.IsSpace)
}

func normalizeSurroundingNewlines(value string) string {
	return "\n" + stripSurroundingNewlines(value) + "\n"
}
EOF

gofmt -w "$ROOT/main.go"
chmod -R u+rwX "$ROOT/fonts"
chmod u+rw "$ROOT/go.mod" "$ROOT/main.go"
find "$ROOT" -type f -name '*.py' -exec rm -f {} \;
