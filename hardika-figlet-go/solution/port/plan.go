package main

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var invalidOverlap = errors.New("invalid glyph overlap")

type operation struct {
	glyph         glyph
	overlap       int
	previousWidth int
	lineBreak     bool
}

type renderingPlan struct {
	layout fontLayout
	policy collisionPolicy
	width  int
	align  string
	ops    []operation
}

type checkpoint struct {
	position int
	opCount  int
}

func planRendering(f font, text string, opts options) (renderingPlan, error) {
	rtl := opts.direction == "right-to-left" || opts.direction == "auto" && f.layout.rtl
	align := opts.justify
	if align == "auto" && rtl {
		align = "right"
	}
	plan := renderingPlan{
		layout: f.layout,
		policy: collisionPolicy{mode: f.layout.horizontal, hardblank: f.layout.hardblank, rtl: rtl},
		width:  opts.width,
		align:  align,
	}
	rows := make([]string, f.layout.height)
	previousWidth := 0
	retried := false
	blank := checkpoint{position: -1}
	characters := []rune(text)
	for position := 0; position < len(characters); {
		ch := characters[position]
		g, found := f.glyphs[ch]
		if ch == ' ' || ch == '\n' {
			blank = checkpoint{position, len(plan.ops)}
		}
		if ch == '\n' {
			plan.ops = append(plan.ops, operation{lineBreak: true})
			clear(rows)
			previousWidth = 0
			blank.position = -1
			position++
			continue
		}
		if !found {
			position++
			continue
		}
		if g.width > opts.width {
			return renderingPlan{}, fmt.Errorf("width too small for %U", ch)
		}
		overlap := plan.policy.overlap(rows, g, previousWidth)
		if utf8.RuneCountInString(rows[0])+g.width-overlap >= opts.width {
			if blank.position >= 0 {
				plan.ops = plan.ops[:blank.opCount]
				position = blank.position + 1
			} else {
				if retried {
					return renderingPlan{}, fmt.Errorf("no room for %U on an empty line", ch)
				}
				retried = true
			}
			plan.ops = append(plan.ops, operation{lineBreak: true})
			clear(rows)
			blank.position = -1
			previousWidth = g.width
			continue
		}
		if err := plan.policy.place(rows, g, overlap, previousWidth); err != nil {
			return renderingPlan{}, err
		}
		plan.ops = append(plan.ops, operation{glyph: g, overlap: overlap, previousWidth: previousWidth})
		retried = false
		previousWidth = g.width
		position++
	}
	if rows[0] != "" {
		plan.ops = append(plan.ops, operation{lineBreak: true})
	}
	return plan, nil
}

func (p renderingPlan) serialize() (string, error) {
	rows := make([]string, p.layout.height)
	var out strings.Builder
	for _, op := range p.ops {
		if !op.lineBreak {
			if err := p.policy.place(rows, op.glyph, op.overlap, op.previousWidth); err != nil {
				return "", err
			}
			continue
		}
		for _, row := range rows {
			available := p.width - utf8.RuneCountInString(row)
			padding := 0
			switch p.align {
			case "right":
				padding = available - 1
			case "center":
				padding = available / 2
			}
			out.WriteString(strings.Repeat(" ", max(padding, 0)))
			out.WriteString(strings.ReplaceAll(row, string(p.layout.hardblank), " "))
			out.WriteByte('\n')
		}
		clear(rows)
	}
	return out.String(), nil
}
