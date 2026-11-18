package main

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestStandardFontsParse(t *testing.T) {
	fonts, err := fontFiles.ReadDir("fonts")
	if err != nil || len(fonts) != 67 {
		t.Fatalf("embedded fonts: %d, %v", len(fonts), err)
	}
	for _, item := range fonts {
		name := strings.TrimSuffix(item.Name(), ".flf")
		t.Run(name, func(t *testing.T) {
			f, err := loadFont(name)
			if err != nil {
				t.Fatal(err)
			}
			if f.layout.height < 1 || len(f.glyphs[' '].rows) != f.layout.height {
				t.Fatal("invalid font height or space matrix")
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("a\r\nb\vc\f\u001cd\u0085e\u2029")
	if !reflect.DeepEqual(got, []string{"a", "b", "c", "", "d", "e"}) {
		t.Fatalf("splitLines = %q", got)
	}
}

func TestCollisions(t *testing.T) {
	tests := []struct {
		name              string
		mode              layoutMode
		left, right, want rune
		rtl               bool
		previousWidth     int
		ok                bool
	}{
		{"equal", smushing | equal, '|', '|', '|', false, 2, true},
		{"underscore", smushing | lowline, '_', '/', '/', false, 2, true},
		{"hierarchy", smushing | hierarchy, '/', '>', '>', false, 2, true},
		{"opposite", smushing | opposite, ']', '[', '|', false, 2, true},
		{"forward slash first", smushing | bigX, '/', '\\', '|', false, 2, true},
		{"backward slash first", smushing | bigX, '\\', '/', 'Y', false, 2, true},
		{"big X", smushing | bigX, '>', '<', 'X', false, 2, true},
		{"hardblank pair", smushing | hardblankPair, '$', '$', '$', false, 2, true},
		{"hardblank is protected", smushing | equal, '$', '$', 0, false, 2, false},
		{"kerning", kerning, ' ', '/', '/', false, 1, true},
		{"no smush while kerning", kerning, '|', '|', 0, false, 2, false},
		{"one column", smushing | equal, '|', '|', 0, false, 1, false},
		{"universal LTR", smushing, 'a', 'b', 'b', false, 2, true},
		{"universal RTL", smushing, 'a', 'b', 'a', true, 2, true},
		{"universal ink wins", smushing, '$', 'b', 'b', true, 2, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := collisionPolicy{mode: tc.mode, hardblank: '$', rtl: tc.rtl}
			got, ok := policy.merge(tc.left, tc.right, tc.previousWidth, 2)
			if got != tc.want && tc.ok || ok != tc.ok {
				t.Fatalf("merge = (%q, %t), want (%q, %t)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestPlanReplaysBlankRollback(t *testing.T) {
	f, err := loadFont("term")
	if err != nil {
		t.Fatal(err)
	}
	p, err := planRendering(f, "AB CD\nEF", options{direction: "left-to-right", justify: "left", width: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ops) != 9 {
		t.Fatalf("wanted 6 placements and 3 breaks, got %d operations", len(p.ops))
	}
	if got, err := p.serialize(); err != nil || got != "AB\nCD\nEF\n" {
		t.Fatalf("serialize = %q, %v", got, err)
	}
}

func TestCLI(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default fonts embedded", []string{"--font", "term", "A"}, "A\n\n"},
		{"options interspersed", []string{"A", "-fterm", "B", "-jleft"}, "A B\n\n"},
		{"literal hyphen with terminator", []string{"-fterm", "--", "-w"}, "-w\n\n"},
		{"transforms", []string{"-fterm", "-rFns", "A/\n_"}, "-\n/V\n"},
		{"normalization", []string{"-fterm", "-n", " A  \n\n"}, "\n A\n\n"},
		{"legacy octal width", []string{"-fterm", "-w010", "ABCDEFGH"}, "ABCDEFG\nH\n\n"},
		{"hexadecimal width", []string{"-fterm", "-w0x8", "ABCDEFGH"}, "ABCDEFG\nH\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(tc.args, &out); err != nil || out.String() != tc.want {
				t.Fatalf("run = %q, %v, want %q", out.String(), err, tc.want)
			}
		})
	}
}

func TestEmbeddedFontFromAnotherWorkingDirectory(t *testing.T) {
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
	var out bytes.Buffer
	if err := run([]string{"-fterm", "A"}, &out); err != nil || out.String() != "A\n\n" {
		t.Fatalf("run = %q, %v", out.String(), err)
	}
}
