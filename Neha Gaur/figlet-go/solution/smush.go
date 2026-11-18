package main

import (
	"strings"
)

const (
	SM_EQUAL     = 1  // smush equal chars (not hardblanks)
	SM_LOWLINE   = 2  // smush _ with any char in hierarchy
	SM_HIERARCHY = 4  // hierarchy: |, /\, [], {}, (), <>
	SM_PAIR      = 8  // hierarchy: [ + ] -> |, { + } -> |, ( + ) -> |
	SM_BIGX      = 16 // / + \ -> |, \ + / -> Y, > + < -> X
	SM_HARDBLANK = 32 // hardblank + hardblank -> hardblank
	SM_KERN      = 64
	SM_SMUSH     = 128
)

func smushChars(font *FigletFont, left, right rune, prevCharWidth, curCharWidth int, direction string) (rune, bool) {
	if left == ' ' {
		return right, true
	}
	if right == ' ' {
		return left, true
	}

	// Disallow overlapping if previous or current char has width < 2
	if prevCharWidth < 2 || curCharWidth < 2 {
		return 0, false
	}

	// Kerning only
	if (font.SmushMode & SM_SMUSH) == 0 {
		return 0, false
	}

	// Smushing by universal overlapping (when SM_SMUSH is set, but all rules 1-6 are 0)
	if (font.SmushMode & 63) == 0 {
		if left == font.HardBlank {
			return right, true
		}
		if right == font.HardBlank {
			return left, true
		}
		if direction == "right-to-left" {
			return left, true
		}
		return right, true
	}

	// Rule 6: Hardblank
	if (font.SmushMode & SM_HARDBLANK) != 0 {
		if left == font.HardBlank && right == font.HardBlank {
			return left, true
		}
	}
	if left == font.HardBlank || right == font.HardBlank {
		return 0, false
	}

	// Rule 1: Equal characters
	if (font.SmushMode & SM_EQUAL) != 0 {
		if left == right {
			return left, true
		}
	}

	// Rule 2: Underscore (lowline)
	if (font.SmushMode & SM_LOWLINE) != 0 {
		if left == '_' && strings.ContainsRune(`|/\[]{}()<>`, right) {
			return right, true
		}
		if right == '_' && strings.ContainsRune(`|/\[]{}()<>`, left) {
			return left, true
		}
	}

	// Rule 3: Hierarchy
	if (font.SmushMode & SM_HIERARCHY) != 0 {
		hierarchyPairs := []struct{ a, b string }{
			{"|", `/\[]{}()<>`},
			{`/\`, `[]{}` + `()` + `<>`},
			{"[]", `{}` + `()` + `<>`},
			{"{}", `()` + `<>`},
			{"()", `<>`},
		}
		for _, pair := range hierarchyPairs {
			if strings.ContainsRune(pair.a, left) && strings.ContainsRune(pair.b, right) {
				return right, true
			}
			if strings.ContainsRune(pair.a, right) && strings.ContainsRune(pair.b, left) {
				return left, true
			}
		}
	}

	// Rule 4: Opposite pair
	if (font.SmushMode & SM_PAIR) != 0 {
		pair1 := string([]rune{left, right})
		pair2 := string([]rune{right, left})
		for _, p := range []string{pair1, pair2} {
			if p == "[]" || p == "{}" || p == "()" {
				return '|', true
			}
		}
	}

	// Rule 5: Big X
	if (font.SmushMode & SM_BIGX) != 0 {
		if left == '/' && right == '\\' {
			return '|', true
		}
		if right == '/' && left == '\\' {
			return 'Y', true
		}
		if left == '>' && right == '<' {
			return 'X', true
		}
	}

	return 0, false
}

func smushAmount(font *FigletFont, buffer []string, curChar []string, prevCharWidth, curCharWidth int, direction string) int {
	if (font.SmushMode & (SM_SMUSH | SM_KERN)) == 0 {
		return 0
	}

	maxSmush := curCharWidth
	for row := 0; row < font.Height; row++ {
		lineLeft := buffer[row]
		lineRight := curChar[row]
		if direction == "right-to-left" {
			lineLeft, lineRight = lineRight, lineLeft
		}

		// Only strip ASCII space to match figlet exactly
		trimmedLeft := strings.TrimRight(lineLeft, " ")
		linebd := len(trimmedLeft) - 1
		if linebd < 0 {
			linebd = 0
		}

		var ch1 rune
		if linebd < len(lineLeft) {
			ch1 = rune(lineLeft[linebd])
		} else {
			linebd = 0
			ch1 = 0
		}

		trimmedRight := strings.TrimLeft(lineRight, " ")
		charbd := len(lineRight) - len(trimmedRight)
		var ch2 rune
		if charbd < len(lineRight) {
			ch2 = rune(lineRight[charbd])
		} else {
			charbd = len(lineRight)
			ch2 = 0
		}

		amt := charbd + len(lineLeft) - 1 - linebd

		if ch1 == 0 || ch1 == ' ' {
			amt++
		} else if ch2 != 0 {
			if _, ok := smushChars(font, ch1, ch2, prevCharWidth, curCharWidth, direction); ok {
				amt++
			}
		}

		if amt < maxSmush {
			maxSmush = amt
		}
	}

	return maxSmush
}

func smushRow(font *FigletFont, bufferRow, curCharRow string, maxSmush, prevCharWidth, curCharWidth int, direction string) string {
	addLeft := bufferRow
	addRight := curCharRow

	if direction == "right-to-left" {
		addLeft, addRight = addRight, addLeft
	}

	leftBytes := []byte(addLeft)
	for i := 0; i < maxSmush; i++ {
		idx := len(leftBytes) - maxSmush + i
		var left rune
		if idx >= 0 && idx < len(leftBytes) {
			left = rune(leftBytes[idx])
		}
		var right rune
		if i < len(addRight) {
			right = rune(addRight[i])
		}

		if smushed, ok := smushChars(font, left, right, prevCharWidth, curCharWidth, direction); ok {
			if idx >= 0 && idx < len(leftBytes) {
				leftBytes[idx] = byte(smushed)
			}
		}
	}

	var rightRemainder string
	if maxSmush < len(addRight) {
		rightRemainder = addRight[maxSmush:]
	}

	return string(leftBytes) + rightRemainder
}
