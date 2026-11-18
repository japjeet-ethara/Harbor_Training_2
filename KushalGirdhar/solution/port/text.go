package main

import "strings"

func lineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

func splitLines(s string) []string {
	rows := make([]string, 0, strings.Count(s, "\n")+1)
	start := 0
	for position, r := range s {
		if position < start || !lineBreak(r) {
			continue
		}
		rows = append(rows, s[start:position])
		start = position + len(string(r))
		if r == '\r' && start < len(s) && s[start] == '\n' {
			start++
		}
	}
	if start < len(s) {
		rows = append(rows, s[start:])
	}
	return rows
}

func pythonSpace(r rune) bool {
	return (r >= 9 && r <= 13) || (r >= 28 && r <= 32) ||
		r == 0x85 || r == 0xa0 || r == 0x1680 ||
		(r >= 0x2000 && r <= 0x200a) || r == 0x2028 || r == 0x2029 ||
		r == 0x202f || r == 0x205f || r == 0x3000
}

var mirrorCells = map[rune]rune{
	'(': ')', ')': '(', '/': '\\', '\\': '/', '<': '>', '>': '<',
	'[': ']', ']': '[', '{': '}', '}': '{',
}

var flipCells = map[rune]rune{
	'/': '\\', '\\': '/', 'A': 'V', 'V': 'A', 'M': 'W', 'W': 'M',
	'P': 'b', 'R': 'b', 'b': 'P', '^': 'v', 'v': '^', '_': '-',
	'm': 'w', 'w': 'm',
}

func transform(output string, opts options) string {
	if opts.mirror {
		rows := splitLines(output)
		for i, row := range rows {
			cells := []rune(row)
			for j, k := 0, len(cells)-1; j < k; j, k = j+1, k-1 {
				cells[j], cells[k] = cells[k], cells[j]
			}
			rows[i] = translate(string(cells), mirrorCells)
		}
		output = strings.Join(rows, "\n") + "\n"
	}
	if opts.flip {
		rows := splitLines(output)
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
		for i, row := range rows {
			rows[i] = translate(row, flipCells)
		}
		output = strings.Join(rows, "\n") + "\n"
	}
	if opts.strip || opts.norm {
		rows := splitLines(output)
		for len(rows) > 0 && strings.TrimFunc(rows[0], pythonSpace) == "" {
			rows = rows[1:]
		}
		output = strings.TrimRightFunc(strings.Join(rows, "\n"), pythonSpace)
		if opts.norm && !opts.strip {
			output = "\n" + output + "\n"
		}
	}
	return output
}

func translate(row string, cells map[rune]rune) string {
	return strings.Map(func(r rune) rune {
		if replacement, found := cells[r]; found {
			return replacement
		}
		return r
	}, row)
}
