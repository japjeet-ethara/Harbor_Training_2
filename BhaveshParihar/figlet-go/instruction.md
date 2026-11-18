# FIGlet Go task

This repository is a small Harbor task based on the pyfiglet project. The aim is to build a Go program in `/workspace/repo` that produces the same giant-letter text as pyfiglet for the same command line, using the same font files and the same smushing/kerning rules.

The required artifact is a command-line program that can be built with `go build -o /tmp/figlet .` from `/workspace/repo` and then run as `/tmp/figlet [options] [text..]`. It must support these options exactly: `-f FONT`, `-w COLS`, `-j auto|left|center|right`, `-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed by the input text.

The `.flf` files may be moved or copied so long as they remain inside the repository and the binary can load them from any working directory. The program must not assume the current directory is the repository root.

Hard rules:

- Only the Go standard library is allowed.
- Do not keep any `.py` files anywhere under `/workspace/repo`.
- `go build ./...` must succeed.
- Output must match pyfiglet byte for byte for the grading inputs.

Not part of the grading: speed, memory use, exact wording of errors, and the options `-c`, `-L`, `-l`, and `-i`.

The hidden grading uses automatic cases and partial credit: build correctness, no-Python enforcement, and behaviour parity against the original pyfiglet output.
