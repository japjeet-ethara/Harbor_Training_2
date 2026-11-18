# figlet-go

## The repository

The repository at `/workspace/repo` is a checkout of [pyfiglet](https://github.com/pwaller/pyfiglet) at commit `255827efbc09d662875dad50fa0edc1e1263bf45`, a Python FIGlet implementation released under the MIT licence. Its font setup step is already done: the font files from `pyfiglet/fonts-standard/` were copied into `pyfiglet/fonts/`. There is no Python in this environment, so you cannot run pyfiglet here; its source is in `pyfiglet/__init__.py`.

## What to build

Write a Go program whose main package lives at the repository root (`/workspace/repo`), so that `go build -o /tmp/figlet .`, run in that folder, builds it.

The binary must accept these flags and positional arguments:

| Flag | Argument | Default | Meaning |
|------|----------|---------|---------|
| `-f` | FONT | `standard` | Font name (basename without `.flf`). |
| `-w` | COLS | `80` | Output width in columns, used for wrapping and justification. |
| `-j` | JUSTIFY | `auto` | Justification: `auto`, `left`, `center`, or `right`. |
| `-D` | DIRECTION | `auto` | Print direction: `auto`, `left-to-right`, or `right-to-left`. |
| `-r` | *(none)* | off | Mirror the output left to right; characters such as `/` and `\` or `(` and `)` are swapped too. |
| `-F` | *(none)* | off | Flip the output upside down. |
| `-s` | *(none)* | off | Remove empty lines before and after the drawing, and trailing whitespace at the very end. |
| `-n` | *(none)* | off | Do what `-s` does, then add exactly one empty line before and one empty line after the drawing. Ignored when `-s` is given. |

When several of these are given, they apply in this order: `-r`, then `-F`, then `-s` or `-n`.

All flags come before the positional text. Every remaining argument after the flags is joined with a single space to form the input text.

**The standard output of the Go program must be byte-for-byte identical to what `python -m pyfiglet` from this repository prints for the same arguments**, including the final newline. That is the only correctness criterion.

## Fonts

Every `.flf` font file in `pyfiglet/fonts/` must be supported, referenced by its basename (e.g. `standard`, `slant`). You may move or embed the `.flf` files however you like, but they must remain in the repository (not downloaded at runtime). The built binary must work regardless of the working directory it is run from.

## Hard rules

1. **Go standard library only.** `go.mod` must not contain any `require` lines. No third-party modules.
2. **`go.mod` must declare `go 1.23` or lower.** The environment has Go 1.23 with `GOTOOLCHAIN=local`; a higher version will not build.
3. **`go build ./...` must succeed** with no errors from anywhere in the module.
4. **No `.py` files may remain anywhere in the repository**, including `pyfiglet/fonts/__init__.py`. Read the Python source first if you need it, then delete every Python file before you are done.

## What is not checked

The following are **not tested** and you should not spend time on them:

- Speed or memory usage.
- Error messages or exit codes for bad input.
- The flags `-c`, `-L`, `-l`, and `-i`.
- Any input on which `pyfiglet` itself crashes, hangs, or never finishes (for example, a single character whose glyph is wider than the `-w` width).

## Grading

Your output will be graded by hidden automated cases. Partial credit is awarded. Do your best to match `pyfiglet`'s output exactly on every valid input.
