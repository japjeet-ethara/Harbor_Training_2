# Port pyfiglet to Go

Re-implement the `pyfiglet` command line tool (pinned upstream commit
`255827efbc09d662875dad50fa0edc1e1263bf45`) in **Go 1.23, standard library only**.
Your program must reproduce pyfiglet's output **byte for byte**.

## Environment
* Working directory: `/workspace/repo`. You are user `player` (UID 1000). There is **no Python** on this machine.
* Reference source (read it, do not run it): `/workspace/repo/pyfiglet/__init__.py` – study `FigletFont`, `FigletBuilder`, `FigletString`.
* Fonts: `/workspace/repo/pyfiglet/fonts/*.flf`. Your program must load fonts from `pyfiglet/fonts` (relative to the current directory), falling back to `/workspace/repo/pyfiglet/fonts`; the `FIGLET_FONTS` environment variable, if set, is searched first.
* Example cases (args, text, expected output) are in `/workspace/repo/examples/visible.jsonl` (if present). Each line: `{"id","args","text","expected"}`.

## Deliverable
A Go module in `/workspace/repo/figlet/` (module `figlet`, `package main`) that builds with
`cd /workspace/repo/figlet && go build -o figlet .` into the binary `/workspace/repo/figlet/figlet`.

Rules: standard library only (no third-party imports, no `require` in `go.mod`); no `.py` files anywhere in `/workspace/repo/figlet`; never call or mention Python.

## Command line
```
figlet [options] [--] TEXT...
```
Options are parsed left to right and stop at `--` or at the first non-option argument. The remaining arguments are joined with a single space to form the text. If there are none, read the text from stdin (one trailing `\n` removed).

| Option | Meaning |
|---|---|
| `-f FONT` | font name (default `standard`); file `FONT.flf` in the font directory |
| `-w N` | output width in columns (default `80`) |
| `-j auto\|left\|center\|right` | justification (default `auto`) |
| `-D auto\|left-to-right\|right-to-left` | text direction (default `auto`) |
| `-r` | reverse: mirror the finished output left/right (pyfiglet `FigletString.reverse`) |
| `-F` | flip the finished output upside down (pyfiglet `FigletString.flip`) |
| `-s` | use the font's own smushing/kerning layout (the default) |
| `-n` | disable smushing and kerning completely (full-width layout, i.e. `smushMode = 0`) |

`-s` and `-n` may both appear; the **last one wins**. `auto` direction follows the font's print direction; `auto` justification is `left` for left-to-right and `right` for right-to-left.

The program prints the rendered text exactly as pyfiglet's `renderText` returns it (after `-r` and then `-F` are applied), followed by one extra `\n` (like Python's `print`). Invalid options: message on stderr, exit status 2. Rendering errors: message on stderr, exit status 1.

## What must be reproduced
* Font parsing: header, hardblank, old/full layout, comment lines, the 102 required characters, code-tagged characters, end-mark stripping.
* Layout: kerning and the six smushing rules (equal character, underscore, hierarchy, opposite pair, big X, hardblank), universal smushing, right-to-left building.
* Word wrapping at the given width, justification (left / center / right).
* Mirroring (`-r`) and upside-down (`-F`) of the finished output.
* Characters missing from a font, double spaces, punctuation, Latin-1 characters (`é ß ñ`).

## Grading
Build (1) + no Python (1) + behaviour on 2,000 hidden cases (4), divided by 6. Hidden cases use the same format as the visible ones.
