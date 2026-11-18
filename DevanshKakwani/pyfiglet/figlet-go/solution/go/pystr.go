package main

// Helpers that reproduce the exact Python string semantics pyfiglet relies
// on. All text is handled as []rune because Python measures and indexes
// strings in code points.

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// pyIsSpace reports whether r is whitespace for str.isspace(), str.split(),
// str.strip() and the regex \s class. Python counts the information
// separators U+001C..U+001F as whitespace; Go's unicode.IsSpace does not.
func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// allSpace reports whether every rune of s is Python whitespace.
func allSpace(s []rune) bool {
	for _, r := range s {
		if !pyIsSpace(r) {
			return false
		}
	}
	return true
}

// pyStrip is str.strip() with no arguments.
func pyStrip(s []rune) []rune {
	i, j := 0, len(s)
	for i < j && pyIsSpace(s[i]) {
		i++
	}
	for j > i && pyIsSpace(s[j-1]) {
		j--
	}
	return s[i:j]
}

// pyRStrip is str.rstrip() with no arguments.
func pyRStrip(s []rune) []rune {
	j := len(s)
	for j > 0 && pyIsSpace(s[j-1]) {
		j--
	}
	return s[:j]
}

// pySplitWhitespace is str.split() with no arguments.
func pySplitWhitespace(s []rune) [][]rune {
	var out [][]rune
	i := 0
	for i < len(s) {
		for i < len(s) && pyIsSpace(s[i]) {
			i++
		}
		start := i
		for i < len(s) && !pyIsSpace(s[i]) {
			i++
		}
		if i > start {
			out = append(out, s[start:i])
		}
	}
	return out
}

// isLineBoundary lists the characters str.splitlines() splits on.
func isLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pySplitLines is str.splitlines(): "\r\n" is one boundary and a trailing
// boundary does not produce a final empty line.
func pySplitLines(s []rune) [][]rune {
	var lines [][]rune
	start := 0
	for i := 0; i < len(s); i++ {
		if !isLineBoundary(s[i]) {
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

// decodeUTF8Replace is bytes.decode('utf-8', 'replace') as CPython does it:
// each maximal invalid subpart becomes one U+FFFD. (Go's decoder emits one
// U+FFFD per byte of a truncated sequence, which differs.)
func decodeUTF8Replace(b []byte) []rune {
	out := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			out = append(out, rune(c))
			i++
			continue
		}
		n := 0                           // continuation bytes required
		lo, hi := byte(0x80), byte(0xBF) // allowed range of the first one
		switch {
		case c >= 0xC2 && c <= 0xDF:
			n = 1
		case c == 0xE0:
			n, lo = 2, 0xA0
		case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
			n = 2
		case c == 0xED:
			n, hi = 2, 0x9F
		case c == 0xF0:
			n, lo = 3, 0x90
		case c >= 0xF1 && c <= 0xF3:
			n = 3
		case c == 0xF4:
			n, hi = 3, 0x8F
		default: // 0x80..0xC1, 0xF5..0xFF can never start a sequence
			out = append(out, utf8.RuneError)
			i++
			continue
		}
		j := 1
		for ; j <= n && i+j < len(b); j++ {
			l, h := byte(0x80), byte(0xBF)
			if j == 1 {
				l, h = lo, hi
			}
			if b[i+j] < l || b[i+j] > h {
				break
			}
		}
		if j > n {
			r, _ := utf8.DecodeRune(b[i : i+n+1])
			out = append(out, r)
			i += n + 1
		} else {
			out = append(out, utf8.RuneError) // the valid prefix [i, i+j)
			i += j
		}
	}
	return out
}

// pyInt parses an integer the way Python's int(s) (base 10) or
// int(s, 16) does for ASCII input: surrounding whitespace, an optional sign,
// an optional 0x prefix in base 16, and single underscores between digits.
func pyInt(s []rune, base int) (int64, error) {
	t := pyStrip(s)
	bad := fmt.Errorf("invalid literal for int() with base %d: %q", base, string(s))
	neg := false
	if len(t) > 0 && (t[0] == '+' || t[0] == '-') {
		neg = t[0] == '-'
		t = t[1:]
	}
	if base == 16 && len(t) >= 2 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X') {
		t = t[2:]
		if len(t) > 0 && t[0] == '_' {
			t = t[1:]
		}
	}
	if len(t) == 0 {
		return 0, bad
	}
	var v int64
	prevUnderscore := true // forbids a leading underscore
	for _, r := range t {
		if r == '_' {
			if prevUnderscore {
				return 0, bad
			}
			prevUnderscore = true
			continue
		}
		d := strings.IndexRune("0123456789abcdef", unicode.ToLower(r))
		if d < 0 || d >= base {
			return 0, bad
		}
		prevUnderscore = false
		if v > (1<<62)/int64(base) {
			return 0, fmt.Errorf("integer %q too large", string(s))
		}
		v = v*int64(base) + int64(d)
	}
	if prevUnderscore {
		return 0, bad
	}
	if neg {
		v = -v
	}
	return v, nil
}

// pyRepr is repr() of a str, used in the "font not found" message.
func pyRepr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case unicode.IsPrint(r):
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}
