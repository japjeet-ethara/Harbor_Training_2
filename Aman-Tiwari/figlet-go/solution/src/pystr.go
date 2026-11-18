package main

// Helpers that reproduce Python str semantics exactly (isspace, strip, splitlines).

// pyIsSpace matches Python's str.isspace() (and re's \s for str patterns).
func pyIsSpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0d, r >= 0x1c && r <= 0x20:
		return true
	case r == 0x85, r == 0xa0, r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

func pyStrip(s []rune) []rune {
	return pyRstrip(pyLstrip(s))
}

func pyLstrip(s []rune) []rune {
	i := 0
	for i < len(s) && pyIsSpace(s[i]) {
		i++
	}
	return s[i:]
}

func pyRstrip(s []rune) []rune {
	j := len(s)
	for j > 0 && pyIsSpace(s[j-1]) {
		j--
	}
	return s[:j]
}

// rstripSpace / lstripSpace strip only ASCII ' ' (like str.rstrip(' ')).
func rstripSpace(s []rune) []rune {
	j := len(s)
	for j > 0 && s[j-1] == ' ' {
		j--
	}
	return s[:j]
}

func lstripSpace(s []rune) []rune {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return s[i:]
}

// pySplitlines matches Python's str.splitlines() (no keepends).
func pySplitlines(s []rune) [][]rune {
	var out [][]rune
	start := 0
	i := 0
	for i < len(s) {
		r := s[i]
		brk := false
		switch r {
		case '\n', '\r', 0x0b, 0x0c, 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			brk = true
		}
		if !brk {
			i++
			continue
		}
		out = append(out, s[start:i])
		if r == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			i++
		}
		i++
		start = i
	}
	if start < len(s) {
		out = append(out, s[start:])
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
