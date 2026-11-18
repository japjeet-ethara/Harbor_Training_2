# Port pyfiglet to Go

## 1. The repository

`/workspace/repo` is a snapshot of **pyfiglet**, a pure-Python port of FIGlet. FIGlet renders text as large
ASCII-art letters. All of the program logic is in `pyfiglet/__init__.py`:
- **Font loading:** `FigletFont` parses the `.flf` font files.
- **Layout:** `FigletBuilder` handles kerning, the six smushing rules, word-wrapping and justification.
- **Transformations:** `FigletString` does reverse, flip and blank-line trimming.
- **Command line:** `main()` is the front end.

The fonts are the `.flf` files in `pyfiglet/fonts/`. There is no Python interpreter in this environment, so
treat the pyfiglet source as the specification.

## 2. What to build

Write a Go program in `/workspace/repo` that builds with:

```sh
cd /workspace/repo && go build -o /tmp/figlet .
```

It is called as `/tmp/figlet [options] text...`. Each option is a separate argument, and all options come
before the text. If there are several text arguments, they are joined with single spaces.

| Option | Meaning | Default |
|--------|---------|---------|
| `-f FONT` | Font name without `.flf`. Must accept every `.flf` font in `pyfiglet/fonts/`. | `standard` |
| `-w COLS` | Output width. Used for both word-wrapping and justification. | `80` |
| `-j auto\|left\|center\|right` | Justification within the `-w` width. `auto` means `right` when the direction is right-to-left, otherwise `left`. | `auto` |
| `-D auto\|left-to-right\|right-to-left` | Print direction. `auto` uses the direction field in the font header. | `auto` |
| `-r` | Mirror image. | off |
| `-F` | Upside down. | off |
| `-s` | Remove blank lines at the top and bottom. | off |
| `-n` | Exactly one blank line at the top and at the bottom. If `-s` is also given, `-s` wins. | off |

`-j center` and `-j right` always align within the `-w` value, not within 80 columns. So `-w 40 -j center`
centres the text in 40 columns. The padding is calculated exactly as in pyfiglet's
`justifyString`.

For any combination of these options, stdout must be **byte-for-byte identical** to pyfiglet's output. That
includes spaces at line ends and the final newline that pyfiglet's `main()` adds.

## 3. Fonts

- You may move the `.flf` files, but they must stay inside `/workspace/repo`.
- The program must give the same output whatever directory it is run from.

## 4. Hard rules

- **Standard library only:** Go standard library only. `go.mod` has no `require` lines.
- **No `.py` files:** no `.py` files anywhere in `/workspace/repo`, including the original pyfiglet files.
- **Build:** `go build ./...` succeeds in `/workspace/repo`.

## 5. Not checked

- **Not measured:** speed and memory use.
- **Errors:** error messages and exit codes.
- **Options you don't need:** `-c`, `-L`, `-l` and `-i`.

## 6. Grading

The program is run automatically on cases you cannot see. Each case is scored as a byte-for-byte match with
pyfiglet's output, and your score is the share of cases that match.
