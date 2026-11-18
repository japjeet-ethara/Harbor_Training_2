# Go FIGlet compatibility port

The repository contains pyfiglet, a Python implementation of the FIGlet text-rendering format, together with its standard FIGlet fonts. The source is pinned to commit `255827efbc09d662875dad50fa0edc1e1263bf45`; its font setup copies `fonts-standard/` into the runtime `fonts/` directory.

Build a Go command-line program in `/workspace/repo`. Running `go build -o /tmp/figlet .` from `/workspace/repo` must produce a program accepting `-f FONT`, `-w COLS`, `-j auto|left|center|right`, `-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed by text arguments. The program must match pyfiglet's standard-output bytes exactly for the supported options and fonts.

The FIGfont files may be moved or embedded, but they must remain inside the repository. The program must locate its fonts regardless of the caller's current working directory.

Use only the Go standard library. No `.py` files may remain anywhere in the repository. `go build ./...` must succeed.

Speed, memory use, error-message wording, and the options `-c`, `-L`, `-l`, and `-i` are not evaluated.

The task is graded using automatic cases with partial credit.
