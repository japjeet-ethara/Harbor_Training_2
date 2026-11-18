package main

// Port of pyfiglet.FigletString: reverse, flip, strip/normalize surrounding newlines.

// Differences from identity in FigletString.__reverse_map__.
var reverseMap = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '\\': '/',
	'<': '>', '>': '<', '[': ']', ']': '[', '{': '}', '}': '{',
}

// Differences from identity in FigletString.__flip_map__.
var flipMap = map[rune]rune{
	'/': '\\', '\\': '/', 'A': 'V', 'M': 'W', 'P': 'b', 'R': 'b',
	'V': 'A', 'W': 'M', '^': 'v', '_': '-', 'b': 'P', 'm': 'w',
	'v': '^', 'w': 'm',
}

func translate(row []rune, m map[rune]rune) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if t, ok := m[r]; ok {
			out[i] = t
		} else {
			out[i] = r
		}
	}
	return out
}

func newFromList(rows [][]rune) string {
	return string(joinLines(rows)) + "\n"
}

func reverseString(s string) string {
	var out [][]rune
	for _, row := range pySplitlines([]rune(s)) {
		t := translate(row, reverseMap)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		out = append(out, t)
	}
	return newFromList(out)
}

func flipString(s string) string {
	rows := pySplitlines([]rune(s))
	var out [][]rune
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translate(rows[i], flipMap))
	}
	return newFromList(out)
}

func stripSurroundingNewlines(s string) string {
	var out [][]rune
	seen := false
	for _, row := range pySplitlines([]rune(s)) {
		if len(pyStrip(row)) != 0 || seen {
			seen = true
			out = append(out, row)
		}
	}
	return string(pyRstrip([]rune(newFromList(out))))
}
