# Port pyfiglet to Go

## 1. The repository

`/workspace/repo` contains pyfiglet (<https://github.com/pwaller/pyfiglet>,
commit `255827efbc09d662875dad50fa0edc1e1263bf45`, MIT licence), a pure-Python
implementation of FIGlet. FIGlet draws text as large letters built from ASCII
characters, using font files (`.flf`). When two large letters stand side by
side, pyfiglet moves them together ("kerning") or merges their touching
characters ("smushing") according to the rules stored in each font's header.
It can also wrap long text at a given width, justify it, write right-to-left,
mirror the output and flip it upside down. The rendering logic lives in
`pyfiglet/__init__.py` (mainly the `FigletFont` and `FigletBuilder` classes).
The standard fonts are in `pyfiglet/fonts-standard/` and have already been
copied to `pyfiglet/fonts/`. Python is not installed; read the source to port
it.

## 2. What to build

Write a Go program in `/workspace/repo`. Running this command in
`/workspace/repo`:

```
go build -o /tmp/figlet .
```

must produce a command-line program that takes the same options as pyfiglet
and prints exactly the same bytes to stdout as pyfiglet does for the same
arguments. That includes every space, every trailing space on a line, every
empty line and the final newline.

The `main` package must therefore be at the repository root, not in a
subdirectory.

Options (same meaning and same defaults as pyfiglet):

| Option | Value | Default | Meaning |
|---|---|---|---|
| `-f FONT` | font name, no extension | `standard` | Font to render with. |
| `-w COLS` | integer | `80` | Width used for line wrapping and justification. |
| `-j SIDE` | `auto`, `left`, `center`, `right` | `auto` | Justification. `auto` means `left` when the direction is left-to-right and `right` when it is right-to-left. |
| `-D DIRECTION` | `auto`, `left-to-right`, `right-to-left` | `auto` | Print direction. `auto` uses the direction stored in the font header. |
| `-r` | none | off | Mirror the output horizontally. |
| `-F` | none | off | Flip the output upside down. |
| `-s` | none | off | Strip empty leading and trailing lines from the output. |
| `-n` | none | off | Make the output have exactly one empty line before and after it. If `-s` is also given, `-s` wins. |

Note that `-j`, `-D`, `-f` and `-w` each take the next argument as their
value. Options may appear in any order. All remaining arguments are joined
with single spaces and rendered as the text, exactly as pyfiglet does. The
text may contain any Unicode characters, including Latin-1 letters and
characters the font does not define; handle them the way pyfiglet does.

Fonts that must work: `standard`, `slant`, `small`, `big`, `mini`, `banner`,
`shadow`, `script`.

## 3. Fonts

You may move, copy or embed (for example with `go:embed`) the `.flf` font
files, but they must stay inside `/workspace/repo`. Nothing may be downloaded
at runtime. The built program must work from any working directory.

## 4. Hard rules

- Go standard library only. `go.mod` must have no `require` lines.
- `go build ./...` must succeed in `/workspace/repo`.
- No `.py` file may exist anywhere under `/workspace/repo` when you finish.
- Do not install any program named `python`, `python3`, `pyfiglet`, `figlet`
  or `toilet` on the `PATH` (this includes `go install` of a binary with one
  of those names). Build your program only with the command in Section 2.

## 5. Not checked

Speed, memory use, error messages, exit codes for bad input, and the pyfiglet
options `-c`, `-L`, `-l` and `-i`.

## 6. Grading

Your program is run automatically on a hidden set of argument combinations
covering the options and fonts above, and its stdout is compared with
pyfiglet's. The score is partial: it rewards a successful build, a repository
with no Python left in it, and the share of cases whose output matches
exactly. The Python-removal credit is only given if at least one case
matches.
