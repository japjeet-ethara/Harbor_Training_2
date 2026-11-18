# Port pyfiglet to Go

## 1. The repository

`/workspace/repo` contains pyfiglet (https://github.com/pwaller/pyfiglet, commit
`255827efbc09d662875dad50fa0edc1e1263bf45`, MIT licence), a pure-Python implementation of
FIGlet. It renders text as large ASCII-art letters using FIGlet font files (`.flf`). Its
logic is in `pyfiglet/__init__.py`. That file loads fonts, joins letters together
("smushing", controlled by each font's layout bits), wraps and justifies lines, and parses
the command-line options in `main()`. The 67 standard fonts are in `pyfiglet/fonts/`.
Python is not installed, so you cannot run pyfiglet. Read its source; that source is the
specification for your program.

## 2. What to build

Write a Go program in `/workspace/repo`. Running `go build -o /tmp/figlet .` in
`/workspace/repo` must produce a command-line program with this usage:

```
/tmp/figlet [options] TEXT...
```

| Option | Meaning |
|---|---|
| `-f FONT` | Font name, without its extension. Default: `standard`. |
| `-w COLS` | Output width. Default: `80`. |
| `-j auto\|left\|center\|right` | Justification. Default: `auto`. |
| `-D auto\|left-to-right\|right-to-left` | Print direction. Default: `auto`. |
| `-r` | Mirror the output left to right. |
| `-F` | Flip the output upside down. |
| `-s` | Strip blank lines before and after the output. |
| `-n` | Exactly one blank line before and after the output. |

Options are given as separate arguments before the text, and several may be used together
(for example `-f slant -w 40 -j center -r`). All remaining arguments, joined with single
spaces, are the text.

For every valid input, the standard output must be **byte for byte** the same as
`python -m pyfiglet` would print with the same arguments. This includes spaces, blank
lines and the final newline. Some points that are easy to get wrong:

- `-w` sets the width used both for wrapping and for `-j`. For example, with `-w 40 -j center`
  the text is centred within 40 columns, not 80. Copy pyfiglet's exact padding arithmetic
  for `center` and `right`.
- Apply the steps in the order `main()` applies them: render, then `-r`, then `-F`, then
  `-s` or `-n`. If both `-s` and `-n` are given, `-s` wins.
- Smushing must follow `FigletBuilder.smushChars` and `smushAmount` exactly. Where a code
  comment disagrees with the code, the code is correct.

## 3. Fonts

You may move or reorganise the `.flf` files, but they must stay inside `/workspace/repo`.
`/tmp/figlet` must work when run from any working directory.

## 4. Hard rules

- Use only the Go standard library. No third-party modules.
- No `.py` file may remain anywhere in `/workspace/repo` when you finish. Delete the
  Python sources once you have read them.
- `go build ./...` in `/workspace/repo` must succeed.

## 5. Not checked

Speed, memory use, and error messages are not checked, and neither are the
options `-c`, `-L`, `-l` and `-i`. You do not need to implement those options.

## 6. Grading

Your program is graded automatically on cases you cannot see, each comparing its output
with pyfiglet's. You receive partial credit for each case that matches.
