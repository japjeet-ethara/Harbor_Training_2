# FIGlet in Go

You are working in `/workspace/repo`. Your task is to build a pure-Go implementation of FIGlet, matching the behavior of the original pyfiglet project.

## 1. Build

Create a command-line Go program that builds with:

`go build -o /tmp/figlet .`

The resulting binary must work regardless of the current working directory.

## 2. Command-line options

Support these options:

- `-f FONT`: Font name (default: `standard`).
- `-w COLS`: Terminal width (default: `80`).
- `-j SIDE`: Justification: `auto`, `left`, `center`, or `right`.
- `-D DIRECTION`: Direction: `auto`, `left-to-right`, or `right-to-left`.
- `-r`: Mirror the output horizontally.
- `-F`: Flip the output vertically.
- `-s`: Remove empty leading and trailing lines.
- `-n`: Add one empty line before and after the output. If `-s` is also specified, `-s` takes precedence.

Join multiple text arguments with single spaces. Handle non-ASCII characters and characters unsupported by the selected font consistently with pyfiglet.

Output must match pyfiglet byte for byte, including spaces and line breaks.

## 3. Fonts

Use the `.flf` font files provided in the repository. Fonts may be moved within the repository or embedded using `//go:embed`.

The following fonts must work: `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`, and `script`.

## 4. Requirements

- Use only the Go standard library. Do not add external dependencies.
- Remove all Python source files from the repository.
- Ensure `go build ./...` succeeds.
- Preserve the original FIGlet rendering behavior, including kerning, smushing, wrapping, justification, and direction handling.
