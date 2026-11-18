package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Fonts are copied into this directory by solution/solve.sh.
//
//go:embed fonts/*
var fontFiles embed.FS

type Glyph struct {
	rows  []string
	width int
}
type Font struct {
	name      string
	height    int
	hard      rune
	direction int
	layout    int
	glyphs    map[rune]Glyph
}

func trimEndMarker(s string, marker rune) string {
	r := []rune(s)
	for len(r) > 0 && (r[len(r)-1] == ' ' || r[len(r)-1] == '\t' || r[len(r)-1] == '\r') {
		r = r[:len(r)-1]
	}
	for len(r) > 0 && r[len(r)-1] == marker {
		r = r[:len(r)-1]
	}
	return string(r)
}
func parseFont(name string, data []byte) (*Font, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid font %s", name)
	}
	header := strings.TrimSuffix(lines[0], "\r")
	if !(strings.HasPrefix(header, "flf2") || strings.HasPrefix(header, "tlf2")) || len([]rune(header)) < 6 {
		return nil, fmt.Errorf("invalid font header: %s", name)
	}
	hr := []rune(header)
	hard := hr[5]
	fields := strings.Fields(string(hr[6:]))
	if len(fields) < 5 {
		return nil, fmt.Errorf("malformed font header: %s", name)
	}
	ints := make([]int, len(fields))
	for i := range fields {
		n, e := strconv.Atoi(fields[i])
		if e != nil {
			if i < 5 {
				return nil, e
			}
			break
		}
		ints[i] = n
	}
	height, comments := ints[0], ints[4]
	if height < 1 || height > 100 {
		return nil, fmt.Errorf("bad height")
	}
	direction, layout := 0, 0
	if len(ints) > 5 {
		direction = ints[5]
	}
	if len(ints) > 6 {
		layout = ints[6]
	} else {
		old := ints[3]
		if old == 0 {
			layout = 64
		} else if old < 0 {
			layout = 0
		} else {
			layout = (old & 31) | 128
		}
	}
	idx := 1 + comments
	if idx > len(lines) {
		return nil, fmt.Errorf("bad comment count")
	}
	f := &Font{name: name, height: height, hard: hard, direction: direction, layout: layout, glyphs: map[rune]Glyph{}}
	readGlyph := func() (Glyph, error) {
		if idx+height > len(lines)+1 {
			return Glyph{}, fmt.Errorf("truncated glyph")
		}
		rows := make([]string, height)
		width := 0
		marker := rune('@')
		for j := 0; j < height; j++ {
			if idx >= len(lines) {
				return Glyph{}, fmt.Errorf("truncated glyph")
			}
			row := strings.TrimSuffix(lines[idx], "\r")
			idx++
			if j == 0 {
				rr := []rune(row)
				end := len(rr)
				for end > 0 && (rr[end-1] == ' ' || rr[end-1] == '\t' || rr[end-1] == '\r') {
					end--
				}
				if end > 0 {
					marker = rr[end-1]
				}
			}
			row = trimEndMarker(row, marker)
			rows[j] = row
			if n := len([]rune(row)); n > width {
				width = n
			}
		}
		return Glyph{rows, width}, nil
	}
	for c := rune(32); c <= 126; c++ {
		g, e := readGlyph()
		if e != nil {
			return nil, e
		}
		nonempty := false
		for _, r := range g.rows {
			if r != "" {
				nonempty = true
			}
		}
		if c == 32 || nonempty {
			f.glyphs[c] = g
		}
	}
	for _, c := range []rune("ÄÖÜäöüß") {
		if idx >= len(lines) {
			break
		}
		g, e := readGlyph()
		if e != nil {
			break
		}
		nonempty := false
		for _, r := range g.rows {
			if r != "" {
				nonempty = true
			}
		}
		if nonempty {
			f.glyphs[c] = g
		}
	}
	for idx < len(lines) {
		tag := strings.TrimSpace(lines[idx])
		idx++
		if tag == "" {
			continue
		}
		p := strings.Fields(tag)
		if len(p) == 0 {
			continue
		}
		var code int
		var e error
		if strings.HasPrefix(strings.ToLower(p[0]), "0x") {
			parsed, parseErr := strconv.ParseInt(p[0][2:], 16, 32)
			code, e = int(parsed), parseErr
			if e != nil {
				continue
			}
		} else {
			continue
		}
		g, e := readGlyph()
		if e == nil {
			nonempty := false
			for _, r := range g.rows {
				if r != "" {
					nonempty = true
				}
			}
			if nonempty {
				f.glyphs[rune(code)] = g
			}
		}
	}
	return f, nil
}
func loadFont(name string) (*Font, error) {
	for _, ext := range []string{"flf", "tlf"} {
		fn := name + "." + ext
		b, e := fontFiles.ReadFile("fonts/" + fn)
		if e == nil {
			return parseFont(name, b)
		}
	}
	// Fallback for local development when the font directory is adjacent to the executable or cwd.
	for _, dir := range []string{"fonts", filepath.Join(filepath.Dir(os.Args[0]), "fonts"), filepath.Join("pyfiglet", "fonts")} {
		for _, ext := range []string{"flf", "tlf"} {
			b, e := os.ReadFile(filepath.Join(dir, name+"."+ext))
			if e == nil {
				return parseFont(name, b)
			}
		}
	}
	return nil, fmt.Errorf("font not found: %s", name)
}

type renderer struct {
	f                   *Font
	direction, justify  string
	width               int
	prevWidth, curWidth int
}

func (r *renderer) smushChar(a, b rune) (rune, bool) {
	if a == ' ' {
		return b, true
	}
	if b == ' ' {
		return a, true
	}
	if r.prevWidth < 2 || r.curWidth < 2 {
		return 0, false
	}
	mode := r.f.layout
	if mode&128 == 0 {
		return 0, false
	}
	if mode&63 == 0 {
		if a == r.f.hard {
			return b, true
		}
		if b == r.f.hard {
			return a, true
		}
		if r.direction == "right-to-left" {
			return a, true
		}
		return b, true
	}
	if mode&32 != 0 && a == r.f.hard && b == r.f.hard {
		return a, true
	}
	if a == r.f.hard || b == r.f.hard {
		return 0, false
	}
	if mode&1 != 0 && a == b {
		return a, true
	}
	if mode&2 != 0 {
		allowed := "|/\\[]{}()<>"
		if a == '_' && strings.ContainsRune(allowed, b) {
			return b, true
		}
		if b == '_' && strings.ContainsRune(allowed, a) {
			return a, true
		}
	}
	if mode&4 != 0 {
		groups := []string{"|", "/\\", "[]", "{}", "()", "<>"}
		ai, bi := -1, -1
		for i, g := range groups {
			if strings.ContainsRune(g, a) {
				ai = i
			}
			if strings.ContainsRune(g, b) {
				bi = i
			}
		}
		if ai >= 0 && bi >= 0 && ai != bi {
			if ai < bi {
				return b, true
			}
			return a, true
		}
	}
	if mode&8 != 0 {
		p := string([]rune{a, b})
		q := string([]rune{b, a})
		if p == "[]" || p == "{}" || p == "()" || q == "[]" || q == "{}" || q == "()" {
			return '|', true
		}
	}
	if mode&16 != 0 {
		if a == '/' && b == '\\' {
			return '|', true
		}
		if a == '\\' && b == '/' {
			return 'Y', true
		}
		if a == '>' && b == '<' {
			return 'X', true
		}
	}
	return 0, false
}
func rstripSpaces(s string) string { return strings.TrimRight(s, " ") }
func lstripSpaces(s string) string { return strings.TrimLeft(s, " ") }
func (r *renderer) overlap(buf []string, g Glyph) int {
	if r.f.layout&(64|128) == 0 {
		return 0
	}
	max := g.width
	for row := 0; row < r.f.height; row++ {
		left, right := buf[row], g.rows[row]
		if r.direction == "right-to-left" {
			left, right = right, left
		}
		lb := len([]rune(rstripSpaces(left))) - 1
		if lb < 0 {
			lb = 0
		}
		lr := []rune(left)
		var ch1 rune
		if lb < len(lr) {
			ch1 = lr[lb]
		}
		rb := len([]rune(right)) - len([]rune(lstripSpaces(right)))
		rr := []rune(right)
		var ch2 rune
		if rb < len(rr) {
			ch2 = rr[rb]
		} else {
			rb = len(rr)
		}
		amt := rb + len(lr) - 1 - lb
		if ch1 == 0 || ch1 == ' ' {
			amt++
		} else if ch2 != 0 {
			r.curWidth = g.width
			if _, ok := r.smushChar(ch1, ch2); ok {
				amt++
			}
		}
		if amt < max {
			max = amt
		}
	}
	if max < 0 {
		return 0
	}
	return max
}
func (r *renderer) appendGlyph(buf []string, g Glyph, sm int) []string {
	out := make([]string, r.f.height)
	for y := 0; y < r.f.height; y++ {
		left, right := buf[y], g.rows[y]
		if r.direction == "right-to-left" {
			left, right = right, left
		}
		lr, rr := []rune(left), []rune(right)
		start := len(lr) - sm
		for i := 0; i < sm; i++ {
			li := start + i
			var a, b rune
			if li >= 0 && li < len(lr) {
				a = lr[li]
			}
			if i < len(rr) {
				b = rr[i]
			}
			v, ok := r.smushChar(a, b)
			if ok && li >= 0 && li < len(lr) {
				lr[li] = v
			}
		}
		tail := min(sm, len(rr))
		out[y] = string(lr) + string(rr[tail:])
	}
	return out
}
func copyRows(a []string) []string { return append([]string(nil), a...) }
func (r *renderer) render(text string) string {
	buf := make([]string, r.f.height)
	queue := [][]string{}
	blanks := []struct {
		b []string
		i int
	}{}
	r.prevWidth = 0
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '\n' {
			blanks = append(blanks, struct {
				b []string
				i int
			}{copyRows(buf), i})
			queue = append(queue, copyRows(buf))
			buf = make([]string, r.f.height)
			blanks = nil
			r.prevWidth = 0
			continue
		}
		g, ok := r.f.glyphs[c]
		if !ok {
			continue
		}
		if r.width < g.width {
			continue
		}
		r.curWidth = g.width
		sm := r.overlap(buf, g)
		total := len([]rune(buf[0])) + g.width - sm
		if c == ' ' {
			blanks = append(blanks, struct {
				b []string
				i int
			}{copyRows(buf), i})
		}
		if total >= r.width {
			if len(blanks) > 0 {
				last := blanks[len(blanks)-1]
				queue = append(queue, last.b)
				i = last.i
				buf = make([]string, r.f.height)
				blanks = nil
				r.prevWidth = 0
				continue
			}
			queue = append(queue, copyRows(buf))
			i--
			buf = make([]string, r.f.height)
			r.prevWidth = 0
			continue
		}
		buf = r.appendGlyph(buf, g, sm)
		r.prevWidth = r.curWidth
	}
	if buf[0] != "" {
		queue = append(queue, buf)
	}
	var sb strings.Builder
	for _, line := range queue {
		for y, row := range line {
			switch r.justify {
			case "right":
				row = strings.Repeat(" ", max(0, r.width-len([]rune(row))-1)) + row
			case "center":
				row = strings.Repeat(" ", max(0, (r.width-len([]rune(row)))/2)) + row
			}
			line[y] = row
		}
		for _, row := range line {
			sb.WriteString(strings.ReplaceAll(row, string(r.f.hard), " "))
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func translate(s string, table string) string {
	src := []rune(" !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~")
	dst := []rune(table)
	m := map[rune]rune{}
	for i, c := range src {
		if i < len(dst) {
			m[c] = dst[i]
		}
	}
	out := []rune(s)
	for i, c := range out {
		if v, ok := m[c]; ok {
			out[i] = v
		}
	}
	return string(out)
}
func reverseArt(s string) string {
	m := map[rune]rune{'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<', '[': ']', ']': '[', '{': '}', '}': '{'}
	rows := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, row := range rows {
		rr := []rune(row)
		for j, c := range rr {
			if v, ok := m[c]; ok {
				rr[j] = v
			}
		}
		for a, b := 0, len(rr)-1; a < b; a, b = a+1, b-1 {
			rr[a], rr[b] = rr[b], rr[a]
		}
		rows[i] = string(rr)
	}
	return strings.Join(rows, "\n") + "\n"
}
func flipArt(s string) string {
	m := map[rune]rune{'A': 'V', 'M': 'W', 'P': 'b', 'R': 'b', 'V': 'A', 'W': 'M', '[': '[', '\\': '/', '/': '\\', ']': ']', '^': 'v', '_': '-', 'b': 'P', 'm': 'w', 'v': '^', 'w': 'm'}
	rows := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	for i, row := range rows {
		rr := []rune(row)
		for j, c := range rr {
			if v, ok := m[c]; ok {
				rr[j] = v
			}
		}
		rows[i] = string(rr)
	}
	return strings.Join(rows, "\n") + "\n"
}

func stripSurrounding(s string) string {
	s = strings.TrimRight(s, " \t\r\n")
	if s == "" {
		return ""
	}
	rows := strings.Split(s, "\n")
	start, end := 0, len(rows)
	for start < end && strings.TrimSpace(rows[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(rows[end-1]) == "" {
		end--
	}
	return strings.Join(rows[start:end], "\n")
}

func main() {
	font := flag.String("f", "standard", "font")
	width := flag.Int("w", 80, "width")
	justify := flag.String("j", "auto", "justification")
	direction := flag.String("D", "auto", "direction")
	rev := flag.Bool("r", false, "reverse")
	flip := flag.Bool("F", false, "flip")
	strip := flag.Bool("s", false, "strip surrounding newlines")
	norm := flag.Bool("n", false, "normalize surrounding newlines")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: figlet [options] text")
		os.Exit(1)
	}
	f, e := loadFont(*font)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	d := *direction
	if d == "auto" {
		if f.direction == 1 {
			d = "right-to-left"
		} else {
			d = "left-to-right"
		}
	}
	j := *justify
	if j == "auto" {
		if d == "right-to-left" {
			j = "right"
		} else {
			j = "left"
		}
	}
	text := strings.Join(flag.Args(), " ")
	r := (&renderer{f: f, direction: d, justify: j, width: *width}).render(text)
	if *rev {
		r = reverseArt(r)
	}
	if *flip {
		r = flipArt(r)
	}
	if *strip {
		r = stripSurrounding(r)
	} else if *norm {
		r = "\n" + stripSurrounding(r) + "\n"
	}
	fmt.Print(r)
	fmt.Println()
}
