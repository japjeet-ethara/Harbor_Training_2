# Port pyfiglet to Go

## 1. The repository

`/workspace/repo` contains **pyfiglet** (MIT licence, commit
`255827efbc09d662875dad50fa0edc1e1263bf45`), a Python implementation of FIGlet. It
renders short text as large ASCII-art letters using `.flf` font files. Adjacent letters
are pushed together ("smushed") according to the layout rules stored in each font's
header, and the result can be wrapped, justified, printed right-to-left, mirrored and
flipped. The Python code lives in `pyfiglet/__init__.py`; the fonts are in
`pyfiglet/fonts/`. There is no Python interpreter in this environment.

## 2. What to build

A Go program in `/workspace/repo`. Running

```
cd /workspace/repo && go build -o /tmp/figlet .
```

must produce a command-line program `/tmp/figlet` that accepts the same options as
`python -m pyfiglet`:

| option | values |
|---|---|
| `-f FONT` | a font name, default `standard` |
| `-w COLS` | integer width, default `80` |
| `-j SIDE` | `auto`, `left`, `center`, `right` (default `auto`) |
| `-D DIRECTION` | `auto`, `left-to-right`, `right-to-left` (default `auto`) |
| `-r` | mirror the output |
| `-F` | flip the output upside down |
| `-s` | strip surrounding empty lines |
| `-n` | normalize surrounding empty lines |

Options come first, then the text. All remaining arguments are joined with a single
space to form the text (UTF-8). For every such command line, the program must write to
standard output **exactly the bytes** pyfiglet at the commit above writes: the same
characters, the same spaces (including trailing spaces), the same empty lines and the
same final newline(s).

Behaviour is defined by pyfiglet's code, not by the FIGlet specification or by the C
`figlet` program. In particular, `-w` is the width used for both wrapping and
justification: `-j center` and `-j right` position the text inside `-w` columns.

The fonts `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow` and `script`
(from `pyfiglet/fonts/`) must work.

## 3. Fonts

You may move the `.flf` files or embed them in the binary, but they must stay inside
`/workspace/repo`. `/tmp/figlet` must work when started from any working directory.

## 4. Hard rules

* Go standard library only (no third-party modules).
* No `.py` files anywhere under `/workspace/repo` when you finish.
* `go build ./...` in `/workspace/repo` must succeed.

## 5. Not checked

Speed, memory use, error messages and exit codes, inputs for which pyfiglet itself
fails or never finishes, and the options `-c`, `-L`, `-l`, `-i`.

## 6. Grading

Your program is graded automatically on hidden command lines built from the options
above. The score is partial: the more command lines you get exactly right, the higher it
is.
