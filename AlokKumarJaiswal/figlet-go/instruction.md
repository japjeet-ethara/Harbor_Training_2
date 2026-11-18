# FIGlet in Go

## 1. Repository
`/workspace/repo` contains the MIT-licensed pyfiglet reference source at commit `255827efbc09d662875dad50fa0edc1e1263bf45`.
It renders text as FIGlet art. The pinned implementation defines behavior.

## 2. Task
Replace the Python implementation with a Go command-line program.
Place `go.mod` and the `main` package at `/workspace/repo`.
Running this command from `/workspace/repo` must build the executable:
```bash
go build -o /tmp/figlet .
```

Support the following options, followed by the text:
- `-f FONT`: Font to render with (default: `standard`).
- `-w COLS`: Terminal width in columns (default: `80`). Controls both wrapping and justification width.
- `-j auto|left|center|right`: Text justification (default: `auto`).
When `auto`, use left justification for resolved left-to-right direction
and right justification for resolved right-to-left direction.
An explicit justification overrides that automatic choice.
- `-D auto|left-to-right|right-to-left`: Text print direction (default: `auto`). In `auto` mode, direction is determined by the font header.
- `-r`: Horizontally mirror the rendered output, translating mirror
characters and reversing character order within each line
(default: disabled).
- `-F`: Flip rendered output text over (default: disabled). Vertically mirrors characters and reverses line order.
- `-s`: Remove leading whitespace-only rows and trailing whitespace
from the rendered output (default: disabled).
The CLI still writes its final newline.
- `-n`: Apply the same trimming as `-s`, then add a newline before
and after the trimmed rendered output (default: disabled).
The CLI additionally writes its final newline.

For supported invocations, stdout must match the original pyfiglet CLI byte for byte, including spaces, blank lines and final newlines.
Match option defaults and interactions from the pinned implementation:
- If both `-s` and `-n` are specified, `-s` takes precedence.
- Output transformations apply in the following order: rendering/justification, reverse (`-r`), flip (`-F`), and newline stripping/normalization (`-s`/`-n`).

## 3. Fonts
Use the supplied fonts from the standard-font collection.
The `.flf` files may be relocated but must remain within the repository.
The executable must locate its fonts and work from any working directory.

## 4. Hard rules
- Use only the Go standard library.
- No `.py` files may remain anywhere under `/workspace/repo` when finished.
- `go build ./...` must pass from `/workspace/repo`.

## 5. Not scored
- Speed, memory use and error-message wording are not scored.
- Options `-c`, `-L`, `-l` and `-i` are outside scope.

## 6. Grading
Hidden automatic checks award partial credit for build, removal of Python source files and exact rendering behavior.
