# Pyfiglet notes

## 1) License name

This project is under the MIT License.

The project headers in both `pyfiglet/__init__.py` and `LICENSE` say:

- `The MIT License (MIT)`
- `setup.py` also declares `license='MIT'`

## 2) Why `python -m pyfiglet hello` fails immediately after cloning, and which Makefile step fixes it

It fails because a clean clone does not include the generated font package directory `pyfiglet/fonts`.

The relevant code path is `FigletFont.preloadFont()`: it tries to load `pyfiglet.fonts` via `importlib.resources.files('pyfiglet.fonts')`, and if that package/resource is missing, it raises `FontNotFound`.

`setup.py` contains the bootstrap logic:

- if `pyfiglet/fonts` does not exist, it copies `pyfiglet/fonts-standard` into `pyfiglet/fonts`

That is the repo setup step which makes the default font available.

The Makefile has the concrete fix:

- `make minimal` copies `pyfiglet/fonts-standard/*` into `pyfiglet/fonts` and then runs `python3 -m build`
- `make full` does the same, then also adds the contributed fonts from `pyfiglet/fonts-contrib`

So the fix is: `make minimal` (or `make full` if you want the contrib fonts too).

## 3) Smushing rules and kerning

These are the logic names from `FigletBuilder` in `pyfiglet/__init__.py`.

- Kerning (`SM_KERN`): when the font allows kerning, adjacent characters can be pulled together without full overlap; the gap is reduced. Example: `A` + `V` -> `AV` (the spacing is reduced rather than a literal overlap).

- Equal-character smushing (`SM_EQUAL`): if the two edge characters are the same (and not hardblanks), they collapse to one. Example: `H` + `H` -> `H`

- Underscore smushing (`SM_LOWLINE`): `_` can merge with the hierarchy set. Example: `_` + `|` -> `|`

- Hierarchy smushing (`SM_HIERARCHY`): characters in the hierarchy collapse toward one another, e.g. `|`, `/`, `\`, `[]`, `{}`, `()`, `<>`. Example: `|` + `/` -> `/`

- Pair smushing (`SM_PAIR`): special pairs collapse to a single `|`. Example: `[` + `]` -> `|`

- Big-X smushing (`SM_BIGX`): slash/backslash and greater/less forms collapse into a special merged character. Example: `/` + `\` -> `|`

- Hardblank smushing (`SM_HARDBLANK`): if the font's hardblank character is used on both sides, it stays as the hardblank. Example (assuming hardblank is `#`): `#` + `#` -> `#`

Notes:

- The code checks these rules in order in `smushChars()`.
- `SM_SMUSH` is the overall "smushing enabled" flag; `SM_KERN` is a separate kerning path.

## 4) Option meanings

From `main()` in `pyfiglet/__init__.py`:

- `-f`, `--font FONT`: choose the font to render with; default is `standard`.

- `-w`, `--width COLS`: set the terminal width used for wrapping/justification.

- `-j`, `--justify SIDE`: set text justification. Choices are `auto`, `left`, `center`, or `right`; defaults to the print direction.

- `-D`, `--direction DIRECTION`: set text direction. Choices are `auto`, `left-to-right`, or `right-to-left`.

- `-r`, `--reverse`: show the mirror image of the rendered text.

- `-F`, `--flip`: flip the rendered output over (vertical flip / upside-down style).

- `-s`, `--strip-surrounding-newlines`: remove empty leading and trailing lines around the output.

- `-n`, `--normalize-surrounding-newlines`: normalize surrounding empty lines to exactly one blank line before and after the output.

In short:

- `-r` = mirror horizontally
- `-F` = flip vertically
- `-s` = discard surrounding blank lines
- `-n` = normalize to one surrounding blank line
