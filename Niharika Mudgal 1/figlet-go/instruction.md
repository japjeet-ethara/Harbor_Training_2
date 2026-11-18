# Port pyfiglet to Go

## 1. The repository

`/workspace/repo` contains **pyfiglet** (https://github.com/pwaller/pyfiglet), a Python program that draws text as giant letters made of ASCII characters (FIGlet). The Python source is in `pyfiglet/__init__.py` and the fonts are the `.flf` files in `pyfiglet/fonts/`. Python is **not** installed in this environment, so you cannot run the original: read its source. Its behaviour, including its quirks, is the specification.

## 2. What to build

A Go program in `/workspace/repo`. Running `go build -o /tmp/figlet .` in `/workspace/repo` must produce a command-line program:

```
/tmp/figlet [options] TEXT...
```

It accepts the same options as pyfiglet, always written **before** the text:

| Option | Meaning | Values (default) |
|---|---|---|
| `-f FONT` | font name | `standard` |
| `-w COLS` | width in columns, used for wrapping **and** for justification | integer (`80`) |
| `-j SIDE` | justification | `auto`, `left`, `center`, `right` (`auto`) |
| `-D DIR` | direction | `auto`, `left-to-right`, `right-to-left` (`auto`) |
| `-r` | mirror the output | off |
| `-F` | flip the output upside down | off |
| `-s` | strip empty lines at the start and end of the output | off |
| `-n` | output has exactly one empty line before and after | off |

`TEXT` is all remaining arguments joined with a single space. It may contain any Unicode characters and newline characters.

For the same arguments, the program prints **exactly the same bytes to standard output as pyfiglet's command line** (`python -m pyfiglet ...`): UTF-8, including the final newline. `-j` and `-w` use the same width: `-j center -w 40` centres within 40 columns, not 80. The program must support at least these fonts: `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`, `script`.

## 3. Fonts

The `.flf` font files may be moved or renamed, but they must stay inside the repository. The built program must work when it is started from any working directory (for example from `/`), so it must not depend on the current directory. Embedding the fonts with `go:embed` is one way to do this.

## 4. Hard rules

- Go standard library only (no third-party modules, no network at build time).
- No `.py` file may exist anywhere under `/workspace/repo` when you finish. Delete the Python sources once you no longer need them.
- `go build ./...` must pass in `/workspace/repo`.

## 5. Not checked

Speed, memory, error messages, exit status; the options `-c`, `-L`, `-l`, `-i`; and any input on which pyfiglet itself crashes or never finishes (for example a `-w` value too small for one letter).

## 6. Grading

Your work is graded by hidden automatic cases. The score is a weighted mean, so partial credit is possible: 1 part for a successful build, 1 part for having no Python left (only counted once the program produces at least some correct output), and 4 parts for the share of cases where your output is byte-for-byte identical to pyfiglet's.
