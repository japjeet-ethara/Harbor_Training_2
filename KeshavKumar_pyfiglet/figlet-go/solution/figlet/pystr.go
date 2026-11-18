package main

// Helpers that reproduce the exact Python str semantics pyfiglet relies on.
// All text is handled as []rune, because Python indexes strings by code point.

import (
	"unicode"
	"unicode/utf8"
)

// none stands for Python's empty string '' where a single character is expected.
const none rune = -1

// pyIsSpace mirrors str.isspace(): Go's unicode.IsSpace plus \x1c-\x1f.
func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// isLineBreak mirrors the boundaries used by str.splitlines().
func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// splitLines mirrors str.splitlines() (no trailing empty element; \r\n is one break).
func splitLines(s []rune) [][]rune {
	var lines [][]rune
	start := 0
	for i := 0; i < len(s); i++ {
		if !isLineBreak(s[i]) {
			continue
		}
		lines = append(lines, s[start:i])
		if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			i++
		}
		start = i + 1
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// rstripSpace / lstripSpace mirror rstrip(' ') / lstrip(' ') (ASCII space only).
func rstripSpace(s []rune) []rune {
	n := len(s)
	for n > 0 && s[n-1] == ' ' {
		n--
	}
	return s[:n]
}

func lstripSpace(s []rune) []rune {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return s[i:]
}

// rstripWS / stripWS mirror rstrip() / strip() with no argument.
func rstripWS(s []rune) []rune {
	n := len(s)
	for n > 0 && pyIsSpace(s[n-1]) {
		n--
	}
	return s[:n]
}

func stripWS(s []rune) []rune {
	s = rstripWS(s)
	i := 0
	for i < len(s) && pyIsSpace(s[i]) {
		i++
	}
	return s[i:]
}

// splitWS mirrors str.split() with no argument.
func splitWS(s []rune) [][]rune {
	var out [][]rune
	i := 0
	for i < len(s) {
		for i < len(s) && pyIsSpace(s[i]) {
			i++
		}
		if i >= len(s) {
			break
		}
		j := i
		for j < len(s) && !pyIsSpace(s[j]) {
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

func concat(a, b []rune) []rune {
	out := make([]rune, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}

// decodeUTF8Replace mirrors bytes.decode('utf-8', 'replace'): every maximal
// invalid subpart becomes one U+FFFD (Go's own decoder would emit one per byte).
func decodeUTF8Replace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			out = append(out, rune(c))
			i++
			continue
		}
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
		default:
			out = append(out, utf8.RuneError)
			i++
			continue
		}
		j, ok := i+1, true
		for k := 0; k < need; k++ {
			l, h := byte(0x80), byte(0xBF)
			if k == 0 {
				l, h = lo, hi
			}
			if j >= len(b) || b[j] < l || b[j] > h {
				ok = false
				break
			}
			j++
		}
		if !ok {
			out = append(out, utf8.RuneError)
			i = j
			continue
		}
		r, _ := utf8.DecodeRune(b[i:j])
		out = append(out, r)
		i = j
	}
	return out
}
