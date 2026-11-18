package main

import (
	"fmt"
	"strings"
)

const (
	equal = 1 << iota
	lowline
	hierarchy
	oppositePair
	bigX
	hardblankRule
	kern
	smush
)

type glyphBuilder struct {
	font         *figFont
	rows         [][]rune
	rtl          bool
	previousSize int
	currentSize  int
}

func (b *glyphBuilder) sides(row int, next glyph) ([]rune, []rune) {
	if b.rtl {
		return next.rows[row], b.rows[row]
	}
	return b.rows[row], next.rows[row]
}

func (b *glyphBuilder) allowedOverlap(next glyph) int {
	if b.font.layout&(kern|smush) == 0 {
		return 0
	}
	best := b.currentSize
	for row := range b.rows {
		left, right := b.sides(row, next)
		leftEdge := len(left) - 1
		for leftEdge > 0 && left[leftEdge] == ' ' {
			leftEdge--
		}
		leftEdge = max(leftEdge, 0)
		rightEdge := 0
		for rightEdge < len(right) && right[rightEdge] == ' ' {
			rightEdge++
		}

		overlap := rightEdge + len(left) - 1 - leftEdge
		if len(left) == 0 || left[leftEdge] == ' ' {
			overlap++
		} else if rightEdge < len(right) {
			if _, compatible := b.collision(left[leftEdge], right[rightEdge]); compatible {
				overlap++
			}
		}
		best = min(best, overlap)
	}
	return best
}

func (b *glyphBuilder) insert(next glyph, overlap int) error {
	for row := range b.rows {
		left, right := b.sides(row, next)
		if overlap > len(right) {
			return fmt.Errorf("overlap exceeds row width")
		}
		merged := make([]rune, len(left), len(left)+len(right)-overlap)
		copy(merged, left)
		for offset := 0; offset < overlap; offset++ {
			index := len(left) - overlap + offset
			if index < 0 {
				continue
			}
			cell, compatible := b.collision(left[index], right[offset])
			if !compatible {
				return fmt.Errorf("incompatible glyph cells %q and %q", left[index], right[offset])
			}
			merged[index] = cell
		}
		b.rows[row] = append(merged, right[overlap:]...)
	}
	return nil
}

func (b *glyphBuilder) collision(left, right rune) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}
	mode, hardblank := b.font.layout, b.font.hardblank
	if b.currentSize < 2 || b.previousSize < 2 || mode&smush == 0 {
		return 0, false
	}
	if mode&63 == 0 {
		switch {
		case left == hardblank:
			return right, true
		case right == hardblank || b.rtl:
			return left, true
		default:
			return right, true
		}
	}
	if mode&hardblankRule != 0 && matchesHardblank(left, right, hardblank) {
		return hardblank, true
	}
	if left == hardblank || right == hardblank {
		return 0, false
	}
	if mode&equal != 0 && matchesEqual(left, right) {
		return left, true
	}
	if mode&lowline != 0 {
		if cell, ok := resolveLowline(left, right); ok {
			return cell, true
		}
	}
	if mode&hierarchy != 0 {
		if cell, ok := resolveHierarchy(left, right); ok {
			return cell, true
		}
	}
	if mode&oppositePair != 0 && matchesOppositePair(left, right) {
		return '|', true
	}
	if mode&bigX != 0 {
		return resolveBigX(left, right)
	}
	return 0, false
}

func matchesHardblank(left, right, hardblank rune) bool {
	return left == hardblank && right == hardblank
}

func matchesEqual(left, right rune) bool {
	return left == right
}

func resolveLowline(left, right rune) (rune, bool) {
	if left == '_' && strings.ContainsRune("|/\\[]{}()<>", right) {
		return right, true
	}
	if right == '_' && strings.ContainsRune("|/\\[]{}()<>", left) {
		return left, true
	}
	return 0, false
}

var hierarchyClasses = map[rune]int{
	'|': 1, '/': 2, '\\': 2, '[': 3, ']': 3, '{': 4, '}': 4,
	'(': 5, ')': 5, '<': 6, '>': 6,
}

func resolveHierarchy(left, right rune) (rune, bool) {
	a, b := hierarchyClasses[left], hierarchyClasses[right]
	switch {
	case a == 0 || b == 0 || a == b:
		return 0, false
	case a > b:
		return left, true
	default:
		return right, true
	}
}

func matchesOppositePair(left, right rune) bool {
	switch [2]rune{left, right} {
	case [2]rune{'[', ']'}, [2]rune{']', '['},
		[2]rune{'{', '}'}, [2]rune{'}', '{'},
		[2]rune{'(', ')'}, [2]rune{')', '('}:
		return true
	}
	return false
}

func resolveBigX(left, right rune) (rune, bool) {
	switch [2]rune{left, right} {
	case [2]rune{'/', '\\'}:
		return '|', true
	case [2]rune{'\\', '/'}:
		return 'Y', true
	case [2]rune{'>', '<'}:
		return 'X', true
	}
	return 0, false
}
