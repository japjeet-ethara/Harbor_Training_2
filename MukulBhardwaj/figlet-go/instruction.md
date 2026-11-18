# Port pyfiglet to Go

The directory /workspace/repo contains pyfiglet, a Python implementation of FIGlet. FIGlet renders text as large ASCII-art letters using .flf font files. When two letters are placed side by side, they can be squeezed together ("smushed") using several rules, and the program also supports wrapping, justification, text direction, mirroring and flipping.

## What to build

Write a Go program in /workspace/repo. Running `go build -o /tmp/figlet .` in /workspace/repo must produce a command-line program. It must accept the same options as pyfiglet's command line:

- -f FONT: the font name
- -w COLS: the width in columns used for wrapping and justification
- -j auto|left|center|right: justification
- -D auto|left-to-right|right-to-left: text direction
- -r: mirror the output
- -F: flip the output upside down
- -s: strip empty leading and trailing lines
- -n: output has one empty line before and after

The options come first, followed by the text to render. For every input and every combination of these options, the program must print exactly the same output as pyfiglet, byte for byte, including every space. When -j and -w are both given, justification is relative to the -w width.

## Fonts

The .flf font files may be moved, but they must stay inside /workspace/repo (or be embedded in the program). The program must work when run from any working directory.

## Rules

- Go standard library only.
- No .py files may remain anywhere in /workspace/repo.
- `go build ./...` must pass in /workspace/repo.

## Not checked

Speed, memory use, error messages, and the options -c, -L, -l and -i.

## Grading

Your work is graded by hidden automatic checks. The score is partial: you earn credit for each part that passes.