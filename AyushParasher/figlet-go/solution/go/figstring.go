package main

// Port of pyfiglet.FigletString: reverse (-r), flip (-F), strip (-s), normalize (-n).

// Translation tables for code points 0x20..0x7F, copied from __reverse_map__ and
// __flip_map__. Everything else maps to itself (str.translate leaves code points
// >= 256 untouched, and both maps are the identity on 0x00..0x1F and 0x80..0xFF).
const reverseASCII = ` !"#$%&')(*+,-.\` +
	"0123456789:;>=<?" +
	"@ABCDEFGHIJKLMNO" +
	`PQRSTUVWXYZ]/[^_` +
	"`abcdefghijklmno" +
	"pqrstuvwxyz}|{~\x7f"

const flipASCII = ` !"#$%&'()*+,-.\` +
	"0123456789:;<=>?" +
	"@VBCDEFGHIJKLWNO" +
	`bQbSTUAMXYZ[/]v-` +
	"`aPcdefghijklwno" +
	"pqrstu^mxyz{|}~\x7f"

func translate(row []rune, table string) []rune {
	out := make([]rune, len(row))
	for i, r := range row {
		if r >= 0x20 && r <= 0x7f {
			out[i] = rune(table[r-0x20])
		} else {
			out[i] = r
		}
	}
	return out
}

func newFromList(rows [][]rune) []rune {
	var out []rune
	for i, r := range rows {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, r...)
	}
	return append(out, '\n')
}

func figReverse(s []rune) []rune {
	var out [][]rune
	for _, row := range pySplitlines(s) {
		t := translate(row, reverseASCII)
		for i, j := 0, len(t)-1; i < j; i, j = i+1, j-1 {
			t[i], t[j] = t[j], t[i]
		}
		out = append(out, t)
	}
	return newFromList(out)
}

func figFlip(s []rune) []rune {
	rows := pySplitlines(s)
	var out [][]rune
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, translate(rows[i], flipASCII))
	}
	return newFromList(out)
}

func figStrip(s []rune) []rune {
	var out [][]rune
	seen := false
	for _, row := range pySplitlines(s) {
		if len(pyStrip(row)) != 0 || seen {
			seen = true
			out = append(out, row)
		}
	}
	return pyRstrip(newFromList(out))
}

func figNormalize(s []rune) []rune {
	out := []rune{'\n'}
	out = append(out, figStrip(s)...)
	return append(out, '\n')
}
