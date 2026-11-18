# Port pyfiglet to Go

The checkout in `/workspace/repo` contains pyfiglet, a Python implementation of FIGlet, and its prepared standard FIGlet fonts. It turns text into character art using font-specific spacing, glyph overlaps, and layout rules. Use the existing source and fonts as the behavioral reference.

Write a Go standard-library CLI in `/workspace/repo`. Running `go build -o /tmp/figlet .` there must produce the executable. It must accept `-f FONT`, `-w COLS`, `-j auto|left|center|right`, `-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed by the text. Match pyfiglet's stdout byte for byte, including spaces and newlines. The selected width controls both wrapping and justification. Preserve the reference's defaults and its behavior when options are combined.

Accept `--` as the option terminator. Join multiple text arguments with one space.

Keep the `.flf` fonts in the repository. You may move or embed them. The compiled executable must render from any working directory. The final repository must contain zero `.py` files, and `go build ./...` must pass.

Hidden automatic grading awards partial credit. Speed, memory, error wording, color, and font installation, listing, and information are outside the grading scope.
