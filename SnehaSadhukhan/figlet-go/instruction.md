# Port pyfiglet to Go

## The repository

`/workspace/repo` contains **pyfiglet**, a pure-Python implementation of FIGlet: a
program that turns ordinary text into large letters drawn with ASCII characters.
All of its logic lives in `pyfiglet/__init__.py`. It reads a FIGlet font file
(`.flf`) — a header line describing the font (hardblank character, letter height,
print direction and layout/smushing flags), followed by comment lines and the
drawing of every character — and then renders the input text one character at a
time. Each new letter is slid as far left as the font's layout rules allow:
*kerning* moves it until it just touches the previous letter, and *smushing* moves
it one column further and merges the two touching characters using the font's
smushing rules (equal character, underscore, hierarchy, opposite pair, big X and
hardblank). Lines that become too wide are word-wrapped, each line is justified,
hardblanks are turned into spaces, and the result may be mirrored, flipped or have
its surrounding blank lines adjusted. The standard fonts are in
`pyfiglet/fonts-standard/` and a copy in `pyfiglet/fonts/`.

## What to build

Write a Go program in `/workspace/repo`. Running the following from
`/workspace/repo` must produce the program:

```
go build -o /tmp/figlet .
```

The program is invoked as:

```
/tmp/figlet [options] TEXT...
```

Options may appear anywhere among the arguments, before, between or after the
words of the text. Every argument that is not an option (or an option's value) is
part of the text; these arguments are joined with single spaces in their original
order. An argument `--` ends option parsing: everything after it is text, even if
it starts with `-`.

Its standard output must match what pyfiglet's command-line tool (`pyfiglet`,
`python -m pyfiglet`) prints for the same arguments **byte for byte**, including
every space and every newline.

### Options

| Option | Meaning | Default |
|---|---|---|
| `-f FONT` | Font to render with, given by name without extension (e.g. `standard`, `slant`). | `standard` |
| `-w COLS` | Output width in columns, used for wrapping and justification (see below). | `80` |
| `-j auto\|left\|center\|right` | Justification (see below). | `auto` |
| `-D auto\|left-to-right\|right-to-left` | Print direction. `auto` uses the direction stored in the font header (`0` or missing = left-to-right, `1` = right-to-left). | `auto` |
| `-r` | Reverse: mirror the output left-to-right, as pyfiglet does. | off |
| `-F` | Flip: turn the output upside down, as pyfiglet does. | off |
| `-s` | Strip surrounding newlines (see below). | off |
| `-n` | Normalize surrounding newlines (see below). | off |

Post-processing happens in pyfiglet's order: render, then `-r`, then `-F`, then
`-s` or `-n`. If both `-s` and `-n` are given, `-s` takes effect.

`-s` works on the rendered result as follows:

1. Every row at the top that is empty or contains only whitespace is removed.
2. All trailing whitespace is then stripped from the end of the whole result. This
   removes whitespace-only rows at the bottom, any trailing spaces on the last
   remaining row, and the final newline.
3. The program then writes the result followed by a single newline.

Rows in the middle of the output are kept unchanged (even if they are blank), and
leading spaces on rows, including justification padding, are preserved.

`-n` takes the result of steps 1 and 2 above and outputs a newline, then that
result, then a newline, and the program then writes its final newline. The output
therefore starts with one empty line and ends with one empty line.

### Width (`-w`) precisely

- `COLS` is a number of characters (Unicode code points). The width of a rendered
  line is the length of its **first row**, measured *before* hardblanks are
  replaced by spaces.
- Characters are added to the current output line while the line stays **shorter
  than** `COLS`. When adding the next character (after kerning/smushing) would make
  the line `COLS` characters or longer, the line is broken. A rendered line
  therefore never exceeds `COLS - 1` characters before justification.
- Lines break at the most recent space in the text (word wrap). The line that is
  output is the line as it was just **before** that space was added, so the space
  glyph is omitted from the finished line, and it is not carried to the next line
  either; rendering continues with the character after the space. Any characters
  that had already been added after that space are rendered again on the new line.
  If the current line has no space, the break happens just before the character
  that did not fit.
- A newline character in the text always starts a new output line.

### Justification (`-j`) precisely

Justification is applied separately to every row of every output line, by
**prepending** spaces to that row; no spaces are ever appended. `len(row)` below is
the row's length in characters before hardblanks are replaced.

| Value | Spaces prepended to each row |
|---|---|
| `left` | none |
| `center` | `floor((COLS - len(row)) / 2)` |
| `right` | `COLS - len(row) - 1` |
| `auto` | same as `left` when the effective direction is left-to-right, same as `right` when it is right-to-left |

A negative amount means no spaces are added. Because the amount is computed for
each row from that row's own length, rows of the same line may receive different
amounts of padding.

## Fonts

The `.flf` font files may be moved or reorganised, but they must remain inside the
repository. The program must work correctly from **any working directory**: it is
built to `/tmp/figlet` and may be run from anywhere, so it cannot rely on being
started inside `/workspace/repo`.

## Hard rules

- Use the **Go standard library only**; no third-party modules.
- There must be **no `.py` files anywhere in the repository** when you are done.
- `go build ./...` must succeed when run in `/workspace/repo`.

## Not checked

- Speed
- Memory usage
- Error messages
- The options `-c`, `-L`, `-l` and `-i`

## Grading

Your program is graded on hidden, automatically run cases. Each case compares your
program's output with pyfiglet's, and partial credit is awarded.
