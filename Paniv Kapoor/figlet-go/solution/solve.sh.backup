#!/bin/bash
set -euo pipefail

cd /workspace/repo

# 1. Prepare fonts directory for embedding and runtime discovery
mkdir -p /workspace/repo/fonts
if [ -d /workspace/repo/pyfiglet/fonts ] && [ "$(ls -A /workspace/repo/pyfiglet/fonts 2>/dev/null)" ]; then
    cp -a /workspace/repo/pyfiglet/fonts/. /workspace/repo/fonts/ 2>/dev/null || true
fi
if [ ! -f /workspace/repo/fonts/standard.flf ] && [ -d /workspace/repo/pyfiglet/fonts-standard ]; then
    cp -a /workspace/repo/pyfiglet/fonts-standard/. /workspace/repo/fonts/ 2>/dev/null || true
fi

# Remove non-font files that may be in fonts-standard (like __init__.py, .flc files)
# Keep only .flf and .tlf font files
find /workspace/repo/fonts -type f ! -name "*.flf" ! -name "*.tlf" -delete 2>/dev/null || true

# 2. Delete all python files and build caches from the repository
find /workspace/repo -name "*.py" -type f -delete
find /workspace/repo -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true
find /workspace/repo -name "*.pyc" -type f -delete
rm -rf /workspace/repo/pyfiglet/fonts-standard /workspace/repo/pyfiglet/fonts-contrib /workspace/repo/pyfiglet/test-fonts /workspace/repo/pyfiglet/tests 2>/dev/null || true
rm -f /workspace/repo/setup.py /workspace/repo/setup.cfg /workspace/repo/MANIFEST.in /workspace/repo/pyproject.toml /workspace/repo/pyrightconfig.json /workspace/repo/.travis.yml /workspace/repo/dev-requirements.txt /workspace/repo/requirements.txt /workspace/repo/test_font.sh 2>/dev/null || true

# 3. Create go.mod
cat << 'GOMOD' > /workspace/repo/go.mod
module figlet

go 1.23
GOMOD

# 4. Create main.go implementing complete PyFiglet behavior
cat << 'GOEOF' > /workspace/repo/main.go
package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:embed fonts
var embeddedFonts embed.FS

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

type Font struct {
	name           string
	hardBlank      rune
	height         int
	baseLine       int
	maxLength      int
	oldLayout      int
	commentLines   int
	printDirection int
	smushMode      int
	chars          map[rune][]string
	widths         map[rune]int
}

type CLIOptions struct {
	Font      string
	Width     int
	Justify   string
	Direction string
	Reverse   bool
	Flip      bool
	Strip     bool
	Normalize bool
	Args      []string
}

func parseCLI(args []string) (*CLIOptions, error) {
	opts := &CLIOptions{
		Font:      "standard",
		Width:     80,
		Justify:   "auto",
		Direction: "auto",
	}

	positionalOnly := false
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if positionalOnly {
			positional = append(positional, arg)
			i++
			continue
		}

		if arg == "--" {
			positionalOnly = true
			i++
			continue
		}

		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			i++
			continue
		}

		// Long options
		if strings.HasPrefix(arg, "--") {
			opt := arg[2:]
			val := ""
			hasVal := false
			if idx := strings.Index(opt, "="); idx != -1 {
				val = opt[idx+1:]
				opt = opt[:idx]
				hasVal = true
			}

			switch opt {
			case "font":
				if !hasVal {
					if i+1 >= len(args) {
						return nil, fmt.Errorf("option --font requires argument")
					}
					i++
					val = args[i]
				}
				opts.Font = val
			case "width":
				if !hasVal {
					if i+1 >= len(args) {
						return nil, fmt.Errorf("option --width requires argument")
					}
					i++
					val = args[i]
				}
				w, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid width: %v", err)
				}
				opts.Width = w
			case "justify":
				if !hasVal {
					if i+1 >= len(args) {
						return nil, fmt.Errorf("option --justify requires argument")
					}
					i++
					val = args[i]
				}
				if val != "auto" && val != "left" && val != "center" && val != "right" {
					return nil, fmt.Errorf("invalid justify: %s", val)
				}
				opts.Justify = val
			case "direction":
				if !hasVal {
					if i+1 >= len(args) {
						return nil, fmt.Errorf("option --direction requires argument")
					}
					i++
					val = args[i]
				}
				if val != "auto" && val != "left-to-right" && val != "right-to-left" {
					return nil, fmt.Errorf("invalid direction: %s", val)
				}
				opts.Direction = val
			case "reverse":
				opts.Reverse = true
			case "flip":
				opts.Flip = true
			case "strip-surrounding-newlines":
				opts.Strip = true
			case "normalize-surrounding-newlines":
				opts.Normalize = true
			default:
				return nil, fmt.Errorf("unknown option --%s", opt)
			}
			i++
			continue
		}

		// Short options (cluster)
		cluster := arg[1:]
		if idx := strings.Index(cluster, "="); idx != -1 {
			flag := cluster[:idx]
			val := cluster[idx+1:]
			switch flag {
			case "f":
				opts.Font = val
			case "w":
				w, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid width: %v", err)
				}
				opts.Width = w
			case "j":
				if val != "auto" && val != "left" && val != "center" && val != "right" {
					return nil, fmt.Errorf("invalid justify: %s", val)
				}
				opts.Justify = val
			case "D":
				if val != "auto" && val != "left-to-right" && val != "right-to-left" {
					return nil, fmt.Errorf("invalid direction: %s", val)
				}
				opts.Direction = val
			default:
				return nil, fmt.Errorf("unknown option -%s", flag)
			}
			i++
			continue
		}

		j := 0
		for j < len(cluster) {
			ch := cluster[j]
			switch ch {
			case 'r':
				opts.Reverse = true
				j++
			case 'F':
				opts.Flip = true
				j++
			case 's':
				opts.Strip = true
				j++
			case 'n':
				opts.Normalize = true
				j++
			case 'f', 'w', 'j', 'D':
				val := ""
				if j+1 < len(cluster) {
					val = cluster[j+1:]
					j = len(cluster)
				} else {
					if i+1 >= len(args) {
						return nil, fmt.Errorf("option -%c requires argument", ch)
					}
					i++
					val = args[i]
					j = len(cluster)
				}
				switch ch {
				case 'f':
					opts.Font = val
				case 'w':
					w, err := strconv.Atoi(val)
					if err != nil {
						return nil, fmt.Errorf("invalid width: %v", err)
					}
					opts.Width = w
				case 'j':
					if val != "auto" && val != "left" && val != "center" && val != "right" {
						return nil, fmt.Errorf("invalid justify: %s", val)
					}
					opts.Justify = val
				case 'D':
					if val != "auto" && val != "left-to-right" && val != "right-to-left" {
						return nil, fmt.Errorf("invalid direction: %s", val)
					}
					opts.Direction = val
				}
			default:
				return nil, fmt.Errorf("unknown option -%c", ch)
			}
		}
		i++
	}

	opts.Args = positional
	return opts, nil
}

// splitLines splits a string on \n, \r\n, and \r line endings,
// and matches Python's str.splitlines() semantics:
// a trailing line terminator does NOT produce a trailing empty string.
// e.g., "a\nb\n" -> ["a", "b"]
//       "a\nb\n\n" -> ["a", "b", ""]
//       "a\nb" -> ["a", "b"]
func pyLines(s string) []string {
	if len(s) == 0 {
		return nil
	}
	var res []string
	start := 0
	i := 0
	n := len(s)
	for i < n {
		if s[i] == '\r' {
			res = append(res, s[start:i])
			if i+1 < n && s[i+1] == '\n' {
				i++
			}
			i++
			start = i
		} else if s[i] == '\n' {
			res = append(res, s[start:i])
			i++
			start = i
		} else {
			i++
		}
	}
	// If there is remaining content after the last separator, include it.
	// But if the string ended with a separator (start == n), don't add trailing "".
	if start < n {
		res = append(res, s[start:])
	}
	return res
}

// splitLinesForParsing splits a string on \n for font parsing purposes.
// Unlike pyLines, this keeps interior blank lines needed for font char boundaries.
func splitLinesForParsing(s string) []string {
	if len(s) == 0 {
		return nil
	}
	var res []string
	start := 0
	i := 0
	n := len(s)
	for i < n {
		if s[i] == '\r' {
			if i+1 < n && s[i+1] == '\n' {
				res = append(res, s[start:i])
				i += 2
				start = i
			} else {
				res = append(res, s[start:i])
				i++
				start = i
			}
		} else if s[i] == '\n' {
			res = append(res, s[start:i])
			i++
			start = i
		} else {
			i++
		}
	}
	if start < n {
		res = append(res, s[start:])
	}
	return res
}

func parseChar(lines *[]string, height int) (int, []string, error) {
	if len(*lines) < height {
		return 0, nil, fmt.Errorf("unexpected end of font data")
	}
	var endChar rune
	hasEndChar := false
	width := 0
	res := make([]string, 0, height)

	for row := 0; row < height; row++ {
		line := (*lines)[0]
		*lines = (*lines)[1:]

		if !hasEndChar {
			runes := []rune(line)
			idx := len(runes) - 1
			for idx >= 0 && unicode.IsSpace(runes[idx]) {
				idx--
			}
			if idx < 0 {
				return 0, nil, fmt.Errorf("line has no end marker")
			}
			endChar = runes[idx]
			hasEndChar = true
		}

		runes := []rune(line)
		idx := len(runes) - 1
		for idx >= 0 && unicode.IsSpace(runes[idx]) {
			idx--
		}
		count := 0
		for count < 2 && idx >= 0 && runes[idx] == endChar {
			count++
			idx--
		}
		if count > 0 {
			line = string(runes[:idx+1])
		}

		rLen := utf8.RuneCountInString(line)
		if rLen > width {
			width = rLen
		}
		res = append(res, line)
	}

	return width, res, nil
}

func loadFontData(fontName string) ([]byte, error) {
	extensions := []string{"tlf", "flf"}
	var names []string
	if strings.HasSuffix(fontName, ".tlf") || strings.HasSuffix(fontName, ".flf") {
		names = append(names, fontName)
	}
	for _, ext := range extensions {
		names = append(names, fontName+"."+ext)
	}

	// 1. Embedded filesystem (always works regardless of cwd)
	for _, name := range names {
		base := filepath.Base(name)
		data, err := embeddedFonts.ReadFile("fonts/" + base)
		if err == nil && len(data) > 0 {
			return data, nil
		}
	}

	// 2. Local search directories (fallback)
	searchDirs := []string{
		".",
		"fonts",
		"pyfiglet/fonts",
		"/usr/local/share/pyfiglet",
		"/usr/local/share/figlet",
		"/usr/share/figlet",
	}
	for _, name := range names {
		for _, dir := range searchDirs {
			p := filepath.Join(dir, name)
			if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
				return data, nil
			}
		}
	}

	// 3. Direct absolute path
	if data, err := os.ReadFile(fontName); err == nil && len(data) > 0 {
		return data, nil
	}

	return nil, fmt.Errorf("requested font '%s' not found", fontName)
}

func loadFont(fontName string) (*Font, error) {
	rawBytes, err := loadFontData(fontName)
	if err != nil {
		return nil, err
	}

	// Handle ZIP-encoded fonts
	if len(rawBytes) >= 4 && rawBytes[0] == 'P' && rawBytes[1] == 'K' && rawBytes[2] == 0x03 && rawBytes[3] == 0x04 {
		zr, err := zip.NewReader(bytes.NewReader(rawBytes), int64(len(rawBytes)))
		if err == nil && len(zr.File) > 0 {
			rc, err := zr.File[0].Open()
			if err == nil {
				defer rc.Close()
				if unzipped, err := io.ReadAll(rc); err == nil {
					rawBytes = unzipped
				}
			}
		}
	}

	// Replace Unicode line-splitting characters (not \n or \r) with space,
	// to match PyFiglet's re.sub(r"[\u0085\u2028\u2029]", " ", data)
	rawStr := string(rawBytes)
	var b strings.Builder
	b.Grow(len(rawStr))
	for _, r := range rawStr {
		if r == '\u0085' || r == '\u2028' || r == '\u2029' {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}

	lines := splitLinesForParsing(b.String())
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty font data for %s", fontName)
	}

	headerLine := lines[0]
	lines = lines[1:]

	// Validate magic number: must start with "flf2" or "tlf2"
	if len(headerLine) < 5 {
		return nil, fmt.Errorf("invalid header for %s", fontName)
	}
	prefix := headerLine[:4]
	if prefix != "flf2" && prefix != "tlf2" {
		return nil, fmt.Errorf("%s is not a valid figlet font", fontName)
	}

	// Parse header fields (after the 5th char which is the hardblank)
	remHeader := headerLine[5:]
	fields := strings.Fields(remHeader)
	if len(fields) < 6 {
		return nil, fmt.Errorf("malformed header for %s", fontName)
	}

	hardBlankRunes := []rune(fields[0])
	if len(hardBlankRunes) == 0 {
		return nil, fmt.Errorf("missing hardblank in %s", fontName)
	}
	hardBlank := hardBlankRunes[0]

	height, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, err
	}
	baseLine, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, err
	}
	maxLength, err := strconv.Atoi(fields[3])
	if err != nil {
		return nil, err
	}
	oldLayout, err := strconv.Atoi(fields[4])
	if err != nil {
		return nil, err
	}
	commentLines, err := strconv.Atoi(fields[5])
	if err != nil {
		return nil, err
	}

	printDirection := 0
	if len(fields) > 6 {
		if pDir, err := strconv.Atoi(fields[6]); err == nil {
			printDirection = pDir
		}
	}

	var fullLayout *int
	if len(fields) > 7 {
		if fLay, err := strconv.Atoi(fields[7]); err == nil {
			fullLayout = &fLay
		}
	}

	// Convert old layout to smushMode if fullLayout not present
	smushMode := 0
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
		return nil, fmt.Errorf("not enough lines for comments in %s", fontName)
	}
	lines = lines[commentLines:]

	chars := make(map[rune][]string)
	widths := make(map[rune]int)

	// Load standard ASCII character set (32..126)
	for i := 32; i < 127; i++ {
		w, letter, err := parseChar(&lines, height)
		if err != nil {
			return nil, err
		}
		allEmpty := true
		for _, s := range letter {
			if s != "" {
				allEmpty = false
				break
			}
		}
		// Always store space (32), only store others if non-empty
		if i == 32 || !allEmpty {
			chars[rune(i)] = letter
			widths[rune(i)] = w
		}
	}

	// Load German Umlaute (follow directly after char 127)
	german := []rune("ÄÖÜäöüß")
	if len(lines) >= height*len(german) {
		for _, r := range german {
			w, letter, err := parseChar(&lines, height)
			if err != nil {
				break
			}
			allEmpty := true
			for _, s := range letter {
				if s != "" {
					allEmpty = false
					break
				}
			}
			if !allEmpty {
				chars[r] = letter
				widths[r] = w
			}
		}
	}

	// Load extended character set (tagged with hex code comment)
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		lines = lines[1:]
		if line == "" {
			continue
		}
		codeStr := strings.SplitN(line, " ", 2)[0]
		if strings.HasPrefix(strings.ToLower(codeStr), "0x") {
			code, err := strconv.ParseInt(codeStr[2:], 16, 64)
			if err == nil {
				w, letter, err := parseChar(&lines, height)
				if err == nil {
					allEmpty := true
					for _, s := range letter {
						if s != "" {
							allEmpty = false
							break
						}
					}
					if !allEmpty {
						chars[rune(code)] = letter
						widths[rune(code)] = w
					}
				}
			}
		}
	}

	return &Font{
		name:           fontName,
		hardBlank:      hardBlank,
		height:         height,
		baseLine:       baseLine,
		maxLength:      maxLength,
		oldLayout:      oldLayout,
		commentLines:   commentLines,
		printDirection: printDirection,
		smushMode:      smushMode,
		chars:          chars,
		widths:         widths,
	}, nil
}

type blankMarker struct {
	buffer   []string
	iterator int
}

type FigletBuilder struct {
	font          *Font
	text          []rune
	direction     string
	justify       string
	width         int
	iterator      int
	curCharWidth  int
	prevCharWidth int
	maxSmush      int
	buffer        []string
	blankMarkers  []blankMarker
	product       [][]string
}

func newFigletBuilder(font *Font, text string, direction, justify string, width int) *FigletBuilder {
	if direction == "auto" {
		if font.printDirection == 1 {
			direction = "right-to-left"
		} else {
			direction = "left-to-right"
		}
	}
	if justify == "auto" {
		if direction == "left-to-right" {
			justify = "left"
		} else {
			justify = "right"
		}
	}

	buf := make([]string, font.height)
	return &FigletBuilder{
		font:      font,
		text:      []rune(text),
		direction: direction,
		justify:   justify,
		width:     width,
		buffer:    buf,
	}
}

// smushChars implements PyFiglet's smushChars logic exactly.
// left and right are the touching characters from left-buffer and right-character
// (after swapping for RTL). Returns (result, true) if smushing is possible.
func (b *FigletBuilder) smushChars(left, right rune) (rune, bool) {
	// Space always yields to the other character (before width check)
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}

	// No sub-cell overlap for narrow glyphs
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}

	// Kerning-only mode: no actual smushing
	if (b.font.smushMode & SM_SMUSH) == 0 {
		return 0, false
	}

	hardBlank := b.font.hardBlank

	// Universal overlap mode (SM_SMUSH set, but none of the 6 rule bits set)
	if (b.font.smushMode & 63) == 0 {
		// Hardblank yields to visible character
		if left == hardBlank {
			return right, true
		}
		if right == hardBlank {
			return left, true
		}
		// Later input character wins
		// In RTL mode, the new char is on the left (left argument after swap)
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}

	// Rule-based smushing
	// SM_HARDBLANK: two hardblanks smush to hardblank
	if (b.font.smushMode & SM_HARDBLANK) != 0 {
		if left == hardBlank && right == hardBlank {
			return left, true
		}
	}
	// Either hardblank but not both: no smush in rule mode
	if left == hardBlank || right == hardBlank {
		return 0, false
	}

	// SM_EQUAL: identical characters smush to themselves
	if (b.font.smushMode & SM_EQUAL) != 0 {
		if left == right {
			return left, true
		}
	}

	// SM_LOWLINE: underscore smushes with hierarchy characters
	if (b.font.smushMode & SM_LOWLINE) != 0 {
		lowlineClass := "|/\\[]{}()<>"
		if left == '_' && strings.ContainsRune(lowlineClass, right) {
			return right, true
		}
		if right == '_' && strings.ContainsRune(lowlineClass, left) {
			return left, true
		}
	}

	// SM_HIERARCHY: higher class wins
	if (b.font.smushMode & SM_HIERARCHY) != 0 {
		classOf := func(r rune) int {
			switch r {
			case '|':
				return 1
			case '/', '\\':
				return 2
			case '[', ']':
				return 3
			case '{', '}':
				return 4
			case '(', ')':
				return 5
			case '<', '>':
				return 6
			default:
				return 0
			}
		}
		cLeft := classOf(left)
		cRight := classOf(right)
		if cLeft > 0 && cRight > 0 && cLeft != cRight {
			if cLeft < cRight {
				return right, true
			}
			return left, true
		}
	}

	// SM_PAIR: opposite pairs become |
	if (b.font.smushMode & SM_PAIR) != 0 {
		if (left == '[' && right == ']') || (left == ']' && right == '[') ||
			(left == '{' && right == '}') || (left == '}' && right == '{') ||
			(left == '(' && right == ')') || (left == ')' && right == '(') {
			return '|', true
		}
	}

	// SM_BIGX: special X crossings
	if (b.font.smushMode & SM_BIGX) != 0 {
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

// currentSmushAmount calculates how many columns the new character can overlap with the buffer.
func (b *FigletBuilder) currentSmushAmount(curChar []string) int {
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

		rLeft := []rune(lineLeft)
		rRight := []rune(lineRight)

		// Find rightmost non-space in left string
		k := len(rLeft) - 1
		for k >= 0 && rLeft[k] == ' ' {
			k--
		}
		linebd := k
		if linebd < 0 {
			linebd = 0
		}
		var ch1 rune
		hasCh1 := false
		if linebd < len(rLeft) {
			ch1 = rLeft[linebd]
			hasCh1 = true
		} else {
			linebd = 0
		}

		// Find leftmost non-space in right string
		charbd := 0
		for charbd < len(rRight) && rRight[charbd] == ' ' {
			charbd++
		}
		var ch2 rune
		hasCh2 := false
		if charbd < len(rRight) {
			ch2 = rRight[charbd]
			hasCh2 = true
		}

		amt := charbd + len(rLeft) - 1 - linebd

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

func (b *FigletBuilder) smushRow(curChar []string, row int) ([]rune, []rune) {
	addLeft := []rune(b.buffer[row])
	addRight := []rune(curChar[row])

	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		var left rune
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		var right rune
		if i < len(addRight) {
			right = addRight[i]
		}
		smushed, ok := b.smushChars(left, right)
		if ok && idx >= 0 && idx < len(addLeft) {
			addLeft[idx] = smushed
		}
	}

	return addLeft, addRight
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
	addLeft, addRight := b.smushRow(curChar, row)
	var rightTail []rune
	if b.maxSmush < len(addRight) {
		rightTail = addRight[b.maxSmush:]
	}
	b.buffer[row] = string(append(addLeft, rightTail...))
}

func (b *FigletBuilder) cutBufferCommon() {
	b.buffer = make([]string, b.font.height)
	b.blankMarkers = nil
	b.prevCharWidth = 0
	if b.iterator >= 0 && b.iterator < len(b.text) {
		c := b.text[b.iterator]
		if curChar, ok := b.font.chars[c]; ok {
			b.maxSmush = b.currentSmushAmount(curChar)
		}
	}
}

func (b *FigletBuilder) cutBufferAtLastBlank(savedBuffer []string, savedIterator int) {
	b.product = append(b.product, savedBuffer)
	b.iterator = savedIterator
	b.cutBufferCommon()
}

func (b *FigletBuilder) cutBufferAtLastChar() {
	bufCopy := make([]string, len(b.buffer))
	copy(bufCopy, b.buffer)
	b.product = append(b.product, bufCopy)
	b.iterator--
	b.cutBufferCommon()
}

func (b *FigletBuilder) getLastBlank() ([]string, int, bool) {
	if len(b.blankMarkers) == 0 {
		return nil, -1, false
	}
	last := b.blankMarkers[len(b.blankMarkers)-1]
	b.blankMarkers = b.blankMarkers[:len(b.blankMarkers)-1]
	return last.buffer, last.iterator, true
}

func (b *FigletBuilder) handleNewLine() {
	savedBuffer, savedIterator, ok := b.getLastBlank()
	if ok && savedIterator != -1 {
		b.cutBufferAtLastBlank(savedBuffer, savedIterator)
	} else {
		b.cutBufferAtLastChar()
	}
}

func (b *FigletBuilder) copyBuffer() []string {
	cp := make([]string, len(b.buffer))
	copy(cp, b.buffer)
	return cp
}

func (b *FigletBuilder) render() (string, error) {
	for b.iterator < len(b.text) {
		c := b.text[b.iterator]

		// Newline in input: flush current buffer immediately
		if c == '\n' {
			b.blankMarkers = append(b.blankMarkers, blankMarker{
				buffer:   b.copyBuffer(),
				iterator: b.iterator,
			})
			b.handleNewLine()
			b.iterator++
			continue
		}

		curChar, ok := b.font.chars[c]
		if !ok {
			// Unknown character: skip
			b.iterator++
			continue
		}

		cw := b.font.widths[c]
		if b.width < cw {
			return "", fmt.Errorf("width is not enough to print this character")
		}

		b.curCharWidth = cw
		b.maxSmush = b.currentSmushAmount(curChar)

		bufferRowLen := 0
		if len(b.buffer) > 0 {
			bufferRowLen = utf8.RuneCountInString(b.buffer[0])
		}
		currentTotalWidth := bufferRowLen + b.curCharWidth - b.maxSmush

		// Record spaces as potential line-break points
		if c == ' ' {
			b.blankMarkers = append(b.blankMarkers, blankMarker{
				buffer:   b.copyBuffer(),
				iterator: b.iterator,
			})
		}

		if currentTotalWidth >= b.width {
			b.handleNewLine()
		} else {
			for row := 0; row < b.font.height; row++ {
				b.addCurCharRowToBufferRow(curChar, row)
			}
		}

		b.prevCharWidth = b.curCharWidth
		b.iterator++
	}

	// Flush remaining buffer
	if len(b.buffer) > 0 && b.buffer[0] != "" {
		b.product = append(b.product, b.copyBuffer())
	}

	var stringAcc strings.Builder
	hardBlankStr := string(b.font.hardBlank)

	for _, buf := range b.product {
		rowCopies := make([]string, len(buf))
		copy(rowCopies, buf)

		if b.justify == "right" {
			for row := 0; row < b.font.height; row++ {
				rLen := utf8.RuneCountInString(rowCopies[row])
				pad := b.width - rLen - 1
				if pad > 0 {
					rowCopies[row] = strings.Repeat(" ", pad) + rowCopies[row]
				}
			}
		} else if b.justify == "center" {
			for row := 0; row < b.font.height; row++ {
				rLen := utf8.RuneCountInString(rowCopies[row])
				pad := (b.width - rLen) / 2
				if pad > 0 {
					rowCopies[row] = strings.Repeat(" ", pad) + rowCopies[row]
				}
			}
		}

		line := strings.Join(rowCopies, "\n") + "\n"
		line = strings.ReplaceAll(line, hardBlankStr, " ")
		stringAcc.WriteString(line)
	}

	return stringAcc.String(), nil
}

// reverseText mirrors each line left-right, applying PyFiglet's __reverse_map__
// character substitutions.  Uses pyLines() (matches Python str.splitlines())
// so the trailing '\n' on the render output does not produce a spurious blank line.
func reverseText(s string) string {
	lines := pyLines(s)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		runes := []rune(line)
		// Apply character map (symmetric pair swaps) then reverse string order.
		// Python does: row.translate(map)[::-1]
		// Since every swap is symmetric (( <-> ), etc.) the order is equivalent
		// to: reverse order then apply map. We apply map first, then reverse.
		for idx, r := range runes {
			switch r {
			case '(':
				runes[idx] = ')'
			case ')':
				runes[idx] = '('
			case '<':
				runes[idx] = '>'
			case '>':
				runes[idx] = '<'
			case '[':
				runes[idx] = ']'
			case ']':
				runes[idx] = '['
			case '{':
				runes[idx] = '}'
			case '}':
				runes[idx] = '{'
			case '/':
				runes[idx] = '\\'
			case '\\':
				runes[idx] = '/'
			}
		}
		// Reverse character order
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

// flipText reverses the lines top-to-bottom and applies PyFiglet's __flip_map__
// character substitutions.  Uses pyLines() for correct trailing-newline handling.
func flipText(s string) string {
	lines := pyLines(s)
	// Reverse line order
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		runes := []rune(line)
		for idx, r := range runes {
			switch r {
			case '/':
				runes[idx] = '\\'
			case '\\':
				runes[idx] = '/'
			case 'A':
				runes[idx] = 'V'
			case 'V':
				runes[idx] = 'A'
			case 'M':
				runes[idx] = 'W'
			case 'W':
				runes[idx] = 'M'
			case 'P', 'R':
				runes[idx] = 'b'
			case 'b':
				runes[idx] = 'P'
			case '^':
				runes[idx] = 'v'
			case 'v':
				runes[idx] = '^'
			case '_':
				runes[idx] = '-'
			case 'm':
				runes[idx] = 'w'
			case 'w':
				runes[idx] = 'm'
			}
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n") + "\n"
}

// stripSurroundingNewlines matches PyFiglet's FigletString.strip_surrounding_newlines():
// drops leading lines whose strip() is empty, then rstrips the entire result.
func stripSurroundingNewlines(s string) string {
	lines := pyLines(s)
	var out []string
	charsSeen := false
	for _, line := range lines {
		if strings.TrimSpace(line) != "" || charsSeen {
			charsSeen = true
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return ""
	}
	joined := strings.Join(out, "\n") + "\n"
	return strings.TrimRight(joined, " \t\r\n")
}

// normalizeSurroundingNewlines matches PyFiglet's normalize_surrounding_newlines():
// '\n' + strip_surrounding_newlines() + '\n'
func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}

func main() {
	opts, err := parseCLI(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: %v\n", err)
		os.Exit(1)
	}

	if len(opts.Args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: figlet [options] [text..]\n")
		os.Exit(1)
	}

	text := strings.Join(opts.Args, " ")

	font, err := loadFont(opts.Font)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: requested font '%s' not found.\n", opts.Font)
		os.Exit(1)
	}

	builder := newFigletBuilder(font, text, opts.Direction, opts.Justify, opts.Width)
	res, err := builder.render()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyfiglet error: %v\n", err)
		os.Exit(1)
	}

	// Post-processing order: reverse, then flip, then newline handling
	if opts.Reverse {
		res = reverseText(res)
	}
	if opts.Flip {
		res = flipText(res)
	}

	// -s takes precedence over -n
	if opts.Strip {
		res = stripSurroundingNewlines(res)
	} else if opts.Normalize {
		res = normalizeSurroundingNewlines(res)
	}

	// Write result then one extra newline (matching pyfiglet's main())
	os.Stdout.WriteString(res)
	os.Stdout.WriteString("\n")
}
GOEOF

# 5. Build executable and verify builds succeed
go build -o /tmp/figlet .
go build ./...

# 6. Smoke tests: verify font discovery works from any cwd
(cd /tmp && /tmp/figlet -f standard "Go Figlet" > /dev/null)
(cd / && /tmp/figlet -f slant "test" > /dev/null)

# 7. Verify non-zero exit on missing font
if /tmp/figlet -f doesnotexist hello > /dev/null 2>&1; then
    echo "ERROR: missing font should have returned non-zero" >&2
    exit 1
fi

# 8. Verify non-zero exit on missing text
if /tmp/figlet > /dev/null 2>&1; then
    echo "ERROR: missing text should have returned non-zero" >&2
    exit 1
fi

echo "All build and smoke tests passed."
