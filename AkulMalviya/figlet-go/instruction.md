
# Task: Build a Go Version of FIGlet

## Repository

pyfiglet is a Python FIGlet renderer that converts text into large ASCII-art lettering. This checkout is pinned to commit `255827efbc09d662875dad50fa0edc1e1263bf45` and includes the standard FIGlet fonts.

## What to Build

Create a Go program in `/workspace/repo` that reproduces the Python pyfiglet command-line output exactly. Run the build command below from `/workspace/repo`.

## Required Command-Line Interface

Build the program using:

`go build -o /tmp/figlet .`

The resulting executable must support:

- `-f FONT`: Select a font.
- `-w COLS`: Set the output width.
- `-j auto|left|center|right`: Set text justification.
- `-D auto|left-to-right|right-to-left`: Set text direction.
- `-r`: Reverse the output.
- `-F`: Flip the output.
- `-s`: Strip surrounding newlines.
- `-n`: Normalize surrounding newlines.
- Remaining command-line arguments: Text to render.

`-w` sets both the wrapping width and the width used for justification; center justification uses this same value.

## Correctness Requirements

- Output must match the Python pyfiglet implementation byte for byte.
- The `.flf` fonts may be moved, but must remain inside the repository. The program must find them when launched from any current working directory.
- Use only the Go standard library.
- Do not leave any `.py` files anywhere in the repository, and do not require Python at runtime.

## Build Requirements

The following command must succeed:

`go build ./...`

The program must be a genuine Go implementation, not a wrapper around Python or another external FIGlet executable.

## Not Checked

Speed, memory use, and error-message wording are not checked. The options `-c`, `-L`, `-l`, and `-i` are outside the required scope.

## Grading

Grading uses hidden automatic cases and awards partial scores.
