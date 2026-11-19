package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
)

// The FIGlet fonts live in the repository (fonts/) and are compiled into the
// binary, so the program works from any working directory.
//
//go:embed fonts
var embeddedFonts embed.FS

const embeddedFontDir = "fonts"

// Extra locations searched after the embedded fonts, as pyfiglet does.
var fontDirectories = []string{
	"/usr/local/share/pyfiglet/",
	"/usr/local/share/figlet",
	"/usr/share/figlet",
}

type fontNotFound struct{ name string }

func (e fontNotFound) Error() string { return e.name }

type fontError struct{ msg string }

func (e fontError) Error() string { return e.msg }

// figletFont mirrors pyfiglet.FigletFont.
type figletFont struct {
	name           string
	comment        string
	chars          map[rune][][]rune
	width          map[rune]int
	height         int
	hardBlank      string
	printDirection *int
	smushMode      int
}

func embeddedExists(fn string) bool {
	p := embeddedFontDir + "/" + fn
	if !fs.ValidPath(p) {
		return false
	}
	_, err := fs.Stat(embeddedFonts, p)
	return err == nil
}

// pyJoin behaves like os.path.join(location, name) for the cases used here.
func pyJoin(location, name string) string {
	if strings.HasPrefix(name, "/") {
		return name
	}
	if location == "" || strings.HasSuffix(location, "/") {
		return location + name
	}
	return location + "/" + name
}

func isRegularFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// preloadFont mirrors FigletFont.preloadFont, including its search order:
// .tlf before .flf; the package (embedded) fonts first, then ./ and the font
// directories, where a later match replaces an earlier one.
func preloadFont(name string) (string, error) {
	type source struct {
		embedded bool
		path     string
	}
	var found *source
	for _, ext := range []string{"tlf", "flf"} {
		fn := name + "." + ext
		if embeddedExists(fn) {
			found = &source{embedded: true, path: embeddedFontDir + "/" + fn}
			break
		}
		for _, loc := range append([]string{"./"}, fontDirectories...) {
			full := pyJoin(loc, fn)
			if isRegularFile(full) {
				found = &source{path: full}
				break
			}
		}
	}
	if found == nil {
		return "", fontNotFound{name}
	}
	var raw []byte
	var err error
	if found.embedded {
		raw, err = embeddedFonts.ReadFile(found.path)
	} else {
		raw, err = os.ReadFile(found.path)
	}
	if err != nil {
		return "", err
	}
	if zr, zerr := zip.NewReader(bytes.NewReader(raw), int64(len(raw))); zerr == nil && len(zr.File) > 0 {
		rc, err := zr.File[0].Open()
		if err != nil {
			return "", err
		}
		raw, err = io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", err
		}
	}
	if len(raw) == 0 {
		return "", fontNotFound{name}
	}
	return pyDecodeUTF8Replace(raw), nil
}

func newFigletFont(name string) (*figletFont, error) {
	data, err := preloadFont(name)
	if err != nil {
		return nil, err
	}
	f := &figletFont{
		name:  name,
		chars: map[rune][][]rune{},
		width: map[rune]int{},
	}
	if err := f.loadFont(data); err != nil {
		return nil, fontError{fmt.Sprintf("problem parsing %s font: %v", name, err)}
	}
	return f, nil
}

func hasMagic(line []rune) bool {
	return len(line) >= 5 && (line[0] == 't' || line[0] == 'f') &&
		line[1] == 'l' && line[2] == 'f' && line[3] == '2' && line[4] != '\n'
}

var errPop = errors.New("pop from empty list")

// loadFont mirrors FigletFont.loadFont.
func (f *figletFont) loadFont(text string) error {
	text = strings.Map(func(r rune) rune {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			return ' '
		}
		return r
	}, text)
	lines := pySplitlines(text)
	pop := func() (string, error) {
		if len(lines) == 0 {
			return "", errPop
		}
		l := lines[0]
		lines = lines[1:]
		return l, nil
	}

	headerLine, err := pop()
	if err != nil {
		return err
	}
	hr := []rune(headerLine)
	if !hasMagic(hr) {
		return fmt.Errorf("%s is not a valid figlet font", f.name)
	}
	header := pySplitWhitespace(string(hr[5:]))
	if len(header) < 6 {
		return fmt.Errorf("malformed header for %s", f.name)
	}
	hardBlank := header[0]
	nums := make([]int, 5)
	for i := 0; i < 5; i++ {
		v, err := pyParseInt(header[1+i], 10)
		if err != nil {
			return err
		}
		nums[i] = v
	}
	height, oldLayout, commentLines := nums[0], nums[3], nums[4]
	var printDirection, fullLayout *int
	if len(header) > 6 {
		v, err := pyParseInt(header[6], 10)
		if err != nil {
			return err
		}
		printDirection = &v
	}
	if len(header) > 7 {
		v, err := pyParseInt(header[7], 10)
		if err != nil {
			return err
		}
		fullLayout = &v
	}
	if fullLayout == nil {
		var v int
		switch {
		case oldLayout == 0:
			v = 64
		case oldLayout < 0:
			v = 0
		default:
			v = (oldLayout & 31) | 128
		}
		fullLayout = &v
	}
	f.height = height
	f.hardBlank = hardBlank
	f.printDirection = printDirection
	f.smushMode = *fullLayout

	for i := 0; i < commentLines; i++ {
		l, err := pop()
		if err != nil {
			return err
		}
		f.comment += l
	}

	loadChar := func() (int, [][]rune, error) {
		width := 0
		var rows [][]rune
		var end rune
		haveEnd := false
		for i := 0; i < height; i++ {
			l, err := pop()
			if err != nil {
				return 0, nil, err
			}
			line := []rune(l)
			if !haveEnd {
				e, ok := endMarkerChar(line)
				if !ok {
					return 0, nil, errors.New("'NoneType' object has no attribute 'group'")
				}
				end, haveEnd = e, true
			}
			line = stripEndMarker(line, end)
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
		w, letter, err := loadChar()
		if err != nil {
			return err
		}
		if i == 32 || nonEmpty(letter) {
			f.chars[rune(i)] = letter
			f.width[rune(i)] = w
		}
	}
	if len(lines) > 0 {
		for _, c := range "ÄÖÜäöüß" {
			w, letter, err := loadChar()
			if err != nil {
				return err
			}
			if nonEmpty(letter) {
				f.chars[c] = letter
				f.width[c] = w
			}
		}
	}
	for len(lines) > 0 {
		l, _ := pop()
		l = pyStrip(l)
		code := strings.SplitN(l, " ", 2)[0]
		if code == "" {
			continue
		}
		if len(code) >= 2 && code[0] == '0' && (code[1] == 'x' || code[1] == 'X') {
			v, inRange, err := pyParseHex(code)
			if err != nil {
				return err
			}
			w, letter, err := loadChar()
			if err != nil {
				return err
			}
			if nonEmpty(letter) && inRange {
				f.chars[rune(v)] = letter
				f.width[rune(v)] = w
			}
		}
	}
	return nil
}

// endMarkerChar mirrors re.search(r'(.)\s*$', line).group(1).
func endMarkerChar(line []rune) (rune, bool) {
	if len(line) == 0 {
		return 0, false
	}
	q := len(line)
	for q > 0 && pyIsSpace(line[q-1]) {
		q--
	}
	if q == 0 {
		return line[0], true
	}
	return line[q-1], true
}

// stripEndMarker mirrors re.sub(re.escape(end) + r'{1,2}\s*$', "", line).
func stripEndMarker(line []rune, end rune) []rune {
	n := len(line)
	wsFrom := make([]bool, n+1) // wsFrom[i]: line[i:] is all whitespace
	wsFrom[n] = true
	for i := n - 1; i >= 0; i-- {
		wsFrom[i] = wsFrom[i+1] && pyIsSpace(line[i])
	}
	for p := 0; p < n; p++ {
		if line[p] != end {
			continue
		}
		if p+1 < n && line[p+1] == end && wsFrom[p+2] {
			return line[:p]
		}
		if wsFrom[p+1] {
			return line[:p]
		}
	}
	return line
}

// isValidFont mirrors FigletFont.isValidFont for a file name.
func isValidFontData(raw []byte) bool {
	if zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw))); err == nil && len(zr.File) > 0 {
		rc, err := zr.File[0].Open()
		if err != nil {
			return false
		}
		raw, _ = io.ReadAll(rc)
		rc.Close()
	}
	first := raw
	if i := bytes.IndexByte(raw, '\n'); i >= 0 {
		first = raw[:i+1]
	}
	header := []rune(pyDecodeUTF8Replace(first))
	return hasMagic(header)
}

// listFonts mirrors FigletFont.getFonts.
func listFonts() []string {
	var names []string
	add := func(name string, read func() ([]byte, error)) {
		if !strings.HasSuffix(name, ".flf") && !strings.HasSuffix(name, ".tlf") {
			return
		}
		raw, err := read()
		if err != nil || !isValidFontData(raw) {
			return
		}
		names = append(names, strings.SplitN(name, ".", 3)[0])
	}
	if entries, err := embeddedFonts.ReadDir(embeddedFontDir); err == nil {
		for _, e := range entries {
			if e.Type().IsRegular() {
				p := path.Join(embeddedFontDir, e.Name())
				add(e.Name(), func() ([]byte, error) { return embeddedFonts.ReadFile(p) })
			}
		}
	}
	for _, dir := range fontDirectories {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			full := pyJoin(dir, e.Name())
			if isRegularFile(full) {
				add(e.Name(), func() ([]byte, error) { return os.ReadFile(full) })
			}
		}
	}
	sort.Strings(names)
	return names
}

// infoFont mirrors FigletFont.infoFont(font) (short=False).
func infoFont(name string) (string, error) {
	data, err := preloadFont(name)
	if err != nil {
		return "", err
	}
	starts := []string{"FONT", "COMMENT", "FONTNAME_REGISTRY", "FAMILY_NAME", "FOUNDRY", "WEIGHT_NAME",
		"SETWIDTH_NAME", "SLANT", "ADD_STYLE_NAME", "PIXEL_SIZE", "POINT_SIZE",
		"RESOLUTION_X", "RESOLUTION_Y", "SPACING", "AVERAGE_WIDTH",
		"FONT_DESCENT", "FONT_ASCENT", "CAP_HEIGHT", "X_HEIGHT", "FACE_NAME", "FULL_NAME",
		"COPYRIGHT", "_DEC_", "DEFAULT_CHAR", "NOTICE", "RELATIVE_"}
	lines := pySplitlines(data)
	if len(lines) > 100 {
		lines = lines[:100]
	}
	var infos []string
	for _, l := range lines {
		r := []rune(l)
		if hasMagic(r) {
			continue
		}
		skip := false
		for _, s := range starts {
			if strings.HasPrefix(l, s) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if len(r) > 0 && strings.ContainsRune("@#$", r[len(r)-1]) {
			continue
		}
		infos = append(infos, l)
	}
	return strings.Join(infos, "\n"), nil
}
