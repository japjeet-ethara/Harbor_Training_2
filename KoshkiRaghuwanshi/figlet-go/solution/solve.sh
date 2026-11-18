#!/usr/bin/env bash
set -euo pipefail

cat << 'EOF' > main.go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
)

type FigletFont struct {
    Name           string
    Height         int
    HardBlank      rune
    PrintDirection int
    HasPrintDir    bool
    SmushMode      int
    Chars          map[rune][]string
    Width          map[rune]int
}

var (
    reMagic     = regexp.MustCompile(`^[tf]lf2.`)
    reEndMarker = regexp.MustCompile(`(.)\s*$`)
    reUnicodeNL = regexp.MustCompile("[\u0085\u2028\u2029]")
)

func findFontFile(name string) (string, error) {
    if !strings.HasSuffix(name, ".flf") && !strings.HasSuffix(name, ".tlf") {
        if _, err := os.Stat(filepath.Join("/workspace/repo/pyfiglet/fonts", name+".flf")); err == nil {
            name = name + ".flf"
        } else if _, err := os.Stat(filepath.Join("/workspace/repo/pyfiglet/fonts", name+".tlf")); err == nil {
            name = name + ".tlf"
        }
    }
    candidates := []string{
        name,
        filepath.Join("/workspace/repo/pyfiglet/fonts", name),
        filepath.Join("/usr/share/figlet", name),
    }
    for _, p := range candidates {
        if st, err := os.Stat(p); err == nil && !st.IsDir() {
            return p, nil
        }
    }
    return "", fmt.Errorf("font not found: %s", name)
}

func splitLinesPy(s string) []string {
    s = reUnicodeNL.ReplaceAllString(s, " ")
    s = strings.ReplaceAll(s, "\r\n", "\n")
    s = strings.ReplaceAll(s, "\r", "\n")
    if strings.HasSuffix(s, "\n") {
        s = s[:len(s)-1]
    }
    if s == "" {
        return nil
    }
    return strings.Split(s, "\n")
}

func parseFont(name string) (*FigletFont, error) {
    path, err := findFontFile(name)
    if err != nil {
        return nil, err
    }
    rawBytes, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    lines := splitLinesPy(string(rawBytes))
    if len(lines) == 0 {
        return nil, fmt.Errorf("empty font")
    }
    headerLine := lines[0]
    lines = lines[1:]
    if !reMagic.MatchString(headerLine) {
        return nil, fmt.Errorf("%s is not a valid figlet font", name)
    }
    headerRest := reMagic.ReplaceAllString(headerLine, "")
    hdr := strings.Fields(headerRest)
    if len(hdr) < 6 {
        return nil, fmt.Errorf("malformed header for %s", name)
    }
    hbRunes := []rune(hdr[0])
    hardBlank := hbRunes[0]
    height, _ := strconv.Atoi(hdr[1])
    oldLayout, _ := strconv.Atoi(hdr[4])
    commentLines, _ := strconv.Atoi(hdr[5])

    hasPrintDir := false
    printDir := 0
    if len(hdr) > 6 {
        hasPrintDir = true
        printDir, _ = strconv.Atoi(hdr[6])
    }
    fullLayout := 0
    hasFullLayout := false
    if len(hdr) > 7 {
        hasFullLayout = true
        fullLayout, _ = strconv.Atoi(hdr[7])
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

    idx := 0
    readChar := func() (int, []string, bool) {
        if idx+height > len(lines) {
            return 0, nil, false
        }
        var endRe *regexp.Regexp
        width := 0
        glyph := make([]string, height)
        for r := 0; r < height; r++ {
            line := lines[idx]
            idx++
            if endRe == nil {
                m := reEndMarker.FindStringSubmatch(line)
                if len(m) > 1 {
                    endRe = regexp.MustCompile(regexp.QuoteMeta(m[1]) + `{1,2}\s*$`)
                }
            }
            if endRe != nil {
                line = endRe.ReplaceAllString(line, "")
            }
            w := len([]rune(line))
            if w > width {
                width = w
            }
            glyph[r] = line
        }
        return width, glyph, true
    }

    font := &FigletFont{
        Name:           name,
        Height:         height,
        HardBlank:      hardBlank,
        PrintDirection: printDir,
        HasPrintDir:    hasPrintDir,
        SmushMode:      fullLayout,
        Chars:          make(map[rune][]string),
        Width:          make(map[rune]int),
    }

    for code := rune(32); code < 127; code++ {
        w, glyph, ok := readChar()
        if !ok {
            break
        }
        if code == 32 || strings.Join(glyph, "") != "" {
            font.Chars[code] = glyph
            font.Width[code] = w
        }
    }

    for _, ch := range []rune("\u00c4\u00d6\u00dc\u00e4\u00f6\u00fc\u00df") {
        w, glyph, ok := readChar()
        if !ok {
            break
        }
        if strings.Join(glyph, "") != "" {
            font.Chars[ch] = glyph
            font.Width[ch] = w
        }
    }
for idx < len(lines) {
tagLine := strings.TrimSpace(lines[idx])
idx++
if tagLine == "" {
continue
}
tok := strings.SplitN(tagLine, " ", 2)[0]
if strings.HasPrefix(strings.ToLower(tok), "0x") {
codeVal, err := strconv.ParseInt(tok[2:], 16, 32)
if err != nil {
continue
}
w, glyph, ok := readChar()
if !ok {
break
}
if strings.Join(glyph, "") != "" {
font.Chars[rune(codeVal)] = glyph
font.Width[rune(codeVal)] = w
}
}
}

return font, nil
}

const (
    smEqual     = 1
    smLowline   = 2
    smHierarchy = 4
    smPair      = 8
    smBigX      = 16
    smHardBlank = 32
    smKern      = 64
    smSmush     = 128
)

type blankMarker struct {
    buffer   []string
    iterator int
}

type FigletBuilder struct {
    Text              []rune
    Font              *FigletFont
    Direction         string
    Width             int
    Justify           string
    Iterator          int
    MaxSmush          int
    CurCharWidth      int
    PrevCharWidth     int
    CurrentTotalWidth int
    BlankMarkers      []blankMarker
    Queue             [][]string
    Buffer            []string
}

func newBuilder(font *FigletFont, direction, justify string, width int) *FigletBuilder {
    dir := direction
    if dir == "auto" {
        if font.HasPrintDir && font.PrintDirection == 1 {
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
    return &FigletBuilder{
        Font:      font,
        Direction: dir,
        Width:     width,
        Justify:   just,
        Buffer:    make([]string, font.Height),
    }
}

func (b *FigletBuilder) getCharAt(i int) ([]string, bool) {
    if i < 0 || i >= len(b.Text) {
        return nil, false
    }
    g, ok := b.Font.Chars[b.Text[i]]
    return g, ok
}

func (b *FigletBuilder) getCharWidthAt(i int) (int, bool) {
    if i < 0 || i >= len(b.Text) {
        return 0, false
    }
    w, ok := b.Font.Width[b.Text[i]]
    return w, ok
}

func (b *FigletBuilder) smushChars(left, right rune) (rune, bool) {
    if left == ' ' {
        return right, true
    }
    if right == ' ' {
        return left, true
    }
    if b.PrevCharWidth < 2 || b.CurCharWidth < 2 {
        return 0, false
    }
    if (b.Font.SmushMode & smSmush) == 0 {
        return 0, false
    }
    if (b.Font.SmushMode & 63) == 0 {
        if left == b.Font.HardBlank {
            return right, true
        }
        if right == b.Font.HardBlank {
            return left, true
        }
        if b.Direction == "right-to-left" {
            return left, true
        }
        return right, true
    }
    if (b.Font.SmushMode & smHardBlank) != 0 {
        if left == b.Font.HardBlank && right == b.Font.HardBlank {
            return left, true
        }
    }
    if left == b.Font.HardBlank || right == b.Font.HardBlank {
        return 0, false
    }
    if (b.Font.SmushMode & smEqual) != 0 {
        if left == right {
            return left, true
        }
    }
    type rule struct{ a, b string }
    var rules []rule
    if (b.Font.SmushMode & smLowline) != 0 {
        rules = append(rules, rule{"_", `|/\[]{}()<>`})
    }
    if (b.Font.SmushMode & smHierarchy) != 0 {
        rules = append(rules,
            rule{"|", `/\[]{}()<>`},
            rule{`\/`, `[]{}()<>`},
            rule{"[]", `{}()<>`},
            rule{"{}", `()<>`},
            rule{"()", `<>`},
        )
    }
    for _, r := range rules {
        if strings.ContainsRune(r.a, left) && strings.ContainsRune(r.b, right) {
            return right, true
        }
        if strings.ContainsRune(r.a, right) && strings.ContainsRune(r.b, left) {
            return left, true
        }
    }
    if (b.Font.SmushMode & smPair) != 0 {
        pair1 := string([]rune{left, right})
        pair2 := string([]rune{right, left})
        if pair1 == "[]" || pair1 == "{}" || pair1 == "()" || pair2 == "[]" || pair2 == "{}" || pair2 == "()" {
            return '|', true
        }
    }
    if (b.Font.SmushMode & smBigX) != 0 {
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

func (b *FigletBuilder) smushAmount(buffer, curChar []string) int {
    if (b.Font.SmushMode & (smSmush | smKern)) == 0 {
        return 0
    }
    maxSmush := b.CurCharWidth
    for row := 0; row < b.Font.Height; row++ {
        lineLeft := []rune(buffer[row])
        lineRight := []rune(curChar[row])
        if b.Direction == "right-to-left" {
            lineLeft, lineRight = lineRight, lineLeft
        }
        trimmedLeft := strings.TrimRight(string(lineLeft), " ")
        linebd := len([]rune(trimmedLeft)) - 1
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

        trimmedRight := strings.TrimLeft(string(lineRight), " ")
        charbd := len(lineRight) - len([]rune(trimmedRight))
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

func (b *FigletBuilder) smushRow(curChar []string, row int) ([]rune, []rune) {
    addLeft := []rune(b.Buffer[row])
    addRight := []rune(curChar[row])
    if b.Direction == "right-to-left" {
        addLeft, addRight = addRight, addLeft
    }
    for i := 0; i < b.MaxSmush; i++ {
        idx := len(addLeft) - b.MaxSmush + i
        var left rune
        hasLeft := false
        if idx >= 0 && idx < len(addLeft) {
            left = addLeft[idx]
            hasLeft = true
        }
        var right rune
        if i < len(addRight) {
            right = addRight[i]
        }
        if hasLeft {
            if smushed, ok := b.smushChars(left, right); ok && idx >= 0 && idx < len(addLeft) {
                addLeft[idx] = smushed
            }
        }
    }
    return addLeft, addRight
}

func (b *FigletBuilder) addCurCharRowToBufferRow(curChar []string, row int) {
    addLeft, addRight := b.smushRow(curChar, row)
    tail := []rune{}
    if b.MaxSmush < len(addRight) {
        tail = addRight[b.MaxSmush:]
    }
    b.Buffer[row] = string(append(addLeft, tail...))
}

func cloneBuf(buf []string) []string {
    cp := make([]string, len(buf))
    copy(cp, buf)
    return cp
}

func (b *FigletBuilder) cutBufferCommon() {
    b.CurrentTotalWidth = 0
    b.Buffer = make([]string, b.Font.Height)
    b.BlankMarkers = nil
    b.PrevCharWidth = 0
    curChar, ok := b.getCharAt(b.Iterator)
    if !ok {
        return
    }
    b.MaxSmush = b.smushAmount(b.Buffer, curChar)
}

func (b *FigletBuilder) handleNewLine() {
    if len(b.BlankMarkers) > 0 {
        top := b.BlankMarkers[len(b.BlankMarkers)-1]
        b.BlankMarkers = b.BlankMarkers[:len(b.BlankMarkers)-1]
        b.Queue = append(b.Queue, top.buffer)
        b.Iterator = top.iterator
        b.cutBufferCommon()
    } else {
        b.Queue = append(b.Queue, b.Buffer)
        b.Iterator--
        b.cutBufferCommon()
    }
}

func (b *FigletBuilder) addCharToProduct() {
    if b.Text[b.Iterator] == '\n' {
        b.BlankMarkers = append(b.BlankMarkers, blankMarker{cloneBuf(b.Buffer), b.Iterator})
        b.handleNewLine()
        return
    }
    curChar, ok := b.getCharAt(b.Iterator)
    if !ok {
        return
    }
    curWidth, _ := b.getCharWidthAt(b.Iterator)
    if b.Width < curWidth {
        fmt.Fprintln(os.Stderr, "Width is not enough to print this character")
        os.Exit(1)
    }
    b.CurCharWidth = curWidth
    b.MaxSmush = b.smushAmount(b.Buffer, curChar)
    b.CurrentTotalWidth = len([]rune(b.Buffer[0])) + b.CurCharWidth - b.MaxSmush

    if b.Text[b.Iterator] == ' ' {
        b.BlankMarkers = append(b.BlankMarkers, blankMarker{cloneBuf(b.Buffer), b.Iterator})
    }
    if b.CurrentTotalWidth >= b.Width {
        b.handleNewLine()
    } else {
        for row := 0; row < b.Font.Height; row++ {
            b.addCurCharRowToBufferRow(curChar, row)
        }
    }
    b.PrevCharWidth = b.CurCharWidth
}

func (b *FigletBuilder) justifyString(buffer []string) []string {
    out := cloneBuf(buffer)
    if b.Justify == "right" {
        for row := 0; row < b.Font.Height; row++ {
            pad := b.Width - len([]rune(out[row])) - 1
            if pad > 0 {
                out[row] = strings.Repeat(" ", pad) + out[row]
            }
        }
    } else if b.Justify == "center" {
        for row := 0; row < b.Font.Height; row++ {
            pad := (b.Width - len([]rune(out[row]))) / 2
            if pad > 0 {
                out[row] = strings.Repeat(" ", pad) + out[row]
            }
        }
    }
    return out
}

func (b *FigletBuilder) render(text string) string {
    b.Text = []rune(text)
    for b.Iterator < len(b.Text) {
        b.addCharToProduct()
        b.Iterator++
    }
    if len(b.Buffer) > 0 && b.Buffer[0] != "" {
        b.Queue = append(b.Queue, b.Buffer)
    }
    var sb strings.Builder
    hb := string(b.Font.HardBlank)
    for _, buf := range b.Queue {
        jbuf := b.justifyString(buf)
        s := strings.Join(jbuf, "\n") + "\n"
        s = strings.ReplaceAll(s, hb, " ")
        sb.WriteString(s)
    }
    return sb.String()
}

func reverseMapRune(r rune) rune {
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

func flipMapRune(r rune) rune {
    switch r {
    case '/':
        return '\\'
    case '\\':
        return '/'
    case 'A':
        return 'V'
    case 'M':
        return 'W'
    case 'P':
        return 'b'
    case 'Q':
        return 'd'
    case 'V':
        return 'A'
    case 'W':
        return 'M'
    case '^':
        return 'v'
    case '_':
        return '-'
    case 'b':
        return 'P'
    case 'd':
        return 'Q'
    case 'v':
        return '^'
    default:
        return r
    }
}

func splitFigletLines(s string) []string {
    if strings.HasSuffix(s, "\n") {
        s = s[:len(s)-1]
    }
    if s == "" {
        return []string{}
    }
    return strings.Split(s, "\n")
}

func reverseStr(s string) string {
    lines := splitFigletLines(s)
    for i, l := range lines {
        runes := []rune(l)
        for j, k := 0, len(runes)-1; j <= k; j, k = j+1, k-1 {
            rj := reverseMapRune(runes[j])
            rk := reverseMapRune(runes[k])
            runes[j], runes[k] = rk, rj
        }
        lines[i] = string(runes)
    }
    return strings.Join(lines, "\n") + "\n"
}

func flipStr(s string) string {
    lines := splitFigletLines(s)
    for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
        lines[i], lines[j] = lines[j], lines[i]
    }
    for i, l := range lines {
        runes := []rune(l)
        for j, r := range runes {
            runes[j] = flipMapRune(r)
        }
        lines[i] = string(runes)
    }
    return strings.Join(lines, "\n") + "\n"
}

func stripSurroundingNewlines(s string) string {
    lines := splitFigletLines(s)
    var out []string
    charsSeen := false
    for _, row := range lines {
        if strings.TrimSpace(row) != "" || charsSeen {
            charsSeen = true
            out = append(out, row)
        }
    }
    res := strings.Join(out, "\n") + "\n"
    return strings.TrimRight(res, " \t\n\r\x0b\x0c")
}

func normalizeSurroundingNewlines(s string) string {
    return "\n" + stripSurroundingNewlines(s) + "\n"
}

func main() {
    fontName := "standard"
    direction := "auto"
    justify := "auto"
    width := 80
    reverse := false
    flip := false
    stripNL := false
    normNL := false
    var positional []string

    args := os.Args[1:]
    for i := 0; i < len(args); i++ {
        a := args[i]
        if a == "--" {
            positional = append(positional, args[i+1:]...)
            break
        }
        if strings.HasPrefix(a, "-") && len(a) > 1 && !strings.HasPrefix(a, "--") {
            flag := a[:2]
            rest := a[2:]
            switch flag {
            case "-f":
                if rest != "" {
                    fontName = rest
                } else if i+1 < len(args) {
                    i++
                    fontName = args[i]
                }
            case "-D":
                if rest != "" {
                    direction = rest
                } else if i+1 < len(args) {
                    i++
                    direction = args[i]
                }
            case "-j":
                if rest != "" {
                    justify = rest
                } else if i+1 < len(args) {
                    i++
                    justify = args[i]
                }
            case "-w":
                val := rest
                if val == "" && i+1 < len(args) {
                    i++
                    val = args[i]
                }
                if w, err := strconv.Atoi(val); err == nil {
                    width = w
                }
            case "-r":
                reverse = true
            case "-F":
                flip = true
            case "-s":
                stripNL = true
            case "-n":
                normNL = true
            }
            continue
        }
        if strings.HasPrefix(a, "--") {
            key := a
            val := ""
            hasVal := false
            if eq := strings.IndexByte(a, '='); eq != -1 {
                key = a[:eq]
                val = a[eq+1:]
                hasVal = true
            }
            switch key {
            case "--font":
                if !hasVal && i+1 < len(args) {
                    i++
                    val = args[i]
                }
                fontName = val
            case "--direction":
                if !hasVal && i+1 < len(args) {
                    i++
                    val = args[i]
                }
                direction = val
            case "--justify":
                if !hasVal && i+1 < len(args) {
                    i++
                    val = args[i]
                }
                justify = val
            case "--width":
                if !hasVal && i+1 < len(args) {
                    i++
                    val = args[i]
                }
                if w, err := strconv.Atoi(val); err == nil {
                    width = w
                }
            case "--reverse":
                reverse = true
            case "--flip":
                flip = true
            case "--strip-surrounding-newlines":
                stripNL = true
            case "--normalize-surrounding-newlines":
                normNL = true
            }
            continue
        }
        positional = append(positional, a)
    }

    font, err := parseFont(fontName)
    if err != nil {
        fmt.Fprintf(os.Stderr, "font error: %v\n", err)
        os.Exit(1)
    }

    text := strings.Join(positional, " ")
    b := newBuilder(font, direction, justify, width)
    res := b.render(text)
    if reverse {
        res = reverseStr(res)
    }
    if flip {
        res = flipStr(res)
    }
    if stripNL {
        res = stripSurroundingNewlines(res)
    } else if normNL {
        res = normalizeSurroundingNewlines(res)
    }
    fmt.Print(res + "\n")
}
EOF

find /workspace/repo -name "*.py" -delete
go mod init figlet 2>/dev/null || true
go build -o /tmp/figlet .