package main

// Helpers that reproduce Python 3 str behaviour exactly where Go's differs.

import (
	"unicode"
	"unicode/utf8"
)

// noChar stands for Python's empty string '' where the original compares single characters.
const noChar rune = -1

// pySpace matches str.isspace() and the re module's \s for str patterns.
// Go's unicode.IsSpace misses U+001C..U+001F, which Python counts as whitespace.
func pySpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// pyLineBreak matches the line boundaries used by str.splitlines().
func pyLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pySplitlines is str.splitlines() (keepends=False).
func pySplitlines(s []rune) [][]rune {
	var out [][]rune
	start := 0
	for i := 0; i < len(s); i++ {
		if !pyLineBreak(s[i]) {
			continue
		}
		out = append(out, s[start:i])
		if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			i++
		}
		start = i + 1
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func pyStrip(s []rune) []rune  { return pyRstrip(pyLstrip(s)) }
func pyLstrip(s []rune) []rune {
	i := 0
	for i < len(s) && pySpace(s[i]) {
		i++
	}
	return s[i:]
}
func pyRstrip(s []rune) []rune {
	j := len(s)
	for j > 0 && pySpace(s[j-1]) {
		j--
	}
	return s[:j]
}

// allSpace reports whether every rune is whitespace (true for an empty slice).
func allSpace(s []rune) bool {
	for _, r := range s {
		if !pySpace(r) {
			return false
		}
	}
	return true
}

// pyFields is str.split() with no arguments.
func pyFields(s []rune) [][]rune {
	var out [][]rune
	i := 0
	for i < len(s) {
		for i < len(s) && pySpace(s[i]) {
			i++
		}
		j := i
		for j < len(s) && !pySpace(s[j]) {
			j++
		}
		if j > i {
			out = append(out, s[i:j])
		}
		i = j
	}
	return out
}

// pyDecodeReplace is bytes.decode('utf-8', 'replace'): each maximal invalid
// subsequence becomes one U+FFFD (Go's decoder would emit one per byte).
func pyDecodeReplace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r != utf8.RuneError || size > 1 {
			out = append(out, r)
			i += size
			continue
		}
		out = append(out, utf8.RuneError)
		i += invalidPrefixLen(b[i:])
	}
	return out
}

// invalidPrefixLen returns the length of the maximal prefix of a valid UTF-8
// sequence at the start of b (at least 1), following Python's decoder.
func invalidPrefixLen(b []byte) int {
	c := b[0]
	var need int
	lo, hi := byte(0x80), byte(0xBF)
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
		return 1
	}
	n := 1
	for k := 1; k <= need && k < len(b); k++ {
		x := b[k]
		if k == 1 {
			if x < lo || x > hi {
				break
			}
		} else if x < 0x80 || x > 0xBF {
			break
		}
		n++
	}
	return n
}
