# figlet-go — Task Instruction

## What the repository is

This repository contains **pyfiglet** — a Python implementation of the classic FIGlet program. FIGlet converts plain text into large ASCII-art letters using `.flf` font files. The rendering engine supports six smushing rules that determine how adjacent letters overlap and squeeze together (equal-char merge, underscore absorption, hierarchy override, opposite-pair merge, big-X crossing, and hardblank merge), plus kerning mode. The library also supports text wrapping, four justification modes (auto, left, center, right), two print directions (left-to-right, right-to-left), horizontal reversal (`-r`), vertical flip (`-F`), and two newline-normalisation modes (`-s`, `-n`).

## What to build

Write a Go program whose source lives in `/workspace/repo`. Running:

```
go build -o /tmp/figlet .
```

from inside `/workspace/repo` must produce a working binary. The binary must accept the same command-line options as pyfiglet and print output that is **byte-for-byte identical** to pyfiglet for every input:

```
/tmp/figlet [OPTIONS] TEXT...
```

| Option | Argument | Meaning |
|--------|----------|---------|
| `-f`   | `FONT`   | Font name (without `.flf`). Default: `standard`. |
| `-w`   | `COLS`   | Terminal width for wrapping and justification. Default: `80`. |
| `-j`   | `auto\|left\|center\|right` | Justification. `auto` follows print direction. |
| `-D`   | `auto\|left-to-right\|right-to-left` | Print direction. `auto` reads from font header. |
| `-r`   | *(flag)* | Mirror each output row (reverse characters and swap `/`↔`\`, `(`↔`)`, etc.). |
| `-F`   | *(flag)* | Flip output upside down (reverse row order and apply vertical mirror map). |
| `-s`   | *(flag)* | Strip empty leading and trailing lines from output. |
| `-n`   | *(flag)* | Normalise surrounding newlines (exactly one blank line before and after). |

Multiple words after the options are joined with a single space, exactly as pyfiglet does.

## Fonts

The `.flf` font files must remain inside the repository. The binary must locate them correctly regardless of the current working directory (use `go:embed` or store an absolute path relative to the binary).

Only the fonts in `pyfiglet/fonts/` are used (the `fonts-standard` set, already copied there). Do not add or remove font files.

## Hard rules

- **Go standard library only.** No third-party modules.
- **No `.py` files** anywhere under `/workspace/repo` when the grader runs.
- `go build ./...` must pass with zero errors.

## Not checked

Speed, memory usage, error messages, and the options `-c` (colour), `-L` (install font), `-l` (list fonts), `-i` (font info) are not tested.

## Grading

Your solution is graded automatically against hidden test cases. Grading is partial — a partly correct solution scores between 0 and 1.
