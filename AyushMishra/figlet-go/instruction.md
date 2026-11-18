# Harbor task instructions

## 1. Repository

The upstream repository is `pwaller/pyfiglet`, an MIT-licensed pure-Python implementation of FIGlet. As recorded in `C:\Users\AyushMishra\Documents\pyfiglet\NOTES.md`, runtime font loading uses the `pyfiglet.fonts` package; standard font definitions are maintained in `pyfiglet/fonts-standard`, and `make minimal` copies them into `pyfiglet/fonts` before building. The renderer supports FIGlet font behavior such as smushing, kerning, text direction, justification, and output transforms.

## 2. What to build

Build a Go command-line program in `/workspace/repo`. Running `go build -o /tmp/figlet .` from `/workspace/repo` must produce a program that accepts these options, followed by the text:

- `-f FONT`
- `-w COLS`
- `-j auto|left|center|right`
- `-D auto|left-to-right|right-to-left`
- `-r`
- `-F`
- `-s`
- `-n`

For supported inputs, the program's output must match pyfiglet byte for byte.

## 3. Fonts

The `.flf` files may be moved within the repository, but must remain in the repository. The program must locate and use them correctly no matter what the current working directory is.

## 4. Hard rules

- Use only the Go standard library.
- Do not leave any `.py` files anywhere in the repository.
- `go build ./...` must pass.

## 5. Not checked

Speed, memory use, error messages, and the `-c`, `-L`, `-l`, and `-i` options are not checked.

## 6. Grading

Grading uses hidden automatic cases and awards partial scores.