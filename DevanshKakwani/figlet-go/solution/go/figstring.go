package main

// Post-processing of a rendered figure: a port of pyfiglet's FigletString.

// Translation tables for str.translate(). Both cover code points 0..255;
// anything above is left unchanged, as Python does when the table lookup
// raises IndexError.
var (
	reverseMap = buildMap(" !\"#$%&')(*+,-.\\" +
		"0123456789:;>=<?" +
		"@ABCDEFGHIJKLMNO" +
		"PQRSTUVWXYZ]/[^_" +
		"`abcdefghijklmno" +
		"pqrstuvwxyz}|{~\x7f")
	flipMap = buildMap(" !\"#$%&'()*+,-.\\" +
		"0123456789:;<=>?" +
		"@VBCDEFGHIJKLWNO" +
		"bQbSTUAMXYZ[/]v-" +
		"`aPcdefghijklwno" +
		"pqrstu^mxyz{|}~\x7f")
)

// buildMap returns a 256-entry table: identity except for 0x20..0x7f,
// which come from printable (96 characters).
func buildMap(printable string) [256]rune {
	if len(printable) != 96 {
		panic("translation map must cover 0x20..0x7f")
	}
	var m [256]rune
	for i := range m {
		m[i] = rune(i)
	}
	for i := 0; i < 96; i++ {
		m[0x20+i] = rune(printable[i])
	}
	return m
}

func translate(row []rune, m *[256]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if r >= 0 && r < 256 {
			r = m[r]
		}
		out[i] = r
	}
	return out
}

// newFromList is FigletString.newFromList: rows joined by "\n" plus a final
// "\n".
func newFromList(rows [][]rune) []rune {
	var out []rune
	for i, r := range rows {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, r...)
	}
	return append(out, '\n')
}

// Reverse mirrors the figure horizontally.
func Reverse(s []rune) []rune {
	var rows [][]rune
	for _, row := range pySplitLines(s) {
		t := translate(row, &reverseMap)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		rows = append(rows, t)
	}
	return newFromList(rows)
}

// Flip turns the figure upside down.
func Flip(s []rune) []rune {
	lines := pySplitLines(s)
	rows := make([][]rune, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		rows = append(rows, translate(lines[i], &flipMap))
	}
	return newFromList(rows)
}

// StripSurroundingNewlines drops blank lines before the first non-blank
// line, keeps everything after it, and finally applies str.rstrip(), which
// also removes trailing whitespace from the last line.
func StripSurroundingNewlines(s []rune) []rune {
	var rows [][]rune
	seen := false
	for _, row := range pySplitLines(s) {
		if len(pyStrip(row)) != 0 || seen {
			seen = true
			rows = append(rows, row)
		}
	}
	return pyRStrip(newFromList(rows))
}

// NormalizeSurroundingNewlines leaves exactly one empty line before and
// after the figure.
func NormalizeSurroundingNewlines(s []rune) []rune {
	out := []rune{'\n'}
	out = append(out, StripSurroundingNewlines(s)...)
	return append(out, '\n')
}
