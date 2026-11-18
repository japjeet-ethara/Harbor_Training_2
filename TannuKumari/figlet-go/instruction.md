# Port this FIGlet renderer to Go

## 1. What is in `/workspace/repo`

`/workspace/repo` is a copy of **pyfiglet** (MIT licensed), a Python
implementation of FIGlet: it turns a short text into large letters built from
ASCII characters, reading letter shapes from `.flf` font files. The interesting
part is `pyfiglet/__init__.py`: a font parser (`FigletFont`) and a renderer
(`FigletBuilder`) that pushes neighbouring letters together with kerning and six
"smushing" rules, wraps text to a terminal width, justifies it, and can render
right-to-left, mirrored or upside down. The font files live in `pyfiglet/fonts/`
(with a copy in `pyfiglet/fonts-standard/`).

There is no Python interpreter in this container and you cannot install one.

## 2. What to build

A Go command-line program in `/workspace/repo` such that, run from
`/workspace/repo`:

```
go build -o /tmp/figlet .
```

produces a binary that accepts the same options as `python -m pyfiglet` and
prints the same bytes.

Options to support, given before the text arguments:

| Option | Argument | Meaning |
|---|---|---|
| `-f`, `--font` | FONT name (default `standard`) | font to render with |
| `-w`, `--width` | COLS (default `80`) | terminal width for wrapping and justification |
| `-j`, `--justify` | `auto`, `left`, `center`, `right` (default `auto`) | justification |
| `-D`, `--direction` | `auto`, `left-to-right`, `right-to-left` (default `auto`) | print direction |
| `-r`, `--reverse` | (flag) | mirror image of the output |
| `-F`, `--flip` | (flag) | flip the output upside down |
| `-s`, `--strip-surrounding-newlines` | (flag) | remove blank leading and trailing lines |
| `-n`, `--normalize-surrounding-newlines` | (flag) | exactly one blank line before and after |

Remaining arguments are the text; they are joined with single spaces, exactly as
the Python program does. The graded cases always use the separated form
(`-f slant`, `-w 40`), never `-fslant` or `--font=slant`.

**Byte for byte** means exactly that: every space, every trailing space, every
newline, including the extra newline the Python program writes at the end. Text
is handled per Unicode code point, not per byte.

## 3. Fonts

* The `.flf` files may be moved or renamed into any directory inside
  `/workspace/repo`, or embedded into the binary with `go:embed`. They must stay
  inside the repository, because the repository is all that is kept.
* The program must find its fonts no matter which directory it is started from;
  it is graded from a different working directory.

## 4. Hard rules

1. Go standard library only. No third-party modules, no network access at build
   or run time.
2. **No `.py` file may remain anywhere under `/workspace/repo`** when you are
   done. Delete them; the Go port is the only implementation left.
3. `go build ./...` must succeed in `/workspace/repo`.
4. Do not change anything outside `/workspace/repo`.

## 5. Not checked

Speed, memory, error messages, exit codes on bad input, help text, and the
options `-c/--color`, `-L/--load`, `-l/--list_fonts`, `-i/--info_font`.

## 6. Grading

An automatic grader runs your binary on a large hidden set of option and text
combinations and compares its standard output with the original
implementation's output, byte for byte. The score is partial: it rises with the
share of combinations you reproduce exactly, and also rewards a clean build and
a repository free of Python. Common words are easy; the score is decided by the
awkward cases — narrow widths that force wrapping, runs of `|`, `_`, `/`, `\`
and brackets, double spaces, accented letters, characters the font does not
contain, right-to-left combined with right or centre justification, and fonts
that use kerning only or universal smushing.
