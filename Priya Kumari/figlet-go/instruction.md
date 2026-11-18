# FIGlet in Go

## 1. What the Repository Is

This repository is a Go reimplementation of [pyfiglet](https://github.com/pwaller/pyfiglet) (commit `255827efbc09d662875dad50fa0edc1e1263bf45`, 2026-08-02), a Python library and command-line tool for rendering text as large ASCII art banners using FIGlet fonts. Pyfiglet loads `.flf` (and `.tlf`) font files, parses their headers and character glyphs, applies configurable smushing rules (equal-character, underscore, hierarchy, opposite-pair, big-X, and hardblank smushing, plus kerning and universal overlap), and produces justified, optionally reversed or flipped, multi-line ASCII output. The original project is MIT-licensed (copyright Christopher Jones, Stefano Rivera, Peter Waller, and contributors).

## 2. What to Build

Build a Go program in `/workspace/repo`. Running the following command in `/workspace/repo` must produce a working binary:

```
go build -o /tmp/figlet .
```

The binary must accept the same command-line options as pyfiglet:

| Flag | Argument | Description |
|------|----------|-------------|
| `-f` | `FONT` | Font name (default: `standard`). |
| `-w` | `COLS` | Output width in columns (default: `80`). |
| `-j` | `auto\|left\|center\|right` | Text justification. `auto` infers from print direction. |
| `-D` | `auto\|left-to-right\|right-to-left` | Text rendering direction. `auto` uses the font's built-in value. |
| `-r` | *(none)* | Reverse (mirror) the output horizontally, translating symmetric characters. |
| `-F` | *(none)* | Flip the output vertically (upside-down), translating vertical-symmetric characters. |
| `-s` | *(none)* | Strip surrounding empty lines. Takes priority over `-n`. |
| `-n` | *(none)* | Normalize surrounding newlines (exactly one blank line before and after). Only applied when `-s` is not set. |

Remaining non-flag arguments are the input text.

When `-w COLS` is specified, justification (`-j`) must use that selected output width. For example, `-w 40 -j center` means center the output within 40 columns.

**The output must be byte-for-byte identical to pyfiglet's output for the same inputs.**

## 3. Fonts

The repository contains `.flf` font files. These files may be relocated within the repository, but they must remain inside it. The built program must locate and load fonts correctly regardless of the working directory from which it is invoked.

## 4. Hard Rules

- **Go standard library only.** No third-party modules.
- **No `.py` files anywhere in the repository.** Not in any directory, at any depth.
- **`go build ./...` must pass** with no errors from the repository root.

## 5. Not Checked

The following are **not evaluated**:

- Execution speed
- Memory usage
- Error message wording or formatting
- The options `-c`, `-L`, `-l`, `-i`

## 6. Grading

Submissions are evaluated by hidden automated cases. Partial credit is awarded.
