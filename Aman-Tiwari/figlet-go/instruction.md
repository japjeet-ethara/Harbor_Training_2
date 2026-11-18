# Port pyfiglet to Go

## The repository

`/workspace/repo` contains **pyfiglet** (commit `255827efbc09d662875dad50fa0edc1e1263bf45`, MIT licence), a pure-Python implementation of FIGlet. FIGlet draws text as large letters made of ordinary characters, using font files (`.flf`) in `pyfiglet/fonts-standard/` (already copied to `pyfiglet/fonts/`). When two letters stand next to each other, pyfiglet pushes them together ("kerning") or merges their touching edges ("smushing") according to the font's layout rules; it can also wrap long text, justify it, render right-to-left, mirror it and flip it. The rendering logic is in `pyfiglet/__init__.py` (`FigletFont`, `FigletBuilder`, `FigletString`, `main`). Python is not installed in this environment.

## What to build

A Go program in `/workspace/repo`. Running

```
cd /workspace/repo && go build -o /tmp/figlet .
```

must produce a command-line program that behaves like `python -m pyfiglet`. It is called as

```
/tmp/figlet [options] TEXT...
```

with these options, each given as a separate argument before the text (for example `-w 40`, not `-w40`):

| option | meaning in pyfiglet |
|---|---|
| `-f FONT` | font name, e.g. `slant` (default `standard`) |
| `-w COLS` | width used for wrapping **and** for justification (default 80) |
| `-j auto\|left\|center\|right` | justification (default `auto`) |
| `-D auto\|left-to-right\|right-to-left` | print direction (default `auto`) |
| `-r` | mirror the output |
| `-F` | flip the output upside down |
| `-s` | strip surrounding empty lines |
| `-n` | normalize surrounding empty lines |

All remaining arguments are the text; like pyfiglet, join them with a single space. The text may contain any Unicode characters.

Standard output must be **byte for byte** identical to what pyfiglet at this commit writes to standard output for the same arguments, including every space and newline, for any combination of the options above and any font in `pyfiglet/fonts-standard/`.

## Fonts

You may move the `.flf` files anywhere inside `/workspace/repo` (or embed them in the binary), but they must stay inside the repository. `/tmp/figlet` must work when started from any working directory.

## Hard rules

- Go standard library only (no third-party modules).
- When you finish, there must be no `.py` files anywhere under `/workspace/repo`.
- `go build ./...` in `/workspace/repo` must succeed.

## Not checked

Speed, memory use, exit codes, standard error, error messages, inputs on which pyfiglet itself fails or never finishes, and the options `-c`, `-L`, `-l`, `-i`.

## Grading

Your program is graded automatically on hidden inputs. The score is partial: a successful build, the absence of Python files, and the share of inputs whose output matches exactly all count.
