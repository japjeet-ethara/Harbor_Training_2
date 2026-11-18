# FIGlet in Go

## 1. Repository Background
This repository is based on `pyfiglet`, a pure-Python implementation of FIGlet that renders text into ASCII art banners using FIGlet font files. The program parses `.flf` fonts, computes glyph bounding boxes, applies horizontal kerning and controlled smushing rules, wraps long lines according to terminal width, and handles text justification, direction, and character flipping.

## 2. What to Build
Build a Go program in `/workspace/repo`. Running:
```sh
go build -o /tmp/figlet .
```
in `/workspace/repo` must produce a command-line program that accepts the same options as pyfiglet:
- `-f FONT`
- `-w COLS`
- `-j auto|left|center|right` (justification operates within the width specified by `-w COLS`)
- `-D auto|left-to-right|right-to-left`
- `-r`
- `-F`
- `-s`
- `-n`

followed by the text. It must print the same output as pyfiglet, byte for byte.

## 3. Fonts
The `.flf` files may move, but must stay in the repository. The program must work from any working directory.

## 4. Hard Rules
- Go standard library only.
- No `.py` files anywhere in the repository.
- `go build ./...` must pass.

## 5. What Is Not Checked
Speed, memory, error messages, and the options `-c`, `-L`, `-l`, `-i` are not checked.

## 6. Grading
Grading uses hidden automatic evaluation, and partial credit is possible.
