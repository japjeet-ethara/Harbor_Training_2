package main

// Helpers that reproduce the Python string semantics pyfiglet relies on.

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// pyIsSpace mirrors str.isspace() for one character (also used for regex \s).
func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

func isLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pySplitlines mirrors str.splitlines() (without keepends).
func pySplitlines(s string) []string {
	var out []string
	r := []rune(s)
	start := 0
	for i := 0; i < len(r); i++ {
		if !isLineBoundary(r[i]) {
			continue
		}
		out = append(out, string(r[start:i]))
		if r[i] == '\r' && i+1 < len(r) && r[i+1] == '\n' {
			i++
		}
		start = i + 1
	}
	if start < len(r) {
		out = append(out, string(r[start:]))
	}
	return out
}

// pySplitWhitespace mirrors str.split() with no arguments.
func pySplitWhitespace(s string) []string {
	return strings.FieldsFunc(s, pyIsSpace)
}

func pyStrip(s string) string  { return strings.TrimFunc(s, pyIsSpace) }
func pyRstrip(s string) string { return strings.TrimRightFunc(s, pyIsSpace) }

func digitValue(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'z':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'Z':
		return int(r-'A') + 10
	}
	return 99
}

var errInvalidLiteral = errors.New("invalid literal for int()")

// parseDigits parses digits in base with Python's single-underscore rule.
// allowLeadingUnderscore is true right after a base prefix ("0x_1f").
func parseDigits(s string, base int, allowLeadingUnderscore bool) (uint64, bool, error) {
	if s == "" {
		return 0, true, errInvalidLiteral
	}
	var v uint64
	inRange := true
	prevUnderscore := false
	for i, r := range s {
		if r == '_' {
			if prevUnderscore || (i == 0 && !allowLeadingUnderscore) {
				return 0, true, errInvalidLiteral
			}
			prevUnderscore = true
			continue
		}
		d := digitValue(r)
		if d >= base {
			return 0, true, errInvalidLiteral
		}
		prevUnderscore = false
		if v > (1<<62)/uint64(base) {
			inRange = false
		} else {
			v = v*uint64(base) + uint64(d)
		}
	}
	if prevUnderscore {
		return 0, true, errInvalidLiteral
	}
	return v, inRange, nil
}

// pyParseInt mirrors int(s, base) for base 2, 8, 10 or 16.
func pyParseInt(s string, base int) (int, error) {
	s = pyStrip(s)
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	afterPrefix := false
	if len(s) >= 2 && s[0] == '0' {
		p := s[1] | 0x20
		if (base == 16 && p == 'x') || (base == 8 && p == 'o') || (base == 2 && p == 'b') {
			s = s[2:]
			afterPrefix = true
		}
	}
	// Note int("010") is 10: leading zeros are fine without a prefix.
	v, inRange, err := parseDigits(s, base, afterPrefix)
	if err != nil {
		return 0, err
	}
	if !inRange || v > 1<<53 {
		return 0, errors.New("integer too large")
	}
	if neg {
		return -int(v), nil
	}
	return int(v), nil
}

// pyParseHex mirrors int(code, 16) for a token starting with "0x"/"0X".
// inRange is false when the value is too large to ever be a character.
func pyParseHex(code string) (int, bool, error) {
	v, inRange, err := parseDigits(code[2:], 16, true)
	if err != nil {
		return 0, false, err
	}
	if !inRange || v > unicode.MaxRune {
		return 0, false, nil
	}
	return int(v), true, nil
}

// pyDecodeUTF8Replace mirrors bytes.decode('UTF-8', 'replace'): each maximal
// invalid subpart becomes a single U+FFFD.
func pyDecodeUTF8Replace(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	cont := func(i int, lo, hi byte) bool { return i < len(b) && b[i] >= lo && b[i] <= hi }
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			sb.WriteByte(c)
			i++
			continue
		}
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
			sb.WriteRune(utf8.RuneError)
			i++
			continue
		}
		j := i + 1
		ok := true
		for k := 0; k < need; k++ {
			l, h := byte(0x80), byte(0xBF)
			if k == 0 {
				l, h = lo, hi
			}
			if !cont(j, l, h) {
				ok = false
				break
			}
			j++
		}
		if ok {
			r, _ := utf8.DecodeRune(b[i:j])
			sb.WriteRune(r)
		} else {
			sb.WriteRune(utf8.RuneError)
		}
		i = j
	}
	return sb.String()
}
