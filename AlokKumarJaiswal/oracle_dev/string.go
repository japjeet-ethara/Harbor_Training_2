package main

import (
	"strings"
)

const reverseMap = "" +
	"\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
	"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
	" !\"#$%&')(*+,-.\\" +
	"0123456789:;>=<?" +
	"@ABCDEFGHIJKLMNO" +
	"PQRSTUVWXYZ]/[^_" +
	"`abcdefghijklmno" +
	"pqrstuvwxyz}|{~\x7f" +
	"\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f" +
	"\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f" +
	"\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf" +
	"\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf" +
	"\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf" +
	"\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf" +
	"\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef" +
	"\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"

const flipMap = "" +
	"\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
	"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
	" !\"#$%&'()*+,-.\\" +
	"0123456789:;<=>?" +
	"@VBCDEFGHIJKLWNO" +
	"bQbSTUAMXYZ[/]v-" +
	"`aPcdefghijklwno" +
	"pqrstu^mxyz{|}~\x7f" +
	"\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f" +
	"\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f" +
	"\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf" +
	"\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf" +
	"\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf" +
	"\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf" +
	"\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef" +
	"\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"

// translate applies a 256-entry translation map to string s.
func translate(s, transMap string) string {
	runes := []rune(s)
	for i, r := range runes {
		if int(r) < len(transMap) {
			runes[i] = rune(transMap[r])
		}
	}
	return string(runes)
}

// newFromList joins lines with newline and adds a trailing newline.
// Replaces FigletString.newFromList.
func newFromList(list []string) string {
	return strings.Join(list, "\n") + "\n"
}

// reverseString reverses characters in each line after translating with reverseMap.
// Replaces FigletString.reverse.
func reverseString(s string) string {
	lines := splitFontLines(s)
	out := make([]string, len(lines))
	for i, row := range lines {
		trans := translate(row, reverseMap)
		r := []rune(trans)
		for j, k := 0, len(r)-1; j < k; j, k = j+1, k-1 {
			r[j], r[k] = r[k], r[j]
		}
		out[i] = string(r)
	}
	return newFromList(out)
}

// flipString vertically mirrors lines and translates characters with flipMap.
// Replaces FigletString.flip.
func flipString(s string) string {
	lines := splitFontLines(s)
	out := make([]string, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		out[len(lines)-1-i] = translate(lines[i], flipMap)
	}
	return newFromList(out)
}

// stripSurroundingNewlines removes leading whitespace-only rows and trailing whitespace.
// Replaces FigletString.strip_surrounding_newlines.
func stripSurroundingNewlines(s string) string {
	lines := splitFontLines(s)
	var out []string
	charsSeen := false
	for _, row := range lines {
		if strings.TrimSpace(row) != "" || charsSeen {
			charsSeen = true
			out = append(out, row)
		}
	}
	res := newFromList(out)
	return strings.TrimRight(res, " \t\r\n")
}

// normalizeSurroundingNewlines wraps stripped output with one newline before and after.
// Replaces FigletString.normalize_surrounding_newlines.
func normalizeSurroundingNewlines(s string) string {
	return "\n" + stripSurroundingNewlines(s) + "\n"
}
