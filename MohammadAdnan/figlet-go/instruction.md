# Port pyfiglet to Go

## 1. The repository

`/workspace/repo` contains pyfiglet (https://github.com/pwaller/pyfiglet, MIT licence) at commit
`255827efbc09d662875dad50fa0edc1e1263bf45`, with its font setup already done: the fonts from
`pyfiglet/fonts-standard/` are copied into `pyfiglet/fonts/`. pyfiglet is a Python
implementation of FIGlet. It draws text in large letters made of ordinary characters, using
`.flf` font files and the FIGlet layout rules (kerning, smushing, wrapping, justification,
print direction). The rendering logic is in `pyfiglet/__init__.py`, in the `FigletFont`,
`FigletBuilder`, `FigletString` and `Figlet` classes and in `main()`. Python is not installed
in this environment, so you cannot run the original; read the source instead.

## 2. What to build

Replace pyfiglet with a Go program in `/workspace/repo`. Running

```
cd /workspace/repo && go build -o /tmp/figlet .
```

must produce a command-line program that behaves like `python -m pyfiglet` from this commit.
It must accept these options:

| Option | Meaning |
|---|---|
| `-f FONT` | font name, default `standard` |
| `-w COLS` | width for wrapping and justification, default `80` |
| `-j auto\|left\|center\|right` | justification, default `auto` |
| `-D auto\|left-to-right\|right-to-left` | print direction, default `auto` |
| `-r` | mirror the output |
| `-F` | flip the output upside down |
| `-s` | strip surrounding empty lines |
| `-n` | normalize surrounding empty lines |

Each option is its own argument, and an option's value is the next argument (for example
`-w 40 -j center`). All options come before the text. Every remaining argument is text. The
words of the text are joined with single spaces, exactly as pyfiglet does, and no text word
starts with `-`.

For every such command line, the program must write to standard output exactly the bytes
pyfiglet writes (UTF-8), including the final newline. This holds for every combination of the
options above, including `-w` widths from 10 to 120 together with every `-j` and `-D` value.
`center` and `right` justification use the `-w` width. Text may contain spaces (including
double spaces), punctuation such as `| / \ _ [ ] { } ( ) < >`, Latin-1 letters such as `é ß ñ`,
and characters the font does not define.

The fonts that must work are `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`
and `script`.

## 3. Fonts

You may move the `.flf` files, or embed them with `go:embed`, but they must stay inside
`/workspace/repo`. `/tmp/figlet` must give the same output whatever directory it is run from.

## 4. Hard rules

- Go standard library only: `go.mod` must have no `require` lines.
- No `.py` file may remain anywhere under `/workspace/repo`.
- `go build ./...` run in `/workspace/repo` must succeed.

## 5. Not checked

Speed, memory use, error messages and exit codes, long option names (`--font` and so on),
the options `-c`, `-L`, `-l`, `-i`, and inputs for which pyfiglet itself fails or never
finishes. The latter happens, for example, when a single letter is as wide as `-w` or wider.

## 6. Grading

Your work is graded automatically on hidden command lines of the kind described above, with
partial credit.
