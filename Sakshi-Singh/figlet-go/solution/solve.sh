#!/bin/bash
# Oracle for figlet-go: writes a Go port of pyfiglet into /workspace/repo, keeps the
# .flf fonts in pyfiglet/fonts/ (embedded into the binary with go:embed), and removes
# every .py file. Go standard library only.
set -euo pipefail
cd /workspace/repo

cat > go.mod <<'GO_SOURCE_EOF'
module figlet

go 1.23
GO_SOURCE_EOF

cat > main.go <<'GO_SOURCE_EOF'
// figlet - a Go port of pyfiglet (commit 255827efbc09d662875dad50fa0edc1e1263bf45).
// Output is byte-for-byte identical to `python -m pyfiglet`.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const version = "1.0.4"

// FigletString.reverse / flip translation tables (only the entries that change).
var reverseMap = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<',
	'[': ']', ']': '[', '{': '}', '}': '{',
}

var flipMap = map[rune]rune{
	'/': '\\', '\\': '/', 'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M', 'P': 'b', 'R': 'b',
	'b': 'P', 'm': 'w', 'w': 'm', 'v': '^', '^': 'v', '_': '-',
}

func translate(row []rune, table map[rune]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if t, ok := table[r]; ok {
			out[i] = t
		} else {
			out[i] = r
		}
	}
	return out
}

func newFromList(rows [][]rune) []rune {
	return append(joinLines(rows), '\n')
}

func reverseFig(s []rune) []rune {
	var out [][]rune
	for _, row := range splitLines(s) {
		t := translate(row, reverseMap)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		out = append(out, t)
	}
	return newFromList(out)
}

func flipFig(s []rune) []rune {
	rows := splitLines(s)
	var out [][]rune
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translate(rows[i], flipMap))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s []rune) []rune {
	var out [][]rune
	seen := false
	for _, row := range splitLines(s) {
		if len(strip(row)) != 0 || seen {
			seen = true
			out = append(out, row)
		}
	}
	return rstrip(newFromList(out))
}

func normalizeSurroundingNewlines(s []rune) []rune {
	out := []rune{'\n'}
	out = append(out, stripSurroundingNewlines(s)...)
	return append(out, '\n')
}

// ---------------------------------------------------------------- colours (-c)

var colorCodes = map[string]int{
	"BLACK": 30, "RED": 31, "GREEN": 32, "YELLOW": 33, "BLUE": 34, "MAGENTA": 35, "CYAN": 36,
	"LIGHT_GRAY": 37, "DEFAULT": 39, "DARK_GRAY": 90, "LIGHT_RED": 91, "LIGHT_GREEN": 92,
	"LIGHT_YELLOW": 93, "LIGHT_BLUE": 94, "LIGHT_MAGENTA": 95, "LIGHT_CYAN": 96, "WHITE": 97,
	"RESET": 0,
}

func colorToANSI(color string, background bool) (string, error) {
	if color == "" {
		return "", nil
	}
	color = strings.ToUpper(color)
	n := strings.Count(color, ";")
	code, known := colorCodes[color]
	if n > 0 && n != 2 {
		return "", fmt.Errorf("Specified color '{}' not a valid color in R;G;B format")
	}
	if n == 0 && !known {
		return "", fmt.Errorf("Specified color '%s' not found in ANSI COLOR_CODES list", color)
	}
	if known {
		if background {
			code += 10
		}
		return fmt.Sprintf("\033[%dm", code), nil
	}
	base := 38
	if background {
		base = 48
	}
	return fmt.Sprintf("\033[%d;2;%sm", base, color), nil
}

func parseColor(color string) (string, error) {
	fg, bg, _ := strings.Cut(color, ":")
	a, err := colorToANSI(fg, false)
	if err != nil {
		return "", err
	}
	b, err := colorToANSI(bg, true)
	if err != nil {
		return "", err
	}
	return a + b, nil
}

// ---------------------------------------------------------------- options (optparse)

type options struct {
	font, direction, justify, color, load string
	width                                 int
	reverse, normalize, strip, flip       bool
	listFonts, infoFont                   bool
}

type optSpec struct {
	short, long string
	takesValue  bool
	choices     []string
}

var specs = []optSpec{
	{"-f", "--font", true, nil},
	{"-D", "--direction", true, []string{"auto", "left-to-right", "right-to-left"}},
	{"-j", "--justify", true, []string{"auto", "left", "center", "right"}},
	{"-w", "--width", true, nil},
	{"-r", "--reverse", false, nil},
	{"-n", "--normalize-surrounding-newlines", false, nil},
	{"-s", "--strip-surrounding-newlines", false, nil},
	{"-F", "--flip", false, nil},
	{"-l", "--list_fonts", false, nil},
	{"-i", "--info_font", false, nil},
	{"-L", "--load", true, nil},
	{"-c", "--color", true, nil},
	{"", "--version", false, nil},
	{"-h", "--help", false, nil},
}

const usage = "Usage: pyfiglet [options] [text..]\n"

func optError(msg string) {
	fmt.Fprint(os.Stderr, usage+"\n")
	fmt.Fprintf(os.Stderr, "pyfiglet: error: %s\n", msg)
	os.Exit(2)
}

// optparse's _parse_num for type="int".
func parseOptInt(v string) (int, error) {
	lv := strings.ToLower(v)
	base, s := 10, v
	switch {
	case strings.HasPrefix(lv, "0x"):
		base, s = 16, v[2:]
	case strings.HasPrefix(lv, "0b"):
		base, s = 2, v[2:]
		if s == "" {
			s = "0"
		}
	case strings.HasPrefix(v, "0"):
		base = 8
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), base, 64)
	return int(n), err
}

func apply(o *options, sp optSpec, name, val string) {
	if sp.choices != nil {
		ok := false
		for _, c := range sp.choices {
			ok = ok || c == val
		}
		if !ok {
			quoted := make([]string, len(sp.choices))
			for i, c := range sp.choices {
				quoted[i] = "'" + c + "'"
			}
			optError(fmt.Sprintf("option %s: invalid choice: '%s' (choose from %s)", name, val, strings.Join(quoted, ", ")))
		}
	}
	switch sp.long {
	case "--font":
		o.font = val
	case "--direction":
		o.direction = val
	case "--justify":
		o.justify = val
	case "--width":
		n, err := parseOptInt(val)
		if err != nil {
			optError(fmt.Sprintf("option %s: invalid integer value: '%s'", name, val))
		}
		o.width = n
	case "--reverse":
		o.reverse = true
	case "--normalize-surrounding-newlines":
		o.normalize = true
	case "--strip-surrounding-newlines":
		o.strip = true
	case "--flip":
		o.flip = true
	case "--list_fonts":
		o.listFonts = true
	case "--info_font":
		o.infoFont = true
	case "--load":
		o.load = val
	case "--color":
		o.color = val
	case "--version":
		fmt.Println(version)
		os.Exit(0)
	case "--help":
		fmt.Print(usage)
		os.Exit(0)
	}
}

func parseArgs(argv []string) (*options, []string) {
	o := &options{font: "standard", direction: "auto", justify: "auto", width: 80, color: ":"}
	var args []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			return o, append(args, argv[i+1:]...)
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a, "=")
			var match *optSpec
			for k := range specs {
				if specs[k].long == name {
					match = &specs[k]
					break
				}
			}
			if match == nil { // unique-prefix abbreviations
				var cands []*optSpec
				for k := range specs {
					if strings.HasPrefix(specs[k].long, name) {
						cands = append(cands, &specs[k])
					}
				}
				if len(cands) == 1 {
					match = cands[0]
				} else if len(cands) > 1 {
					optError(fmt.Sprintf("ambiguous option: %s", name))
				} else {
					optError(fmt.Sprintf("no such option: %s", name))
				}
			}
			if match.takesValue {
				if !hasVal {
					if i+1 >= len(argv) {
						optError(fmt.Sprintf("%s option requires 1 argument", match.long))
					}
					i++
					val = argv[i]
				}
			} else if hasVal {
				optError(fmt.Sprintf("%s option does not take a value", match.long))
			}
			apply(o, *match, match.long, val)
		case len(a) > 1 && a[0] == '-':
			rs := []rune(a)
			for j := 1; j < len(rs); j++ {
				opt := "-" + string(rs[j])
				var match *optSpec
				for k := range specs {
					if specs[k].short == opt {
						match = &specs[k]
					}
				}
				if match == nil {
					optError(fmt.Sprintf("no such option: %s", opt))
				}
				if match.takesValue {
					val := string(rs[j+1:])
					if val == "" {
						if i+1 >= len(argv) {
							optError(fmt.Sprintf("%s option requires 1 argument", opt))
						}
						i++
						val = argv[i]
					}
					apply(o, *match, opt, val)
					break
				}
				apply(o, *match, opt, "")
			}
		default:
			args = append(args, a)
		}
	}
	return o, args
}

// ---------------------------------------------------------------- main

func run() int {
	o, args := parseArgs(os.Args[1:])

	if o.listFonts || o.infoFont || o.load != "" {
		fmt.Fprintln(os.Stderr, "pyfiglet: -l, -i and -L are not supported by this port")
		return 1
	}
	if o.color == "list" {
		fmt.Println("[0-255];[0-255];[0-255] # RGB")
		return 0
	}
	if len(args) == 0 {
		fmt.Print(usage)
		return 1
	}
	text := []rune(strings.Join(args, " "))

	font, err := loadFont(o.font)
	if err == errFontNotFound {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", o.font)
		return 1
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet.FontError:", err)
		return 1
	}

	direction := o.direction
	if direction == "auto" {
		direction = "left-to-right"
		if font.printDirection == 1 {
			direction = "right-to-left"
		}
	}
	justify := o.justify
	if justify == "auto" {
		justify = "left"
		if direction == "right-to-left" {
			justify = "right"
		}
	}

	r, err := newBuilder(text, font, direction, o.width, justify).render()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet error:", err)
		return 1
	}
	if o.reverse {
		r = reverseFig(r)
	}
	if o.flip {
		r = flipFig(r)
	}
	if o.strip {
		r = stripSurroundingNewlines(r)
	} else if o.normalize {
		r = normalizeSurroundingNewlines(r)
	}

	ansi, err := parseColor(o.color)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pyfiglet.InvalidColor:", err)
		return 1
	}
	var sb strings.Builder
	sb.WriteString(ansi)
	sb.WriteString(string(r))
	sb.WriteString("\n")
	if ansi != "" {
		sb.WriteString("\033[0m")
	}
	os.Stdout.WriteString(sb.String())
	return 0
}

func main() {
	os.Exit(run())
}
GO_SOURCE_EOF

cat > pystr.go <<'GO_SOURCE_EOF'
package main

// Helpers that reproduce the exact Python str semantics pyfiglet relies on.
// All text is handled as []rune because Python's len() and indexing count code points.

import (
	"unicode"
	"unicode/utf8"
)

// isSpace matches Python's str.isspace() (and therefore re's \s, str.split(), str.strip()).
func isSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// isLineBreak matches the separators of Python's str.splitlines().
func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', 0x0b, 0x0c, 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// splitLines is Python's str.splitlines() (no keepends).
func splitLines(s []rune) [][]rune {
	var out [][]rune
	start := 0
	for i := 0; i < len(s); i++ {
		if isLineBreak(s[i]) {
			out = append(out, s[start:i])
			if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// strip is Python's str.strip() with no arguments.
func strip(s []rune) []rune {
	a, b := 0, len(s)
	for a < b && isSpace(s[a]) {
		a++
	}
	for b > a && isSpace(s[b-1]) {
		b--
	}
	return s[a:b]
}

// rstrip is Python's str.rstrip() with no arguments.
func rstrip(s []rune) []rune {
	b := len(s)
	for b > 0 && isSpace(s[b-1]) {
		b--
	}
	return s[:b]
}

// fields is Python's str.split() with no arguments.
func fields(s []rune) [][]rune {
	var out [][]rune
	i := 0
	for i < len(s) {
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if i >= len(s) {
			break
		}
		j := i
		for j < len(s) && !isSpace(s[j]) {
			j++
		}
		out = append(out, s[i:j])
		i = j
	}
	return out
}

func joinLines(rows [][]rune) []rune {
	var out []rune
	for i, r := range rows {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, r...)
	}
	return out
}

func cloneRunes(s []rune) []rune {
	return append([]rune(nil), s...)
}

// decodeUTF8Replace is Python's bytes.decode('UTF-8', 'replace'): every maximal invalid
// subpart becomes one U+FFFD (Go's []rune conversion would use one per byte).
func decodeUTF8Replace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			out = append(out, rune(c))
			i++
			continue
		}
		if r, size := utf8.DecodeRune(b[i:]); r != utf8.RuneError || size > 1 {
			out = append(out, r)
			i += size
			continue
		}
		// Invalid: consume the longest prefix that could still start a valid sequence.
		need, lo, hi := 0, byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c == 0xE0:
			need, lo = 2, 0xA0
		case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
			need = 2
		case c == 0xED:
			need, hi = 2, 0x9F
		case c == 0xF0:
			need, lo = 3, 0x90
		case c >= 0xF1 && c <= 0xF3:
			need = 3
		case c == 0xF4:
			need, hi = 3, 0x8F
		}
		j := i + 1
		for k := 0; k < need && j < len(b); k++ {
			if b[j] < lo || b[j] > hi {
				break
			}
			lo, hi = 0x80, 0xBF
			j++
		}
		out = append(out, utf8.RuneError)
		i = j
	}
	return out
}
GO_SOURCE_EOF

cat > font.go <<'GO_SOURCE_EOF'
package main

// Port of pyfiglet.FigletFont: locating and parsing .flf / .tlf font files.

import (
	"archive/zip"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed pyfiglet/fonts
var embeddedFonts embed.FS

var fontDirectories = []string{"/usr/local/share/pyfiglet/", "/usr/local/share/figlet", "/usr/share/figlet"}

var errFontNotFound = errors.New("font not found")

type Font struct {
	name           string
	hardBlank      []rune // header[0]; normally one character
	height         int
	printDirection int // -1 when the header does not give one
	smushMode      int
	chars          map[int64][][]rune
	width          map[int64]int
}

// hb returns the hardblank as a single rune, or -2 (never equal to a cell) if it is not
// exactly one character long.
func (f *Font) hb() rune {
	if len(f.hardBlank) == 1 {
		return f.hardBlank[0]
	}
	return -2
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// preloadFont mirrors FigletFont.preloadFont, including its search order.
func preloadFont(font string) ([]rune, error) {
	var data []byte
	found := false
	var embeddedName, diskName string
	for _, ext := range []string{"tlf", "flf"} {
		fn := font + "." + ext
		name := "pyfiglet/fonts/" + fn
		if b, err := embeddedFonts.ReadFile(name); err == nil {
			_ = b
			embeddedName, diskName, found = name, "", true
			break
		}
		for _, loc := range append([]string{"./"}, fontDirectories...) {
			full := filepath.Join(loc, fn)
			if isFile(full) {
				embeddedName, diskName, found = "", full, true
				break
			}
		}
	}
	if found {
		var err error
		if embeddedName != "" {
			data, err = embeddedFonts.ReadFile(embeddedName)
		} else {
			data, err = os.ReadFile(diskName)
		}
		if err != nil {
			return nil, err
		}
		if zr, zerr := zip.NewReader(bytes.NewReader(data), int64(len(data))); zerr == nil && len(zr.File) > 0 {
			rc, err := zr.File[0].Open()
			if err != nil {
				return nil, err
			}
			var buf bytes.Buffer
			_, err = buf.ReadFrom(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			data = buf.Bytes()
		}
	}
	if len(data) == 0 {
		return nil, errFontNotFound
	}
	return decodeUTF8Replace(data), nil
}

// parsePyInt mirrors int(s) (base 10) for the header numbers.
func parsePyInt(s string) (int, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), "_", "")
	return strconv.Atoi(s)
}

// parsePyHex mirrors int(s, 16) for a string known to start with 0x / 0X.
func parsePyHex(s string) (int64, error) {
	body := s[2:]
	body = strings.TrimPrefix(body, "_")
	if body == "" || strings.HasSuffix(body, "_") || strings.Contains(body, "__") {
		return 0, fmt.Errorf("invalid literal for int() with base 16: %q", s)
	}
	return strconv.ParseInt(strings.ReplaceAll(body, "_", ""), 16, 64)
}

type lineQueue struct{ lines [][]rune }

func (q *lineQueue) pop() ([]rune, error) {
	if len(q.lines) == 0 {
		return nil, errors.New("pop from empty list")
	}
	l := q.lines[0]
	q.lines = q.lines[1:]
	return l, nil
}

// endMarker mirrors reEndMarker = (.)\s*$ : the last non-whitespace character
// (or the first character if the line is all whitespace).
func endMarker(line []rune) (rune, error) {
	if len(line) == 0 {
		return 0, errors.New("'NoneType' object has no attribute 'group'")
	}
	for i := len(line) - 1; i >= 0; i-- {
		if !isSpace(line[i]) {
			return line[i], nil
		}
	}
	return line[0], nil
}

// cutEnd mirrors re.sub(escape(end) + r'{1,2}\s*$', ”, line).
func cutEnd(line []rune, end rune) []rune {
	allSpace := func(s []rune) bool {
		for _, r := range s {
			if !isSpace(r) {
				return false
			}
		}
		return true
	}
	for p := 0; p < len(line); p++ {
		if line[p] != end {
			continue
		}
		if p+1 < len(line) && line[p+1] == end && allSpace(line[p+2:]) {
			return line[:p]
		}
		if allSpace(line[p+1:]) {
			return line[:p]
		}
	}
	return line
}

func loadFont(name string) (*Font, error) {
	raw, err := preloadFont(name)
	if err != nil {
		return nil, err
	}
	f := &Font{name: name, printDirection: -1, chars: map[int64][][]rune{}, width: map[int64]int{}}
	if err := f.parse(raw); err != nil {
		return nil, fmt.Errorf("problem parsing %s font: %v", name, err)
	}
	return f, nil
}

func (f *Font) parse(raw []rune) error {
	data := make([]rune, len(raw))
	for i, r := range raw {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			r = ' '
		}
		data[i] = r
	}
	q := &lineQueue{lines: splitLines(data)}

	header, err := q.pop()
	if err != nil {
		return err
	}
	// reMagicNumber = ^[tf]lf2.
	if len(header) < 5 || (header[0] != 't' && header[0] != 'f') || header[1] != 'l' || header[2] != 'f' || header[3] != '2' || header[4] == '\n' {
		return fmt.Errorf("%s is not a valid figlet font", f.name)
	}
	parts := fields(header[5:])
	if len(parts) < 6 {
		return fmt.Errorf("malformed header for %s", f.name)
	}
	f.hardBlank = parts[0]
	nums := make([]int, 5)
	for i := 0; i < 5; i++ {
		if nums[i], err = parsePyInt(string(parts[i+1])); err != nil {
			return err
		}
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	fullLayout, haveFull := 0, false
	if len(parts) > 6 {
		if f.printDirection, err = parsePyInt(string(parts[6])); err != nil {
			return err
		}
	}
	if len(parts) > 7 {
		if fullLayout, err = parsePyInt(string(parts[7])); err != nil {
			return err
		}
		haveFull = true
	}
	if !haveFull {
		switch {
		case oldLayout == 0:
			fullLayout = 64
		case oldLayout < 0:
			fullLayout = 0
		default:
			fullLayout = (oldLayout & 31) | 128
		}
	}
	f.height = height
	f.smushMode = fullLayout

	for i := 0; i < commentLines; i++ {
		if _, err := q.pop(); err != nil {
			return err
		}
	}

	readChar := func() (int, [][]rune, error) {
		var end rune
		haveEnd := false
		width := 0
		var rows [][]rune
		for i := 0; i < height; i++ {
			line, err := q.pop()
			if err != nil {
				return 0, nil, err
			}
			if !haveEnd {
				if end, err = endMarker(line); err != nil {
					return 0, nil, err
				}
				haveEnd = true
			}
			line = cutEnd(line, end)
			if len(line) > width {
				width = len(line)
			}
			rows = append(rows, line)
		}
		return width, rows, nil
	}
	nonEmpty := func(rows [][]rune) bool {
		for _, r := range rows {
			if len(r) > 0 {
				return true
			}
		}
		return false
	}

	for i := 32; i < 127; i++ {
		w, rows, err := readChar()
		if err != nil {
			return err
		}
		if i == 32 || nonEmpty(rows) {
			f.chars[int64(i)] = rows
			f.width[int64(i)] = w
		}
	}
	if len(q.lines) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, rows, err := readChar()
			if err != nil {
				return err
			}
			if nonEmpty(rows) {
				f.chars[int64(c)] = rows
				f.width[int64(c)] = w
			}
		}
	}
	for len(q.lines) > 0 {
		line, _ := q.pop()
		line = strip(line)
		code := string(line)
		if k := strings.IndexByte(code, ' '); k >= 0 {
			code = code[:k]
		}
		if code == "" {
			continue
		}
		if len(code) >= 2 && code[0] == '0' && (code[1] == 'x' || code[1] == 'X') {
			n, err := parsePyHex(code)
			if err != nil {
				return err
			}
			w, rows, err := readChar()
			if err != nil {
				return err
			}
			if nonEmpty(rows) {
				f.chars[n] = rows
				f.width[n] = w
			}
		}
	}
	return nil
}
GO_SOURCE_EOF

cat > render.go <<'GO_SOURCE_EOF'
package main

// Port of pyfiglet.FigletBuilder: smushing, kerning, wrapping and justification.
// Kept structurally identical to the Python so every quirk is preserved.

import "errors"

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

// noChar stands for Python's empty string ” as a "character".
const noChar rune = -1

var errCharNotPrinted = errors.New("Width is not enough to print this character")

type blankMarker struct {
	buffer   [][]rune
	iterator int
}

type builder struct {
	text      []rune
	direction string
	width     int
	font      *Font
	justify   string

	iterator          int
	maxSmush          int
	curCharWidth      int
	prevCharWidth     int
	currentTotalWidth int

	blankMarkers []blankMarker
	queue        [][][]rune
	buffer       [][]rune
}

func newBuilder(text []rune, font *Font, direction string, width int, justify string) *builder {
	b := &builder{text: text, direction: direction, width: width, font: font, justify: justify}
	b.buffer = b.emptyBuffer()
	return b
}

func (b *builder) emptyBuffer() [][]rune {
	return make([][]rune, b.font.height)
}

func copyBuffer(buf [][]rune) [][]rune {
	out := make([][]rune, len(buf))
	copy(out, buf)
	return out
}

func (b *builder) charAt(i int) ([][]rune, int, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, 0, false
	}
	c := int64(b.text[i])
	rows, ok := b.font.chars[c]
	if !ok {
		return nil, 0, false
	}
	return rows, b.font.width[c], true
}

func (b *builder) render() ([]rune, error) {
	for b.iterator < len(b.text) {
		if err := b.addCharToProduct(); err != nil {
			return nil, err
		}
		b.iterator++
	}
	if len(b.buffer[0]) != 0 {
		b.queue = append(b.queue, b.buffer)
	}
	var out []rune
	hb := b.font.hardBlank
	for _, buf := range b.queue {
		buf = b.justifyBuffer(buf)
		s := append(joinLines(buf), '\n')
		out = append(out, replaceAll(s, hb, ' ')...)
	}
	return out, nil
}

func replaceAll(s []rune, old []rune, with rune) []rune {
	if len(old) == 0 {
		return s
	}
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && string(s[i:i+len(old)]) == string(old) {
			out = append(out, with)
			i += len(old)
			continue
		}
		out = append(out, s[i])
		i++
	}
	return out
}

func (b *builder) addCharToProduct() error {
	curChar, curWidth, ok := b.charAt(b.iterator)

	if b.text[b.iterator] == '\n' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{copyBuffer(b.buffer), b.iterator})
		b.handleNewLine()
		return nil
	}
	if !ok {
		return nil
	}
	if b.width < curWidth {
		return errCharNotPrinted
	}
	b.curCharWidth = curWidth
	var err error
	if b.maxSmush, err = b.smushAmount(b.buffer, curChar); err != nil {
		return err
	}
	b.currentTotalWidth = len(b.buffer[0]) + b.curCharWidth - b.maxSmush

	if b.text[b.iterator] == ' ' {
		b.blankMarkers = append(b.blankMarkers, blankMarker{copyBuffer(b.buffer), b.iterator})
	}

	if b.currentTotalWidth >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.font.height; row++ {
			if err := b.addCurCharRowToBufferRow(curChar, row); err != nil {
				return err
			}
		}
	}
	b.prevCharWidth = b.curCharWidth
	return nil
}

func (b *builder) smushRow(curChar [][]rune, row int) ([]rune, []rune, error) {
	addLeft := b.buffer[row]
	addRight := curChar[row]
	if b.direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	addLeft = cloneRunes(addLeft)

	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := noChar
		if idx >= 0 && idx < len(addLeft) {
			left = addLeft[idx]
		}
		if i >= len(addRight) {
			return nil, nil, errors.New("string index out of range")
		}
		right := addRight[i]
		smushed, ok := b.smushChars(left, right)
		// updateSmushedCharInLeftBuffer
		if idx < 0 || idx > len(addLeft) {
			continue
		}
		if !ok {
			return nil, nil, errors.New("sequence item: expected str instance, NoneType found")
		}
		if smushed == noChar {
			addLeft = append(addLeft[:idx], addLeft[idx+1:]...)
		} else {
			addLeft[idx] = smushed
		}
	}
	return addLeft, addRight, nil
}

func (b *builder) addCurCharRowToBufferRow(curChar [][]rune, row int) error {
	addLeft, addRight, err := b.smushRow(curChar, row)
	if err != nil {
		return err
	}
	line := cloneRunes(addLeft)
	if b.maxSmush < len(addRight) {
		line = append(line, addRight[b.maxSmush:]...)
	}
	b.buffer[row] = line
	return nil
}

func (b *builder) cutBufferCommon() {
	b.currentTotalWidth = 0
	b.buffer = b.emptyBuffer()
	b.blankMarkers = nil
	b.prevCharWidth = 0
	curChar, _, ok := b.charAt(b.iterator)
	if !ok {
		return
	}
	if m, err := b.smushAmount(b.buffer, curChar); err == nil {
		b.maxSmush = m
	}
}

func (b *builder) handleNewLine() {
	if n := len(b.blankMarkers); n > 0 {
		m := b.blankMarkers[n-1]
		b.blankMarkers = b.blankMarkers[:n-1]
		b.queue = append(b.queue, m.buffer)
		b.iterator = m.iterator
		b.cutBufferCommon()
	} else {
		b.queue = append(b.queue, b.buffer)
		b.iterator--
		b.cutBufferCommon()
	}
}

func (b *builder) justifyBuffer(buf [][]rune) [][]rune {
	pad := func(n int, row []rune) []rune {
		if n <= 0 {
			return row
		}
		out := make([]rune, 0, n+len(row))
		for i := 0; i < n; i++ {
			out = append(out, ' ')
		}
		return append(out, row...)
	}
	switch b.justify {
	case "right":
		for row := 0; row < b.font.height; row++ {
			buf[row] = pad(b.width-len(buf[row])-1, buf[row])
		}
	case "center":
		for row := 0; row < b.font.height; row++ {
			buf[row] = pad((b.width-len(buf[row]))/2, buf[row]) // int() truncates toward zero
		}
	}
	return buf
}

func lenRstripSpace(s []rune) int {
	n := len(s)
	for n > 0 && s[n-1] == ' ' {
		n--
	}
	return n
}

func (b *builder) smushAmount(buffer, curChar [][]rune) (int, error) {
	if b.font.smushMode&(smSmush|smKern) == 0 {
		return 0, nil
	}
	maxSmush := b.curCharWidth
	for row := 0; row < b.font.height; row++ {
		lineLeft := buffer[row]
		if row >= len(curChar) {
			return 0, errors.New("list index out of range")
		}
		lineRight := curChar[row]
		if b.direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		linebd := lenRstripSpace(lineLeft) - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := noChar
		if linebd < len(lineLeft) {
			ch1 = lineLeft[linebd]
		} else {
			linebd = 0
		}

		charbd := 0
		for charbd < len(lineRight) && lineRight[charbd] == ' ' {
			charbd++
		}
		ch2 := noChar
		if charbd < len(lineRight) {
			ch2 = lineRight[charbd]
		} else {
			charbd = len(lineRight)
		}

		amt := charbd + len(lineLeft) - 1 - linebd
		if ch1 == noChar || ch1 == ' ' {
			amt++
		} else if ch2 != noChar {
			if _, ok := b.smushChars(ch1, ch2); ok {
				amt++
			}
		}
		if amt < maxSmush {
			maxSmush = amt
		}
	}
	return maxSmush, nil
}

// in mirrors Python's `c in s` for a one-character (or empty) string c.
func in(c rune, s string) bool {
	if c == noChar {
		return true // '' in s is always True
	}
	for _, r := range s {
		if r == c {
			return true
		}
	}
	return false
}

// smushChars returns the smushed character, noChar for Python's ”, and ok=false for None.
func (b *builder) smushChars(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	if b.prevCharWidth < 2 || b.curCharWidth < 2 {
		return 0, false
	}
	mode := b.font.smushMode
	if mode&smSmush == 0 {
		return 0, false
	}
	hb := b.font.hb()
	if mode&63 == 0 {
		if left == hb {
			return right, true
		}
		if right == hb {
			return left, true
		}
		if b.direction == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if mode&smHardblank != 0 && left == hb && right == hb {
		return left, true
	}
	if left == hb || right == hb {
		return 0, false
	}
	if mode&smEqual != 0 && left == right {
		return left, true
	}

	type pair struct{ a, b string }
	var smushes []pair
	if mode&smLowline != 0 {
		smushes = append(smushes, pair{"_", `|/\[]{}()<>`})
	}
	if mode&smHierarchy != 0 {
		smushes = append(smushes,
			pair{"|", `/\[]{}()<>`},
			pair{`\/`, "[]{}()<>"},
			pair{"[]", "{}()<>"},
			pair{"{}", "()<>"},
			pair{"()", "<>"},
		)
	}
	for _, p := range smushes {
		if in(left, p.a) && in(right, p.b) {
			return right, true
		}
		if in(right, p.a) && in(left, p.b) {
			return left, true
		}
	}

	if mode&smPair != 0 && left != noChar && right != noChar {
		for _, pr := range [][2]rune{{left, right}, {right, left}} {
			s := string(pr[:])
			if s == "[]" || s == "{}" || s == "()" {
				return '|', true
			}
		}
	}

	if mode&smBigX != 0 {
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
GO_SOURCE_EOF

# The fonts stay inside the repository; nothing else from the Python package is needed.
find /workspace/repo -name '*.py' -type f -delete
find /workspace/repo -name '__pycache__' -type d -prune -exec rm -rf {} +

go build ./...
go build -o /tmp/figlet .
/tmp/figlet -f slant "oracle ready"
