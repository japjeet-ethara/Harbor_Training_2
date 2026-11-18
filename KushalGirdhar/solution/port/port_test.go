package main

import (
	"os"
	"strings"
	"testing"
)

func TestSmushingRules(t *testing.T) {
	cases := []struct {
		name        string
		mode        int
		left, right rune
		want        rune
		rtl         bool
	}{
		{"equal", smush | equal, '|', '|', '|', false},
		{"lowline", smush | lowline, '_', '/', '/', false},
		{"hierarchy", smush | hierarchy, '|', ']', ']', false},
		{"pair", smush | oppositePair, '}', '{', '|', false},
		{"slash backslash", smush | bigX, '/', '\\', '|', false},
		{"backslash slash", smush | bigX, '\\', '/', 'Y', false},
		{"angles", smush | bigX, '>', '<', 'X', false},
		{"opposite angles", smush | bigX, '<', '>', 0, false},
		{"hardblank", smush | hardblankRule, '$', '$', '$', false},
		{"hardblank excluded", smush | equal, '$', '$', 0, false},
		{"kerning spaces", kern, ' ', '|', '|', false},
		{"kerning collision", kern, '|', '|', 0, false},
		{"universal", smush, 'X', 'Y', 'Y', false},
		{"universal RTL", smush, 'X', 'Y', 'X', true},
		{"universal hardblank", smush, '$', 'Y', 'Y', true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			builder := glyphBuilder{font: &figFont{layout: test.mode, hardblank: '$'},
				currentSize: 3, previousSize: 3, rtl: test.rtl}
			got, ok := builder.collision(test.left, test.right)
			if got != test.want || ok != (test.want != 0) {
				t.Fatalf("collision = (%q, %t), want %q", got, ok, test.want)
			}
			builder.previousSize = 1
			if test.left != ' ' && test.right != ' ' {
				if _, ok := builder.collision(test.left, test.right); ok {
					t.Fatal("one-column glyph collided")
				}
			}
		})
	}
}

func TestFontParser(t *testing.T) {
	blank := []string{"tlf2a$ 2 1 7 0 2 1 160 1", "comment @", "comment ##"}
	for cell := ' '; cell <= '~'; cell++ {
		if cell == 'A' {
			blank = append(blank, "é[]@", "$ @@ \t")
		} else {
			blank = append(blank, "@", "@@")
		}
	}
	for range "ÄÖÜäöüß" {
		blank = append(blank, " :#", " ###")
	}
	blank = append(blank, "0x00F1 Latin", " ñ%", "  %%", "300 decimal", "Q@", "Q@@")
	font, err := parseFontData([]byte(strings.Join(blank, "\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	if font.height != 2 || font.direction != 1 || font.layout != 160 || font.hardblank != '$' {
		t.Fatalf("wrong header values: %+v", font)
	}
	if got := font.glyphs['A']; got.width != 3 || string(got.rows[0]) != "é[]" || string(got.rows[1]) != "$ " {
		t.Fatalf("wrong rune widths or endmarks: %+v", got)
	}
	if got := font.glyphs['ñ']; got.width != 2 || string(got.rows[0]) != " ñ" {
		t.Fatalf("wrong tagged glyph: %+v", got)
	}
	if _, found := font.glyphs[' ']; !found {
		t.Fatal("empty space must be present")
	}
	if _, found := font.glyphs[300]; found {
		t.Fatal("decimal tag should be skipped at this commit")
	}
	if _, found := font.glyphs['B']; found {
		t.Fatal("empty letter should be missing")
	}
}

func TestCLI(t *testing.T) {
	opts, input, err := parseArgs([]string{"sync", "-rFns", "-w20", "-f", "mini", "--width=40",
		"-jcenter", "-D", "right-to-left", "--", "-flag", "edge"})
	if err != nil {
		t.Fatal(err)
	}
	want := options{font: "mini", width: 40, justify: "center", direction: "right-to-left",
		mirror: true, flip: true, strip: true, norm: true}
	if opts != want || input != "sync -flag edge" {
		t.Fatalf("parseArgs = %+v, %q", opts, input)
	}
	if got := transform("\n/A _ \n\n", opts); got != " - V/" {
		t.Fatalf("combined transforms = %q", got)
	}
}

func TestEmbeddedFontsFromAway(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	entries, err := standardFonts.ReadDir("pyfiglet/fonts")
	if err != nil || len(entries) != 67 {
		t.Fatalf("embedded fonts: count=%d err=%v", len(entries), err)
	}
	for _, name := range []string{"standard", "slant", "small", "big", "mini", "banner", "shadow", "script"} {
		t.Run(name, func(t *testing.T) {
			font, err := loadFont(name)
			if err != nil {
				t.Fatal(err)
			}
			out, err := render(font, options{width: 80, direction: "auto", justify: "auto"}, "agent")
			if err != nil || strings.TrimSpace(out) == "" {
				t.Fatalf("render = %q, %v", out, err)
			}
		})
	}
}
