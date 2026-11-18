# Port pyfiglet to Go

## 1. The repository

`/workspace/repo` contains **pyfiglet** (MIT licence), a pure-Python implementation of FIGlet. FIGlet
draws text as large letters built from ordinary characters, using font files (`.flf`). When two large
letters stand next to each other it slides them together and merges ("smushes") the characters
where they touch, following rules declared in the font header (equal characters, underscores,
a hierarchy of `| /\ [] {} () <>`, opposite brackets, and others). It also wraps long text to a
width, justifies it, can lay it out right-to-left, mirror it and flip it upside down. The code is in
`pyfiglet/__init__.py` (`FigletFont`, `FigletBuilder`, `FigletString`, `main`). The fonts are in
`pyfiglet/fonts/`. There is no Python interpreter in this environment.

## 2. What to build

A Go command-line program in `/workspace/repo` that behaves exactly like pyfiglet's `main()`.

- Running `go build -o /tmp/figlet .` in `/workspace/repo` must produce the program.
- It is called as `/tmp/figlet [options] TEXT...`. Options always come first, each as a separate
  argument with its value as the next argument (e.g. `-f slant -w 40 -r hello world`). The first
  argument that is not an option or an option value starts the text. The text never starts with `-`.
- TEXT may be one or more arguments. They are joined with a single space, exactly as pyfiglet does.
  Text may contain any Unicode characters, including spaces, repeated spaces and newlines.
  Characters the font does not define are skipped, as in pyfiglet.
- Options (defaults in brackets):

  | Option | Meaning |
  |--------|---------|
  | `-f FONT` | font name [`standard`]. At least `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`, `script` must work. |
  | `-w COLS` | output width used for wrapping and justification [`80`]. COLS is a positive integer. |
  | `-j auto\|left\|center\|right` | justification [`auto`]. `center` and `right` are relative to COLS (the `-w` value), not to 80. |
  | `-D auto\|left-to-right\|right-to-left` | print direction [`auto`]. |
  | `-r` | mirror the output. |
  | `-F` | flip the output upside down. |
  | `-s` | strip surrounding empty lines. |
  | `-n` | normalize surrounding empty lines. |

  The flags `-r`, `-F`, `-s`, `-n` take no value and may be combined.
- The program writes to standard output **the same bytes** pyfiglet writes to standard output for the
  same arguments: every space (leading and trailing), every line, and the final newline(s), encoded as
  UTF-8. The reference is pyfiglet's actual behaviour at this commit, including its quirks, not the
  FIGlet specification and not the C `figlet` program.
- Inputs on which pyfiglet fails with an exception (e.g. a character wider than COLS) or never
  finishes are not used, so what your program does for them does not matter.

## 3. Fonts

The `.flf` font files may be moved or renamed, but they must remain inside `/workspace/repo`. The
program must work from **any working directory**, including when only the built binary
`/tmp/figlet` is run (for example, embed the fonts with `go:embed`).

## 4. Hard rules

- Go standard library only (Go 1.23 is installed; no network modules, no cgo).
- When you are done there must be **no `.py` file anywhere under `/workspace/repo`** (this includes
  `pyfiglet/fonts/__init__.py`). You may read the Python code before you delete it.
- `go build ./...` in `/workspace/repo` must succeed.

## 5. Not checked

Speed, memory use, exit codes, anything written to standard error, error messages, invalid option
values, and the options `-c`, `-L`, `-l` and `-i`.

## 6. Grading

Grading is automatic, on a hidden set of inputs that combine the options above. You get partial
credit: a part for a working build with no Python left in the repository, and a part for the share of
inputs whose standard output matches pyfiglet's exactly. A program that builds but gets no output
right scores very low.
