# practice/figlet-go

A pure-Go implementation of FIGlet, based on the pinned pyfiglet source.

## Build

Run: `go build -o /tmp/figlet .`

## Usage

- `./figlet -f standard "Hello World"`
- `./figlet -f slant "Hello World"`
- `./figlet -w 40 -j center "Hello World"`

## Supported fonts

The required fonts include standard, slant, small, big, mini, banner, shadow, and script.

## Requirements

- Go 1.23
- Go standard library only

## Task files

- `instruction.md` — implementation requirements
- `environment/` — agent environment Dockerfile
- `tests/` — visible and hidden test data, grader, and verifier
- `solution/` — reference solution
- `NOTES.md` — analysis of the pinned pyfiglet source
