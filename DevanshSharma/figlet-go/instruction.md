# figlet-go — Task Instruction

## What the repository is

The repository at `/workspace/repo` contains **pyfiglet** — a Python implementation of FIGlet, a program that renders short text strings as large ASCII-art letters made of slashes, underscores, and pipes. The `.flf` font files that define each letter's shape live in `pyfiglet/fonts/`. The rendering engine reads a font file, then for each pair of adjacent rendered characters it "smushes" (squeezes) their touching columns together using up to six rules, controlled by a bitmask in the font header.

## What to build

Write a Go program whose source lives in `/workspace/repo`. Running

```
go build -o /tmp/figlet .
```

from `/workspace/repo` must succeed and produce an executable `/tmp/figlet`.

The program accepts the following command-line options followed by the text to render:

| Flag | Argument | Default | Meaning |
|------|----------|---------|---------|
| `-f` | FONT | `standard` | Font name. The `.flf` file must be found inside the repository. |
| `-w` | COLS | `80` | Terminal width in columns. Controls line-wrapping and justification padding. |
| `-j` | `auto`\|`left`\|`center`\|`right` | `auto` | Justification. `auto` means left for left-to-right fonts, right for right-to-left fonts. |
| `-D` | `auto`\|`left-to-right`\|`right-to-left` | `auto` | Print direction. `auto` reads from the font header. |
| `-r` | _(flag)_ | off | Mirror the output horizontally (each row reversed, `/`↔`\`, `(`↔`)`, etc.). Applied after rendering. |
| `-F` | _(flag)_ | off | Flip the output vertically (rows reversed, `^`↔`v`, etc.). Applied after `-r`. |
| `-s` | _(flag)_ | off | Strip all leading and trailing blank lines from the output. |
| `-n` | _(flag)_ | off | Normalize to exactly one blank line before and one after the output. Applied only if `-s` is not set. |

The program must print **exactly** the same bytes to stdout as `python -m pyfiglet` with the same arguments.

## Fonts

The `.flf` font files must remain inside the repository directory tree. The program must locate them correctly regardless of the current working directory when it is invoked (use `go:embed` or compute paths relative to the executable).

## Hard rules

- Use the **Go standard library only**. No third-party packages.
- There must be **no `.py` files** anywhere under `/workspace/repo` when grading runs.
- `go build ./...` run from `/workspace/repo` must pass with exit code 0.

## Not checked

Speed, memory usage, error messages for bad input, and the options `-c`, `-L`, `-l`, `-i` are not tested.

## Grading

Grading is automatic and uses hidden test cases. The score is partial — each correct case adds to the total. Build must pass to earn any behaviour points.
