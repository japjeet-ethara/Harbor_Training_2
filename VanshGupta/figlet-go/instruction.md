# pyfiglet in Go

## What this repository is

This repository contains pyfiglet, an MIT-licensed Python implementation of
FIGlet. FIGlet renders text as multi-line ASCII art using FIGfonts. Its
renderer supports character kerning and six smushing rules, line wrapping,
justification, left-to-right and right-to-left output, mirroring, and vertical
flipping.

## What to build

Build a Go program in `/workspace/repo`. Running
`go build -o /tmp/figlet .` from `/workspace/repo` must produce a command-line
program that accepts `-f FONT`, `-w COLS`, `-j auto|left|center|right`,
`-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed
by the text to render. For the same font, options, and text, its standard
output must match pyfiglet byte for byte.

`-w` sets the wrapping width and the width used for justification. Center
justification uses half of the unused columns at that width as left padding.

## Fonts

The `.flf` and `.tlf` font files may be moved or embedded, but they must remain
inside this repository. The program must find and use them regardless of its
current working directory.

## Hard rules

- Use only the Go standard library.
- Leave no `.py` files anywhere in this repository.
- `go build ./...` must pass.

## Not checked

Speed, memory use, error-message wording, and the options `-c`, `-L`, `-l`,
and `-i` are not checked.

## Grading

Hidden automatic cases determine the score. Partial credit is available.
