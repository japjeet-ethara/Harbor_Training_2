# Port pyfiglet to Go

## The repository

`/workspace/repo` contains **pyfiglet** (MIT licence, commit
`255827efbc09d662875dad50fa0edc1e1263bf45`), a Python implementation of FIGlet. It turns a
short text into large letters drawn with ordinary characters, using `.flf` font files.
Adjacent letters are pushed together by kerning or by "smushing" (overlapping one column
according to the layout rules in each font header). The program also wraps long text,
justifies it, can render right-to-left, mirror the result and flip it upside down. The
reference behaviour is the code in `pyfiglet/__init__.py`, especially `FigletFont`,
`FigletBuilder`, `FigletString`, `Figlet` and `main()`. Fonts are in `pyfiglet/fonts/`.
There is no Python interpreter in this environment.

## What to build

A Go program in `/workspace/repo`. Running

```
cd /workspace/repo && go build -o /tmp/figlet .
```

must produce a command-line program that is invoked as

```
/tmp/figlet [options] TEXT...
```

Options are given first, each as a separate argument with its value in the next argument
(for example `-w 40`). All remaining arguments are the text; join them with single spaces,
exactly as pyfiglet's `main()` does. The options are:

| Option | Meaning |
|---|---|
| `-f FONT` | font name without extension (default `standard`) |
| `-w COLS` | width used for wrapping and justification (default `80`) |
| `-j auto\|left\|center\|right` | justification (default `auto`) |
| `-D auto\|left-to-right\|right-to-left` | direction (default `auto`) |
| `-r` | reverse (mirror) the output |
| `-F` | flip the output upside down |
| `-s` | strip surrounding empty lines |
| `-n` | normalize surrounding newlines |

For every combination of these options and any text, the program must write to standard
output exactly the same bytes as `python -m pyfiglet` with the same arguments. Both `-j center` and `-j right` justify within the `-w` width (80 if `-w`
is not given), exactly as pyfiglet does. When an option combination is accepted by
pyfiglet, it must be accepted by your program and produce the same output; this includes
`auto` values, which take their meaning from the font file.

## Fonts

The program must support every `.flf` font that ships in `pyfiglet/fonts/`. You may move
or reorganise the font files (or embed them in the binary with `go:embed`), but they must
stay inside `/workspace/repo`. The built program must work when run from any working
directory.

## Hard rules

- Go standard library only (no third-party modules).
- When you are finished, no file ending in `.py` may exist anywhere under
  `/workspace/repo`.
- `go build ./...` in `/workspace/repo` must succeed.

## Not checked

Speed, memory use, error messages and exit codes for invalid input, inputs that make
pyfiglet raise an exception, and the options `-c`, `-L`, `-l` and `-i`.

## Grading

Your program is graded automatically on hidden inputs. The score is partial: it is
mostly the share of inputs for which your output matches pyfiglet's byte for byte.
