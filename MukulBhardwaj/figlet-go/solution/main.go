package main

import (
	"embed"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

//go:embed fonts/*
var fontFiles embed.FS

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

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

func allSpace(rs []rune) bool {
	for _, r := range rs {
		if !pyIsSpace(r) {
			return false
		}
	}
	return true
}

// ---------- font loading ----------

type font struct {
	height    int
	hardBlank string
	printDir  int
	smushMode int
	chars     map[int][][]rune
	widths    map[int]int
}

func splitLinesPy(s string) []string {
	var out []string
	var cur []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e:
			out = append(out, string(cur))
			cur = nil
		case '\r':
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, string(cur))
			cur = nil
		default:
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

func atoi(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		fail("bad number in font: " + s)
	}
	return v
}

func endMarker(rs []rune) rune {
	for k := len(rs) - 1; k >= 0; k-- {
		if !pyIsSpace(rs[k]) {
			return rs[k]
		}
	}
	if len(rs) == 0 {
		fail("problem parsing font: empty glyph line")
	}
	return rs[0]
}

// removes the match of  END{1,2}\s*$  (leftmost), like the Python regex
func stripEnd(rs []rune, end rune) []rune {
	for i := 0; i < len(rs); i++ {
		cnt := 0
		for i+cnt < len(rs) && cnt < 2 && rs[i+cnt] == end {
			cnt++
		}
		for e := cnt; e >= 1; e-- {
			if allSpace(rs[i+e:]) {
				return rs[:i]
			}
		}
	}
	return rs
}

func nonEmpty(rows [][]rune) bool {
	for _, r := range rows {
		if len(r) > 0 {
			return true
		}
	}
	return false
}

func loadFont(name string) (*font, bool) {
	var raw []byte
	found := false
	for _, ext := range []string{"tlf", "flf"} {
		b, err := fontFiles.ReadFile("fonts/" + name + "." + ext)
		if err == nil {
			raw = b
			found = true
			break
		}
	}
	if !found || len(raw) == 0 {
		return nil, false
	}
	text := strings.ToValidUTF8(string(raw), "\uFFFD")
	text = strings.Map(func(r rune) rune {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			return ' '
		}
		return r
	}, text)
	data := splitLinesPy(text)
	pos := 0
	pop := func() string {
		if pos >= len(data) {
			fail("problem parsing " + name + " font")
		}
		l := data[pos]
		pos++
		return l
	}

	hr := []rune(pop())
	if len(hr) < 5 || (hr[0] != 't' && hr[0] != 'f') || hr[1] != 'l' || hr[2] != 'f' || hr[3] != '2' {
		fail(name + " is not a valid figlet font")
	}
	fields := strings.Fields(string(hr[5:]))
	if len(fields) < 6 {
		fail("malformed header for " + name)
	}
	f := &font{chars: map[int][][]rune{}, widths: map[int]int{}, printDir: -1}
	f.hardBlank = fields[0]
	height := atoi(fields[1])
	oldLayout := atoi(fields[4])
	commentLines := atoi(fields[5])
	if len(fields) > 6 {
		f.printDir = atoi(fields[6])
	}
	if len(fields) > 7 {
		f.smushMode = atoi(fields[7])
	} else {
		switch {
		case oldLayout == 0:
			f.smushMode = 64
		case oldLayout < 0:
			f.smushMode = 0
		default:
			f.smushMode = (oldLayout & 31) | 128
		}
	}
	f.height = height

	for k := 0; k < commentLines; k++ {
		pop()
	}

	loadChar := func() (int, [][]rune) {
		var end rune
		haveEnd := false
		width := 0
		rows := make([][]rune, 0, height)
		for k := 0; k < height; k++ {
			rs := []rune(pop())
			if !haveEnd {
				end = endMarker(rs)
				haveEnd = true
			}
			rs = stripEnd(rs, end)
			if len(rs) > width {
				width = len(rs)
			}
			rows = append(rows, rs)
		}
		return width, rows
	}

	for i := 32; i < 127; i++ {
		w, letter := loadChar()
		if i == 32 || nonEmpty(letter) {
			f.chars[i] = letter
			f.widths[i] = w
		}
	}
	if pos < len(data) {
		for _, c := range "ÄÖÜäöüß" {
			w, letter := loadChar()
			if nonEmpty(letter) {
				f.chars[int(c)] = letter
				f.widths[int(c)] = w
			}
		}
	}
	for pos < len(data) {
		line := strings.TrimFunc(pop(), pyIsSpace)
		first := line
		if idx := strings.IndexByte(line, ' '); idx >= 0 {
			first = line[:idx]
		}
		if first == "" {
			continue
		}
		if len(first) >= 2 && first[0] == '0' && (first[1] == 'x' || first[1] == 'X') {
			code, err := strconv.ParseInt(first[2:], 16, 64)
			if err != nil {
				fail("problem parsing " + name + " font")
			}
			w, letter := loadChar()
			if nonEmpty(letter) {
				f.chars[int(code)] = letter
				f.widths[int(code)] = w
			}
		}
	}
	return f, true
}

// ---------- rendering engine ----------

type marker struct {
	buf  [][]rune
	iter int
}

type builder struct {
	f        *font
	text     []rune
	dir      string
	width    int
	just     string
	iter     int
	maxSmush int
	curW     int
	prevW    int
	totalW   int
	markers  []marker
	queue    [][][]rune
	buf      [][]rune
}

func newBuf(h int) [][]rune { return make([][]rune, h) }

func copyBuf(b [][]rune) [][]rune {
	c := make([][]rune, len(b))
	copy(c, b)
	return c
}

func (b *builder) charAt(i int) ([][]rune, int, bool) {
	if i < 0 || i >= len(b.text) {
		return nil, 0, false
	}
	c := int(b.text[i])
	g, ok := b.f.chars[c]
	if !ok {
		return nil, 0, false
	}
	return g, b.f.widths[c], true
}

func (b *builder) addChar() {
	cur, w, ok := b.charAt(b.iter)
	if b.text[b.iter] == '\n' {
		b.markers = append(b.markers, marker{copyBuf(b.buf), b.iter})
		b.handleNewLine()
		return
	}
	if !ok {
		return
	}
	if b.width < w {
		fail("width is not enough to print this character")
	}
	b.curW = w
	b.maxSmush = b.smushAmount(b.buf, cur)
	b.totalW = len(b.buf[0]) + b.curW - b.maxSmush
	if b.text[b.iter] == ' ' {
		b.markers = append(b.markers, marker{copyBuf(b.buf), b.iter})
	}
	if b.totalW >= b.width {
		b.handleNewLine()
	} else {
		for row := 0; row < b.f.height; row++ {
			b.addRow(cur, row)
		}
	}
	b.prevW = b.curW
}

func (b *builder) handleNewLine() {
	if len(b.markers) > 0 {
		m := b.markers[len(b.markers)-1]
		b.markers = b.markers[:len(b.markers)-1]
		b.queue = append(b.queue, m.buf)
		b.iter = m.iter
		b.cutCommon()
		return
	}
	if len(b.buf[0]) == 0 {
		fail("character is too wide for the width (original pyfiglet loops forever here)")
	}
	b.queue = append(b.queue, b.buf)
	b.iter--
	b.cutCommon()
}

func (b *builder) cutCommon() {
	b.totalW = 0
	b.buf = newBuf(b.f.height)
	b.markers = nil
	b.prevW = 0
	cur, _, ok := b.charAt(b.iter)
	if !ok {
		return
	}
	b.maxSmush = b.smushAmount(b.buf, cur)
}

func (b *builder) addRow(cur [][]rune, row int) {
	addLeft := b.buf[row]
	addRight := cur[row]
	if b.dir == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}
	for i := 0; i < b.maxSmush; i++ {
		idx := len(addLeft) - b.maxSmush + i
		left := ""
		if idx >= 0 && idx < len(addLeft) {
			left = string(addLeft[idx])
		}
		if i >= len(addRight) {
			fail("index out of range while smushing")
		}
		right := string(addRight[i])
		sm, ok := b.smushChars(left, right)
		if idx < 0 || idx >= len(addLeft) {
			continue
		}
		if !ok {
			fail("cannot smush characters")
		}
		nl := make([]rune, len(addLeft))
		copy(nl, addLeft)
		rs := []rune(sm)
		if len(rs) > 0 {
			nl[idx] = rs[0]
		}
		addLeft = nl
	}
	m := b.maxSmush
	if m > len(addRight) {
		m = len(addRight)
	}
	nb := make([]rune, 0, len(addLeft)+len(addRight))
	nb = append(nb, addLeft...)
	nb = append(nb, addRight[m:]...)
	b.buf[row] = nb
}

func (b *builder) smushAmount(buf, cur [][]rune) int {
	if b.f.smushMode&(smSmush|smKern) == 0 {
		return 0
	}
	maxS := b.curW
	for row := 0; row < b.f.height; row++ {
		left, right := buf[row], cur[row]
		if b.dir == "right-to-left" {
			left, right = right, left
		}
		trimmed := len(left)
		for trimmed > 0 && left[trimmed-1] == ' ' {
			trimmed--
		}
		linebd := trimmed - 1
		if linebd < 0 {
			linebd = 0
		}
		ch1 := ""
		if linebd < len(left) {
			ch1 = string(left[linebd])
		} else {
			linebd = 0
		}
		charbd := 0
		for charbd < len(right) && right[charbd] == ' ' {
			charbd++
		}
		ch2 := ""
		if charbd < len(right) {
			ch2 = string(right[charbd])
		} else {
			charbd = len(right)
		}
		amt := charbd + len(left) - 1 - linebd
		if ch1 == "" || ch1 == " " {
			amt++
		} else if ch2 != "" {
			if _, ok := b.smushChars(ch1, ch2); ok {
				amt++
			}
		}
		if amt < maxS {
			maxS = amt
		}
	}
	return maxS
}

func (b *builder) smushChars(left, right string) (string, bool) {
	if left == " " {
		return right, true
	}
	if right == " " {
		return left, true
	}
	if b.prevW < 2 || b.curW < 2 {
		return "", false
	}
	m := b.f.smushMode
	hb := b.f.hardBlank
	if m&smSmush == 0 {
		return "", false
	}
	if m&63 == 0 {
		if left == hb {
			return right, true
		}
		if right == hb {
			return left, true
		}
		if b.dir == "right-to-left" {
			return left, true
		}
		return right, true
	}
	if m&smHardblank != 0 {
		if left == hb && right == hb {
			return left, true
		}
	}
	if left == hb || right == hb {
		return "", false
	}
	if m&smEqual != 0 {
		if left == right {
			return left, true
		}
	}
	type pair struct{ a, b string }
	var smushes []pair
	if m&smLowline != 0 {
		smushes = append(smushes, pair{"_", "|/\\[]{}()<>"})
	}
	if m&smHierarchy != 0 {
		smushes = append(smushes,
			pair{"|", "/\\[]{}()<>"},
			pair{"\\/", "[]{}()<>"},
			pair{"[]", "{}()<>"},
			pair{"{}", "()<>"},
			pair{"()", "<>"},
		)
	}
	for _, p := range smushes {
		if strings.Contains(p.a, left) && strings.Contains(p.b, right) {
			return right, true
		}
		if strings.Contains(p.a, right) && strings.Contains(p.b, left) {
			return left, true
		}
	}
	if m&smPair != 0 {
		for _, pr := range []string{left + right, right + left} {
			if pr == "[]" || pr == "{}" || pr == "()" {
				return "|", true
			}
		}
	}
	if m&smBigX != 0 {
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

func (b *builder) justify(buf [][]rune) [][]rune {
	out := make([][]rune, len(buf))
	for i, r := range buf {
		n := 0
		switch b.just {
		case "right":
			n = b.width - len(r) - 1
		case "center":
			n = (b.width - len(r)) / 2
		}
		if n > 0 {
			nr := make([]rune, 0, n+len(r))
			for k := 0; k < n; k++ {
				nr = append(nr, ' ')
			}
			r = append(nr, r...)
		}
		out[i] = r
	}
	return out
}

func render(f *font, text []rune, dir string, width int, just string) string {
	b := &builder{f: f, text: text, dir: dir, width: width, just: just}
	b.buf = newBuf(f.height)
	for b.iter < len(b.text) {
		b.addChar()
		b.iter++
	}
	if len(b.buf[0]) != 0 {
		b.queue = append(b.queue, b.buf)
	}
	var sb strings.Builder
	for _, buf := range b.queue {
		buf = b.justify(buf)
		rows := make([]string, len(buf))
		for i, r := range buf {
			rows[i] = string(r)
		}
		s := strings.Join(rows, "\n") + "\n"
		sb.WriteString(strings.ReplaceAll(s, f.hardBlank, " "))
	}
	return sb.String()
}

// ---------- FigletString post-processing ----------

var reverseMap = map[rune]rune{
	'(': ')', ')': '(', '<': '>', '>': '<',
	'[': ']', ']': '[', '{': '}', '}': '{',
	'/': '\\', '\\': '/',
}

var flipMap = map[rune]rune{
	'/': '\\', '\\': '/', '_': '-', '^': 'v', 'v': '^',
	'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M', 'm': 'w', 'w': 'm',
	'b': 'P', 'P': 'b', 'R': 'b',
}

func splitNL(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func newFromList(rows []string) string {
	return strings.Join(rows, "\n") + "\n"
}

func reverseStr(s string) string {
	var out []string
	for _, row := range splitNL(s) {
		rs := []rune(row)
		n := len(rs)
		rev := make([]rune, n)
		for i, r := range rs {
			if t, ok := reverseMap[r]; ok {
				r = t
			}
			rev[n-1-i] = r
		}
		out = append(out, string(rev))
	}
	return newFromList(out)
}

func flipStr(s string) string {
	rows := splitNL(s)
	var out []string
	for i := len(rows) - 1; i >= 0; i-- {
		rs := []rune(rows[i])
		for k, r := range rs {
			if t, ok := flipMap[r]; ok {
				rs[k] = t
			}
		}
		out = append(out, string(rs))
	}
	return newFromList(out)
}

func stripSurrounding(s string) string {
	var out []string
	seen := false
	for _, row := range splitNL(s) {
		if strings.TrimFunc(row, pyIsSpace) != "" || seen {
			seen = true
			out = append(out, row)
		}
	}
	return strings.TrimRightFunc(newFromList(out), pyIsSpace)
}

// ---------- command line ----------

type options struct {
	font      string
	direction string
	justify   string
	width     int
	reverse   bool
	normalize bool
	strip     bool
	flip      bool
	list      bool
}

type optSpec struct {
	short  rune
	long   string
	hasVal bool
}

var specs = []optSpec{
	{'f', "font", true},
	{'D', "direction", true},
	{'j', "justify", true},
	{'w', "width", true},
	{'r', "reverse", false},
	{'n', "normalize-surrounding-newlines", false},
	{'s', "strip-surrounding-newlines", false},
	{'F', "flip", false},
	{'l', "list_fonts", false},
	{'i', "info_font", false},
	{'L', "load", true},
	{'c', "color", true},
}

func usageErr(msg string) {
	fmt.Fprintln(os.Stderr, "Usage: figlet [options] [text..]")
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(2)
}

func choice(val string, allowed ...string) {
	for _, a := range allowed {
		if a == val {
			return
		}
	}
	usageErr("invalid choice: " + val)
}

func parseArgs(argv []string) (options, []string) {
	o := options{font: "standard", direction: "auto", justify: "auto", width: 80}
	var rest []string
	apply := func(long, val string) {
		switch long {
		case "font":
			o.font = val
		case "direction":
			choice(val, "auto", "left-to-right", "right-to-left")
			o.direction = val
		case "justify":
			choice(val, "auto", "left", "center", "right")
			o.justify = val
		case "width":
			w, err := strconv.Atoi(val)
			if err != nil {
				usageErr("option -w: invalid integer value: " + val)
			}
			o.width = w
		case "reverse":
			o.reverse = true
		case "normalize-surrounding-newlines":
			o.normalize = true
		case "strip-surrounding-newlines":
			o.strip = true
		case "flip":
			o.flip = true
		case "list_fonts":
			o.list = true
		}
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			rest = append(rest, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			name, val, hasEq := strings.Cut(a[2:], "=")
			var spec *optSpec
			n := 0
			for k := range specs {
				if specs[k].long == name {
					spec = &specs[k]
					n = 1
					break
				}
				if strings.HasPrefix(specs[k].long, name) {
					spec = &specs[k]
					n++
				}
			}
			if spec == nil || n != 1 {
				usageErr("no such option: " + a)
			}
			if spec.hasVal {
				if !hasEq {
					i++
					if i >= len(argv) {
						usageErr(a + " option requires an argument")
					}
					val = argv[i]
				}
			}
			apply(spec.long, val)
		case len(a) > 1 && a[0] == '-':
			cluster := []rune(a[1:])
			for j := 0; j < len(cluster); j++ {
				var spec *optSpec
				for k := range specs {
					if specs[k].short == cluster[j] {
						spec = &specs[k]
					}
				}
				if spec == nil {
					usageErr("no such option: -" + string(cluster[j]))
				}
				if spec.hasVal {
					val := string(cluster[j+1:])
					if val == "" {
						i++
						if i >= len(argv) {
							usageErr("-" + string(spec.short) + " option requires an argument")
						}
						val = argv[i]
					}
					apply(spec.long, val)
					break
				}
				apply(spec.long, "")
			}
		default:
			rest = append(rest, a)
		}
	}
	return o, rest
}

func main() {
	o, args := parseArgs(os.Args[1:])

	if o.list {
		ents, _ := fontFiles.ReadDir("fonts")
		var names []string
		for _, e := range ents {
			n := e.Name()
			if strings.HasSuffix(n, ".flf") || strings.HasSuffix(n, ".tlf") {
				names = append(names, strings.SplitN(n, ".", 2)[0])
			}
		}
		sort.Strings(names)
		fmt.Println(strings.Join(names, "\n"))
		return
	}
	if len(args) == 0 {
		fmt.Println("Usage: figlet [options] [text..]")
		os.Exit(1)
	}
	text := strings.Join(args, " ")

	f, ok := loadFont(o.font)
	if !ok {
		fmt.Printf("pyfiglet error: requested font '%s' not found.\n", o.font)
		os.Exit(1)
	}

	dir := o.direction
	if dir == "auto" {
		if f.printDir == 1 {
			dir = "right-to-left"
		} else {
			dir = "left-to-right"
		}
	}
	just := o.justify
	if just == "auto" {
		if dir == "left-to-right" {
			just = "left"
		} else {
			just = "right"
		}
	}

	r := render(f, []rune(text), dir, o.width, just)
	if o.reverse {
		r = reverseStr(r)
	}
	if o.flip {
		r = flipStr(r)
	}
	if o.strip {
		r = stripSurrounding(r)
	} else if o.normalize {
		r = "\n" + stripSurrounding(r) + "\n"
	}
	os.Stdout.WriteString(r + "\n")
}