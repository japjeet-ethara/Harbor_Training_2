package main

import (
	"os"
	"slices"
	"testing"
)

func TestCollisionRules(t *testing.T) {
	tests := []struct {
		name              string
		mode              int
		left, right, want rune
		rtl               bool
	}{
		{"equal", smush | equal, '|', '|', '|', false},
		{"lowline", smush | lowline, '_', '/', '/', false},
		{"hierarchy", smush | hierarchy, '/', '[', '[', false},
		{"opposing braces", smush | pair, '}', '{', '|', false},
		{"big X", smush | bigX, '>', '<', 'X', false},
		{"slash order A", smush | bigX, '/', '\\', '|', false},
		{"slash order B", smush | bigX, '\\', '/', 'Y', false},
		{"hardblanks", smush | hardblank, '$', '$', '$', false},
		{"protected hardblank", smush | equal, '$', '$', 0, false},
		{"fitting gap", kern, ' ', '$', '$', false},
		{"fitting collision", kern, '|', '|', 0, false},
		{"universal hardblank", smush, '$', '[', '[', true},
		{"universal foreground", smush, '[', '_', '[', true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rules := collisionRules{test.mode, '$', test.rtl, 2, 3}
			actual, ok := rules.resolve(test.left, test.right)
			if ok != (test.want != 0) || ok && actual != test.want {
				t.Fatalf("collision gives %q, %v, want %q", actual, ok, test.want)
			}
		})
	}

	rules := collisionRules{smush | equal, '$', false, 1, 3}
	if _, ok := rules.resolve('|', '|'); ok {
		t.Fatal("width-one glyph smushed")
	}
}

func TestFontParserUsesColumnsAndCodePoints(t *testing.T) {
	font, err := loadFont("standard")
	if err != nil {
		t.Fatal(err)
	}
	glyph := font.glyphs['ß']
	if font.height != 6 || font.hardblank != '$' || len(glyph.columns) == 0 {
		t.Fatalf("unexpected standard font or missing sharp s")
	}
	canvas := newCanvas(2, 4)
	canvas.setRow(0, []rune(" é $"))
	canvas.setRow(1, []rune("  "))
	expected := []rowExtent{{4, 1, 3}, {2, 2, -1}}
	if !slices.Equal(canvas.rows, expected) || canvas.columns[1][0] != 'é' {
		t.Fatalf("unexpected edges or Unicode cells: %#v", canvas)
	}
	if _, err := decodeFont("flf2a$ 6 5 20 0 0"); err == nil {
		t.Fatal("accepted truncated glyph data")
	}
}

func TestCLIOptionsAndTransformations(t *testing.T) {
	opts, text, err := parseOptions([]string{"tea", "-Fsn", "-w20", "-D", "right-to-left", "--", "-time"})
	if err != nil || opts.width != 20 || !opts.flip || !opts.normalize || !opts.strip || text != "tea -time" {
		t.Fatalf("parse result: %+v %q %v", opts, text, err)
	}
	if got := finish("\nABC  \n\n", options{strip: true, normalize: true}); got != "ABC\n" {
		t.Fatalf("strip precedence gives %q", got)
	}
	if got := finish(" /{}\n", options{reverse: true}); got != "{}\\ \n\n" {
		t.Fatalf("mirror gives %q", got)
	}
	if got := finish("V_\n^m\n", options{flip: true}); got != "vw\nA-\n\n" {
		t.Fatalf("flip gives %q", got)
	}
}

func TestFontsOutsideRepository(t *testing.T) {
	dir := t.TempDir()
	start, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(start); err != nil {
			t.Error(err)
		}
	})
	output, err := run([]string{"-f", "mini", "tea"})
	if err != nil || output == "" {
		t.Fatalf("embedded font lookup from %s: %q, %v", dir, output, err)
	}
}
