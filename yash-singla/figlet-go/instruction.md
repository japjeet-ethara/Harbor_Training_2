# FIGlet-compatible renderer (Go)

## 1. Repository
This repository contains a pinned snapshot of **pyfiglet** (a pure-Python implementation of FIGlet for rendering text as ASCII art using FIGlet font files). The snapshot is based on commit `255827efbc09d662875dad50fa0edc1e1263bf45` (`2026-08-02T22:51:18+01:00`, version `1.0.4`, MIT license). FIGlet fonts (`.flf`) are included in the repository and define the glyph shapes and layout/smushing behavior used when rendering text.

## 2. What to build
Implement a Go program in `/workspace/repo`. Running:

```sh
go build -o /tmp/figlet .
```

from `/workspace/repo` must produce a command-line program. That program must accept the same options as `pyfiglet`:

- `-f FONT` — font to render with (default: `standard`)
- `-w COLS` — terminal width for wrapping/justification (default: `80`)
- `-j auto|left|center|right` — justification (default: `auto`)
- `-D auto|left-to-right|right-to-left` — text direction (default: `auto`)
- `-r` — reverse/mirror the rendered output
- `-F` — flip the rendered output vertically
- `-s` — strip surrounding (leading/trailing) blank lines
- `-n` — normalize surrounding newlines (one empty line before and after), unless `-s` is also given

The options are followed by the text to render. For any supported input and combination of these options, your program must produce the **same output as pyfiglet, byte for byte** (including spacing and newline behavior).

## 3. Fonts
The `.flf` font files may be moved within the repository, but they must remain in the repository. Your program must locate and use the required fonts correctly regardless of the current working directory from which it is invoked.

## 4. Hard rules
- Use the **Go standard library only** (no third-party modules/packages).
- There must be **no `.py` files anywhere in the repository**.
- `go build ./...` run in `/workspace/repo` must pass.

## 5. Not checked
The following are not checked: speed, memory usage, error messages, and the options `-c`, `-L`, `-l`, and `-i`.

## 6. Grading
Your submission is evaluated against hidden automatic cases. Scoring is partial, based on the fraction of cases that match exactly.
