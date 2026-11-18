# Port pyfiglet to Go

## The repository

`/workspace/repo` is a checkout of **pyfiglet** (https://github.com/pwaller/pyfiglet, MIT licence), a Python implementation of FIGlet. FIGlet turns a short text into large ASCII-art letters. It joins neighbouring letters by "smushing" them together (with six smushing rules and kerning), wraps long text to a width, justifies it, and can write right-to-left, mirror it or flip it over. The Python source in `pyfiglet/__init__.py` is the specification of the behaviour. The fonts are the `.flf` files in `pyfiglet/fonts/`. There is **no Python interpreter** on this machine.

## What to build

Write a Go program in `/workspace/repo` (it needs a `go.mod` at the repository root). Running

```
cd /workspace/repo && go build -o /tmp/figlet .
```

must produce a command-line program that takes the same options as `python -m pyfiglet`:

| Option | Meaning |
|---|---|
| `-f FONT` | font name; the font file is `FONT.flf` (default `standard`) |
| `-w COLS` | width used for wrapping and justification (default `80`) |
| `-j auto\|left\|center\|right` | justification (default `auto`) |
| `-D auto\|left-to-right\|right-to-left` | text direction (default `auto`) |
| `-r` | mirror the output |
| `-F` | flip the output upside down |
| `-s` | strip empty leading and trailing lines |
| `-n` | make the output start and end with one empty line |

The options come first, followed by the text. If the text is given as several arguments they are joined with single spaces.

For every input where pyfiglet itself runs successfully, your program must print **exactly the same bytes** to standard output as pyfiglet: every space, every newline, UTF-8 encoded. This includes how options combine: the width given with `-w` is the width used by `-j center` and `-j right`, `-D right-to-left` changes how letters are joined, and when `-s` and `-n` are both given pyfiglet's own priority applies.

## Fonts

The `.flf` files may be moved to another folder, or embedded in the program (for example with `go:embed`), but they must stay inside the repository. The program must give the same output when it is started from any working directory.

## Hard rules

- Go standard library only: `go.mod` must not require any other module.
- When you finish, there must be **no `.py` file anywhere** under `/workspace/repo`. Delete the original Python files.
- `go build ./...` run in `/workspace/repo` must succeed.

## Not checked

Speed, memory use, error messages and exit codes for bad input, and the options `-c`, `-L`, `-l` and `-i`.

## Grading

The solution is graded automatically on hidden cases and receives a partial score. Three things count: the program builds as described above, the repository contains no `.py` files while the program still works, and the share of cases whose output matches pyfiglet exactly (this part counts most).
