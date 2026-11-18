# Task: Port pyfiglet to Go

## 1. Repository

This repository contains pyfiglet, a Python implementation of FIGlet that renders text as large ASCII art using FIGlet font files. It includes font parsing, character layout, kerning, six smushing rules, text wrapping, justification, direction handling, and transformations such as reversing and flipping. The original source is available at https://github.com/pwaller/pyfiglet.

## 2. What to build

Implement a Go command-line program in `/workspace/repo` that reproduces the behaviour of the original pyfiglet command.

This command must work from `/workspace/repo`:

`go build -o /tmp/figlet .`

It must produce an executable at `/tmp/figlet`.

The program must accept the following options, followed by the text to render:

- `-f FONT`: Select the font. Default: `standard`.
- `-w COLS`: Set the output width in columns. Default: `80`.
- `-j auto|left|center|right`: Set text justification. Default: `auto`.
- `-D auto|left-to-right|right-to-left`: Set text direction. Default: `auto`.
- `-r`: Reverse the rendered output.
- `-F`: Flip the rendered output.
- `-s`: Strip surrounding newlines.
- `-n`: Normalize surrounding newlines.

Support these corresponding long option names:

- `--font`
- `--width`
- `--justify`
- `--direction`
- `--reverse`
- `--flip`
- `--strip-surrounding-newlines`
- `--normalize-surrounding-newlines`

The positional text arguments must be rendered as FIGlet ASCII art.

## 3. Exact compatibility

The output must match the original Python pyfiglet implementation byte for byte, including:

- Every character and space.
- Line wrapping.
- Justification and alignment.
- Direction and character ordering.
- Kerning and all six smushing rules.
- Reverse and flip transformations.
- Leading and trailing newline behaviour.

Justification must use the configured output width (`-w COLS`). Center alignment must center each rendered row within that width.

Preserve the original option defaults and interactions.

When both `-s` and `-n` are supplied, `-s` takes precedence.

Reproduce the original Python implementation's behaviour rather than approximating its appearance.

## 4. Fonts

Use the font files included in the repository.

The `.flf` and `.tlf` font files may be moved into another directory within the repository if necessary, but they must remain part of the repository.

The executable must locate the fonts correctly regardless of the current working directory from which it is launched.

Do not depend on fonts installed elsewhere on the machine.

## 5. Hard requirements

- Implement the program in Go.
- Use only the Go standard library.
- Do not use third-party Go packages.
- Do not use Python to implement or run the solution.
- Do not create or include any `.py` files anywhere in the repository.
- `go build ./...` must succeed.
- `go build -o /tmp/figlet .` must succeed from `/workspace/repo`.
- The program must work as a command-line executable.

## 6. Not checked

The following are outside the scope of this task:

- Execution speed.
- Memory usage.
- Exact error messages.
- Colour output (`-c`).
- Font installation (`-L`).
- Font listing (`-l`).
- Font information (`-i`).

Do not implement these out-of-scope options.

## 7. Evaluation

The implementation will be evaluated automatically, and partial scores are possible.

Implement the complete required behaviour rather than hardcoding specific inputs.