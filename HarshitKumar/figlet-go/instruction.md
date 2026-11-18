# What the repository is

The repository at `/workspace/repo` is the Python library pyfiglet. It renders short strings of text as giant ASCII art letters using FIGlet fonts, applying text justification, word wrapping, and directional transformations.

# What to build

Build a Go program in `/workspace/repo` with a `go.mod` and a `main` package at the repository root. Running `go build -o /tmp/figlet .` from `/workspace/repo` must produce a compiled command-line executable at `/tmp/figlet`.

The executable `/tmp/figlet` must accept the following command-line flags followed by the text argument(s). Several text arguments are joined with a single space.
- `-f FONT`: font name to use (defaults to `standard`).
- `-w COLS`: width used for wrapping and justification (integer, default `80`).
- `-j auto|left|center|right`: line justification (`auto`, `left`, `center`, `right`).
- `-D auto|left-to-right|right-to-left`: print direction (`auto`, `left-to-right`, `right-to-left`).
- `-r`: mirror image of the rendered text.
- `-F`: flip the rendered output upside down.
- `-s`: remove empty lines at the start and end of the output.
- `-n`: make the output have exactly one empty line before and after.

If both `-s` and `-n` are given, `-s` takes effect.

When executed, `/tmp/figlet` must print the exact rendered output to stdout, byte for byte identical to pyfiglet. The fonts draw the printable ASCII characters and the letters Ä Ö Ü ä ö ü ß; any other character (for example é, ï or ñ) is skipped and draws nothing, and text made only of such characters produces the same output as pyfiglet does for it.

# Fonts

The `.flf` font files currently located in `pyfiglet/fonts/` may be relocated or embedded, but they must remain within the repository. The resulting `/tmp/figlet` binary must function correctly regardless of the current working directory from which it is invoked.

# Hard rules

- Use only the Go standard library. No third-party modules or external packages are permitted.
- No `.py` files may remain anywhere in the repository when you finish.
- `go build ./...` must succeed without errors.

# Not checked

- Speed, memory usage, and error message wording are not checked.
- The pyfiglet options `-c` (color), `-L` (font install), `-l` (font list), and `-i` (font info) are not checked.

# How it is graded

Hidden automatic test cases will execute `/tmp/figlet` across diverse fonts, text strings, and flag combinations to assign a partial score.
