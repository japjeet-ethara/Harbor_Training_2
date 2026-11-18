# Port pyfiglet to Go

## 1. The repository

`/workspace/repo` contains **pyfiglet** (commit `255827efbc09d662875dad50fa0edc1e1263bf45`, MIT licence), the Python version of FIGlet. It takes a short text and draws it in large letters made of ordinary characters, using font files (`.flf`). Neighbouring letters are pushed together ("kerning") or overlapped ("smushing") according to rules stored in each font's header. Long text is wrapped to a width, and the output can be justified, written right-to-left, mirrored and flipped. All of the logic is in `pyfiglet/__init__.py` (`FigletFont`, `FigletString`, `FigletBuilder`, `Figlet`, `main`). The fonts are in `pyfiglet/fonts-standard/` (a copy is also in `pyfiglet/fonts/`). There is no Python interpreter in this environment: read the source to understand it.

## 2. What to build

A Go program in `/workspace/repo`. Running

```
cd /workspace/repo && go build -o /tmp/figlet .
```

must produce a command-line program, `/tmp/figlet`, that behaves like `python -m pyfiglet`. Its **standard output must be byte for byte identical** to what pyfiglet writes to standard output for the same arguments, including every space, every trailing space and the final newline(s).

Arguments are given in this form:

```
/tmp/figlet [options] TEXT [TEXT ...]
```

* Every option comes before the text. Each option is a separate argument, and an option's value is the next argument (for example `-w` `40`, never `-w40` or `--width=40`).
* The text is all remaining arguments joined with a single space, exactly as pyfiglet does. It may contain any Unicode characters, including spaces, newlines and characters the font does not have.

Options that must be supported, with the same meaning and defaults as pyfiglet:

| Option | Meaning |
|---|---|
| `-f FONT` | font name, for example `slant` (default `standard`) |
| `-w COLS` | width used for wrapping **and** for justification (default `80`). When `-w` is given, `-j center` and `-j right` align within COLS, exactly as pyfiglet does |
| `-j auto\|left\|center\|right` | justification (default `auto`) |
| `-D auto\|left-to-right\|right-to-left` | direction (default `auto`, which takes the direction from the font file) |
| `-r` | mirror the output |
| `-F` | flip the output upside down |
| `-s` | strip surrounding blank lines |
| `-n` | normalize to exactly one blank line before and after |

Several of the flags may be combined in one call, and pyfiglet's order of applying them must be kept.

Only inputs for which pyfiglet finishes normally and prints its result are graded. Inputs that make pyfiglet crash (for example a character wider than COLS) or run forever are not graded.

## 3. Fonts

* Every font in `pyfiglet/fonts-standard/` with the `.flf` extension must be usable by its name (the file name without `.flf`).
* The `.flf` files may be moved or renamed, or embedded with `go:embed`, but the fonts must stay inside `/workspace/repo`.
* The program must work from any working directory. Do not read fonts from a path relative to the current directory.

## 4. Hard rules

* Go standard library only: no third-party modules. The build is done offline with an empty module cache.
* The installed toolchain is Go 1.23 (`GOTOOLCHAIN=local`), so `go.mod` must not ask for a newer Go version.
* When you are finished, there must be **no `.py` file anywhere under `/workspace/repo`**.
* `cd /workspace/repo && go build ./...` must succeed.

## 5. Not checked

Speed (within a few seconds per call), memory use, exit codes, anything written to standard error, error messages for bad input, and the options `-c`, `-L`, `-l`, `-i`, `--version` and `--help`.

## 6. Grading

Your program is run automatically on many inputs you have not seen, covering all the options above, several fonts, narrow and wide widths, and unusual characters. The score is partial: it combines whether the program builds, whether the Python files are gone, and the share of inputs whose output is exactly right.
