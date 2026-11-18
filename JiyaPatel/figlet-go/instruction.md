# Port pyfiglet to Go

## Repository
This repository (`/workspace/repo`) contains a Python implementation of FIGlet (`pyfiglet`), which renders input text into large ASCII-art banners using `.flf` font files located in `pyfiglet/fonts/`. It supports horizontal kerning and six smushing rules (equal character, underscore, hierarchy, opposite pair, big X, and hardblank), word-boundary line wrapping, horizontal justification, print direction, mirroring, vertical flipping, and surrounding newline handling.

## Goal
Build a Go command-line program in `/workspace/repo` that replicates `pyfiglet`'s exact ASCII-art rendering output, byte for byte.

- Running `go build -o /tmp/figlet .` inside `/workspace/repo` must produce an executable that accepts the following flags followed by the text arguments to render:
  - `-f FONT`: Font name (default: `standard`).
  - `-w COLS`: Maximum output width in columns (default: `80`).
  - `-j auto|left|center|right`: Horizontal justification within the specified width `COLS` (default: `auto`).
  - `-D auto|left-to-right|right-to-left`: Print direction (default: `auto`).
  - `-r`: Show mirror image of output text (`--reverse`).
  - `-F`: Flip rendered output text vertically (`--flip`).
  - `-s`: Strip empty leading and trailing lines (`--strip-surrounding-newlines`).
  - `-n`: Normalize surrounding newlines so output has one empty line before and after (`--normalize-surrounding-newlines`).
- The program must print the exact same output to `stdout` as `pyfiglet`, byte for byte, including trailing spaces and newlines.

## Fonts
- All `.flf` font files must remain inside the repository (or be embedded into the Go binary), though their directory location inside the repository may change.
- The compiled binary must work regardless of the current working directory from which it is invoked.

## Hard Rules
- Use only the Go standard library (`go.mod` must have zero external dependencies).
- Delete all `.py` files from `/workspace/repo` before finishing (`find /workspace/repo -name '*.py'` must return nothing).
- `go build ./...` must succeed cleanly.

## Out of Scope
- Execution speed, memory usage, and error message formatting are not checked.
- The flags `-c`, `-L`, `-l`, and `-i` are not required and will not be tested.

## Grading
Your submission is evaluated automatically against a suite of hidden test cases with partial credit awarded based on the proportion of exact byte-for-byte matches.