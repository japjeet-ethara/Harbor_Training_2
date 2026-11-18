# Port Python pyfiglet to Go

## 1. What the Repository Is

`/workspace/repo` contains the Python `pyfiglet` package (MIT License), checked
out at commit `255827efbc09d662875dad50fa0edc1e1263bf45`. pyfiglet is a pure-Python
implementation of FIGlet: it renders text as large ASCII-art letters using
FIGfont (`.flf`) files.

The reference implementation is `pyfiglet/__init__.py`, mainly the
`FigletFont`, `FigletBuilder`, `FigletString` and `Figlet` classes and the
`main()` function. The FIGfont format, including layout modes and smushing
rules, is specified in `doc/figfont.txt`. The fonts are in `pyfiglet/fonts/`.

No Python interpreter is installed, so you cannot run the original. You must
work from its source code.

## 2. What to Build

Write a Go command-line program that behaves exactly like the original
`pyfiglet` command:

- The Go module root is `/workspace/repo`. The program is the `main` package
  in the module root, which is the root executable. The grader builds exactly
  this, and runs the resulting `/tmp/figlet`:

  ```
  cd /workspace/repo && go build -o /tmp/figlet .
  ```

  Packages in subdirectories are compiled only if the root package imports
  them. Nothing else in the module is built or run.

- Usage (the program is called `pyfiglet` below):

  ```
  pyfiglet [options] [text..]
  ```

  All non-option arguments are joined with a single space to form the text
  to render.

Examples:

```
pyfiglet Hello
pyfiglet -f slant "Hello world"
pyfiglet -w 40 -j center Hi there
pyfiglet -D right-to-left -s abc
```

## 3. Hard Rules

- **Go standard library only.** Do not add any third-party modules. `go.mod`
  must have no `require` entries.
- **No Python.** When you finish, no `.py` files may remain anywhere under
  `/workspace/repo`, including hidden directories. This includes the
  original sources, the tests, `setup.py` and `pyfiglet/fonts/__init__.py`.
  Do not create new `.py` files either.
- **Build.** `go build -o /tmp/figlet .` must succeed from `/workspace/repo`.
- **Fonts.** Use the `.flf` files already in `pyfiglet/fonts/`. Do not edit,
  rename or delete them, and do not add fonts.
- **Works from any directory.** The built binary must find its fonts no
  matter which directory it is run from or where it is copied. Embedding the
  fonts at build time with the standard `embed` package is the recommended
  way to do this.

## 4. Behaviour

For every valid invocation, standard output must match the original
`python -m pyfiglet [options] [text..]` **byte for byte**, including every
space and newline, and the program must exit with status `0`.

### Options

| Option | Argument | Default | Meaning |
|---|---|---|---|
| `-f` | `FONT` | `standard` | Font to render with, given by name without the extension (for example `slant`, `big`, `banner`). |
| `-w` | `COLS` (integer) | `80` | Width used for line wrapping and justification. |
| `-j` | `auto` \| `left` \| `center` \| `right` | `auto` | Justification. `auto` means left for left-to-right output and right for right-to-left output. |
| `-D` | `auto` \| `left-to-right` \| `right-to-left` | `auto` | Print direction. `auto` uses the font header's print direction: `1` is right-to-left, anything else is left-to-right. |
| `-r` | none | off | Reverse: mirror the rendered output horizontally (`FigletString.reverse`). |
| `-F` | none | off | Flip: turn the rendered output upside down (`FigletString.flip`). |
| `-s` | none | off | Strip blank lines before and after the figure (`strip_surrounding_newlines`). |
| `-n` | none | off | Keep exactly one empty line before and after the figure (`normalize_surrounding_newlines`). |

Option details:

- Options take their argument as the next word (`-f slant`). They may come
  before, between or after the text words.
- When several options are combined, apply them in this order: render, then
  `-r`, then `-F`, then `-s` or `-n`.
- If both `-s` and `-n` are given, only `-s` applies.
- After the result, the program writes one extra `\n`, just as the original
  `main()` does.

### Rendering

Reproduce the original algorithm exactly:

- **Font loading.** Parse the FIGfont header: hardblank, height, baseline,
  max length, old layout, comment lines, and the optional print direction
  and full layout. Convert the old layout to a full layout when the full
  layout is missing. Then load the ASCII characters 32–126, the seven
  German characters `ÄÖÜäöüß`, and any extra characters given by code. Only
  hex codes beginning with `0x` are used; other code tags are skipped.
  Strip end markers the same way the original does.
- **Layout.** Support full width, kerning (fitting) and smushing, and choose
  between them using the font's layout bits.
- **Smushing.** Implement all six horizontal smushing rules (equal
  character, underscore, hierarchy, opposite pair, big X, hardblank) and
  universal smushing. They must behave exactly as in
  `FigletBuilder.smushAmount` and `FigletBuilder.smushChars`. That includes
  the rule that no smushing happens when the previous or current character
  is narrower than 2 columns, and the special handling of hardblanks.
  Hardblanks are printed as spaces.
- **Wrapping.** A line breaks when its width would reach `-w`. It breaks at
  the last space if there is one, otherwise before the current character. A
  `\n` in the text forces a line break.
- **Justification and direction.** Pad and order the output exactly as
  `justifyString` and the right-to-left handling do.
- **Unsupported characters.** Text characters that the font does not define
  are skipped silently.

### Errors (reference only, not graded)

Every graded case is a successful invocation of the original. For
completeness, the original behaves like this on errors:

| Situation | Original exit status | Original standard output |
|---|---|---|
| Unknown font (`-f nosuchfont`) | `1` | `pyfiglet error: requested font 'nosuchfont' not found.` followed by `\n` |
| No text given | `1` | Usage help |
| A single character wider than `-w` | `1` | Nothing (Python traceback on standard error) |
| A character exactly as wide as `-w` | never exits | Nothing (the original loops forever) |
| Invalid `-j` or `-D` value, or a non-integer `-w` | `2` | Nothing (usage error on standard error) |

### Not required

The original's other options (`-l`, `-i`, `-L`, `-c`, `--version`), long
option names, and `.tlf`/zipped fonts do not need to be supported.

## 5. Not Checked

- Behaviour on errors and invalid input (see "Errors" above)
- Standard error
- Execution speed and memory use, within the per-case time limit
- How the Go code is organised internally

## 6. How It Is Graded

An automatic grader scores three criteria:

| Criterion | Weight | Passes when |
|---|---|---|
| Build | 1 | `go build -o /tmp/figlet .` succeeds in `/workspace/repo`. |
| No Python | 1 | The build succeeds, no file whose name ends in `.py` (any case) exists anywhere under `/workspace/repo`, and at least one hidden case passes. |
| Behaviour | 4 | Scored as the fraction of 2,000 hidden cases that pass. |

`reward = (build + no_python + 4 × behaviour) / 6`, a value from 0.0 to 1.0.
If the build fails, the reward is 0.

The hidden cases are valid, successful invocations of the original, covering
many fonts from `pyfiglet/fonts/`, many inputs and combinations of the
options above. A case passes when `/tmp/figlet` exits with status `0` and
its standard output is byte-for-byte identical to the original's. Each case
runs `/tmp/figlet` with its arguments passed directly, with no shell. It
gets empty standard input, a fresh empty working directory outside
`/workspace/repo`, and a 10-second time limit.
