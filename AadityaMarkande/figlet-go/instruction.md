# Figlet in Go

This repository is pyfiglet, a Python port of FIGlet, from https://github.com/pwaller/pyfiglet at commit 255827efbc09d662875dad50fa0edc1e1263bf45 (2026-08-02T22:51:18+01:00), package version 1.0.4, under the MIT License. It reads FIGlet fonts and renders text with that program's horizontal smushing, kerning, wrapping, justification, and print direction.

## What to build

Build a Go program in `/workspace/repo`. `go build -o /tmp/figlet .`, run in `/workspace/repo`, must produce a command-line program that accepts the same options as pyfiglet, followed by the text:

- `-f FONT` (`--font`)
- `-w COLS` (`--width`)
- `-j auto|left|center|right` (`--justify`)
- `-D auto|left-to-right|right-to-left` (`--direction`)
- `-r` (`--reverse`)
- `-F` (`--flip`)
- `-s` (`--strip-surrounding-newlines`)
- `-n` (`--normalize-surrounding-newlines`)

The remaining arguments are the text, joined with a single space, as pyfiglet joins them. The program must print the same output as pyfiglet, byte for byte.

The default font is `standard`. The default width is `80`. `-D auto` reads the font header's print-direction field: `0` means left-to-right, `1` means right-to-left, and any other value, including a missing field, means left-to-right. `-j auto` follows that resolved direction: left-to-right justifies left, and right-to-left justifies right.

## Fonts

The `.flf` files may be moved, but they must stay inside the repository. The program must work from any working directory.

## Hard rules

- Use the Go standard library only.
- The repository must contain no `.py` files anywhere.
- `go build ./...` must pass.

## Not checked

Speed, memory, error messages, and the options `-c`, `-L`, `-l`, and `-i` are not checked.

## Grading

Grading is automatic and can award a partial score.
