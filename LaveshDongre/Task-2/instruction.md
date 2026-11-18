# Task Instructions

## Overview
This repository contains `pyfiglet`, a pure-Python implementation of FIGlet, which transforms short strings of text into large ASCII art banners using FIGlet fonts (`.flf`). It supports layout modes such as full pitch, kerning, and six distinct horizontal smushing rules (equal characters, underscore, character hierarchy, opposite pairs, big X, and hardblank smushing), as well as multi-column wrapping, justification, text reversal, vertical flipping, and right-to-left formatting.

## Objective
Build a Go twin of the FIGlet rendering program in `/workspace/repo`. When running `go build -o /tmp/figlet .` inside `/workspace/repo`, it must produce a standalone command-line executable at `/tmp/figlet`.

The executable must accept the following command-line flags followed by positional text arguments:
- `-f FONT`: Name or path of the font to use (default: `standard`).
- `-w COLS`: Column width for wrapping and justification (default: `80`).
- `-j auto|left|center|right`: Justification mode (default: `auto`).
- `-D auto|left-to-right|right-to-left`: Text reading/rendering direction (default: `auto`).
- `-r`: Reverse/mirror output text horizontally.
- `-F`: Flip output text vertically.
- `-s`: Strip surrounding blank lines from the rendered output.
- `-n`: Normalize surrounding newlines to one blank line before and after the rendered banner.

The program must reproduce the exact output produced by `pyfiglet`, byte for byte, including trailing spaces and newlines.

## Fonts
The FIGlet font files (`.flf`) located in the repository may be organized or moved into a dedicated directory or embedded into the binary (e.g. using `go:embed`), but they must remain within the repository. The resulting `/tmp/figlet` binary must function identically when executed from any working directory.

## Hard Rules
- Use only the Go standard library (no third-party dependencies).
- No Python files (`.py`) may remain anywhere under `/workspace/repo`.
- `go build ./...` must succeed cleanly with zero errors.

## Not Checked
- Execution speed and memory efficiency.
- Error message wording on invalid arguments.
- Unused options: `-c`, `-L`, `-l`, `-i`.

## Grading
Your implementation will be evaluated automatically on hidden cases covering standard text, boundary characters, wrapping, smushing combinations, justification, and formatting options. Partial credit is awarded based on the proportion of correctly matched cases.
