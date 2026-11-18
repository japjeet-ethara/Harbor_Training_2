package main

import "strings"

type collisionPolicy struct {
	mode      layoutMode
	hardblank rune
	rtl       bool
}

type surfaceRule struct {
	mask  layoutMode
	apply func(rune, rune) (rune, bool)
}

var orderedRules = [...]surfaceRule{
	{equal, func(left, right rune) (rune, bool) { return left, left == right }},
	{lowline, lowlineRule},
	{hierarchy, hierarchyRule},
	{opposite, oppositeRule},
	{bigX, bigXRule},
}

func lowlineRule(left, right rune) (rune, bool) {
	if right == '_' {
		left, right = right, left
	}
	return right, left == '_' && strings.ContainsRune("|/\\[]{}()<>", right)
}

func rank(ch rune) int {
	for index, category := range [...]string{"|", "/\\", "[]", "{}", "()", "<>"} {
		if strings.ContainsRune(category, ch) {
			return index
		}
	}
	return -1
}

func hierarchyRule(left, right rune) (rune, bool) {
	a, b := rank(left), rank(right)
	if a < 0 || b < 0 || a == b {
		return 0, false
	}
	if a > b {
		return left, true
	}
	return right, true
}

func oppositeRule(left, right rune) (rune, bool) {
	switch string([]rune{left, right}) {
	case "[]", "][", "{}", "}{", "()", ")(":
		return '|', true
	default:
		return 0, false
	}
}

func bigXRule(left, right rune) (rune, bool) {
	switch string([]rune{left, right}) {
	case "/\\":
		return '|', true
	case "\\/":
		return 'Y', true
	case "><":
		return 'X', true
	default:
		return 0, false
	}
}

func (p collisionPolicy) merge(left, right rune, previousWidth, width int) (rune, bool) {
	switch {
	case left == ' ':
		return right, true
	case right == ' ':
		return left, true
	case previousWidth < 2 || width < 2 || p.mode&smushing == 0:
		return 0, false
	case p.mode&ruleBits == 0:
		if left == p.hardblank || right != p.hardblank && !p.rtl {
			return right, true
		}
		return left, true
	case left == p.hardblank || right == p.hardblank:
		return p.hardblank, left == right && p.mode&hardblankPair != 0
	}
	for _, rule := range orderedRules {
		if p.mode&rule.mask != 0 {
			if ch, ok := rule.apply(left, right); ok {
				return ch, true
			}
		}
	}
	return 0, false
}

func (p collisionPolicy) oriented(line, glyph string) ([]rune, []rune) {
	if p.rtl {
		return []rune(glyph), []rune(line)
	}
	return []rune(line), []rune(glyph)
}

func (p collisionPolicy) overlap(rows []string, g glyph, previousWidth int) int {
	if p.mode&(smushing|kerning) == 0 {
		return 0
	}
	limit := g.width
	for row, content := range rows {
		left, right := p.oriented(content, g.rows[row])
		last := len(left) - 1
		for last > 0 && left[last] == ' ' {
			last--
		}
		last = max(last, 0)
		first := 0
		for first < len(right) && right[first] == ' ' {
			first++
		}
		amount := first + len(left) - 1 - last
		if len(left) == 0 || left[last] == ' ' {
			amount++
		} else if first < len(right) {
			if _, ok := p.merge(left[last], right[first], previousWidth, g.width); ok {
				amount++
			}
		}
		limit = min(limit, amount)
	}
	return limit
}

func (p collisionPolicy) place(rows []string, g glyph, overlap, previousWidth int) error {
	for row, content := range rows {
		left, right := p.oriented(content, g.rows[row])
		if overlap > len(right) {
			return invalidOverlap
		}
		for offset := 0; offset < overlap; offset++ {
			index := len(left) - overlap + offset
			if index < 0 {
				continue
			}
			ch, ok := p.merge(left[index], right[offset], previousWidth, g.width)
			if !ok {
				return invalidOverlap
			}
			left[index] = ch
		}
		rows[row] = string(left) + string(right[overlap:])
	}
	return nil
}
