# Port pyfiglet to Go

## 1. The repository

`/workspace/repo` holds a copy of **pyfiglet** (https://github.com/pwaller/pyfiglet, commit
`255827efbc09d662875dad50fa0edc1e1263bf45`, MIT licence), a Python implementation of FIGlet. It turns
text into large letters drawn with ASCII characters, using font files (`.flf`). The original Python
source is the specification. Python itself is not installed in this environment.

## 2. What to build

Write a Go program in `/workspace/repo`, so that running this in `/workspace/repo`

```
go build -o /tmp/figlet .
```

produces a command-line program `/tmp/figlet` that behaves like `python -m pyfiglet`. It must accept these options:

| option | values | default |
|---|---|---|
| `-f FONT` | the name of a font that exists in `pyfiglet/fonts-standard` (for example `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`, `script`) | `standard` |
| `-w COLS` | integer width used for line wrapping and justification | `80` |
| `-j SIDE` | `auto`, `left`, `center`, `right` | `auto` |
| `-D DIRECTION` | `auto`, `left-to-right`, `right-to-left` | `auto` |
| `-r` | mirror the output | off |
| `-F` | flip the output upside down | off |
| `-s` | strip empty leading and trailing lines | off |
| `-n` | output has one empty line before and after | off |

The options come first, followed by the text to draw. If several text arguments are given, they are
joined with a single space. `-j center` and `-j right` position each line inside the width given by `-w`
(or 80 when `-w` is not given).

For the same arguments, the program must write to standard output **exactly the same bytes** as
`python -m pyfiglet` (UTF-8, including the final newline) for the pyfiglet commit above.

## 3. Fonts

The `.flf` font files may be moved, but they must stay inside `/workspace/repo` (or be embedded in the
program). The program must give the same output no matter which working directory it is started from.

## 4. Hard rules

- Use only the Go standard library.
- No `.py` file may exist anywhere under `/workspace/repo` when you finish.
- `go build ./...` must succeed in `/workspace/repo`.

## 5. Not checked

Speed, memory use, error messages and exit codes for bad input, and the options `-c`, `-L`, `-l`, `-i`.

## 6. Grading

Your work is graded automatically with hidden cases and gets a partial score between 0 and 1. It combines
whether the program builds, whether the repository is free of `.py` files and the program really works,
and the share of hidden cases for which your output matches pyfiglet exactly.
