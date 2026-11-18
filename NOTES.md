# Notes on pyfiglet

Repository: https://github.com/pwaller/pyfiglet
Commit: 255827efbc09d662875dad50fa0edc1e1263bf45
Commit date: 2026-08-02 22:51:18 +0100

## 1. Licence name

This project is under the MIT License (MIT).

Evidence: `LICENSE` begins with `The MIT License (MIT)`, and `setup.py` declares `license='MIT'`.

## 2. Why `python -m pyfiglet hello` fails immediately after cloning, and the Makefile fix

This is a packaging/setup issue, not a runtime bug in the logic itself.

In a fresh checkout, the runtime font package directory is not present:

- `pyfiglet/__init__.py` tries to resolve fonts from `pyfiglet.fonts` via `importlib.resources.files('pyfiglet.fonts')`.
- The repo contains `pyfiglet/fonts-standard`, but not the generated `pyfiglet/fonts` directory expected by the import path.
- That is why `python -m pyfiglet hello` fails immediately after cloning with `ModuleNotFoundError: No module named 'pyfiglet.fonts'`.

The repository explicitly expects the font files to be copied into `pyfiglet/fonts` before distribution or local use.

Verified from `Makefile`:

```make
minimal:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	python3 -m build

full:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	cp pyfiglet/fonts-contrib/* pyfiglet/fonts
	python3 -m build
```

The relevant repo step is therefore:

- `make minimal`  (copies the standard fonts into `pyfiglet/fonts` and then builds the package)

The `setup.py` file also verifies the same setup rule programmatically:

```python
pkg_src = path.join(here, 'pyfiglet', 'fonts')
repo_src = path.join(here, 'pyfiglet', 'fonts-standard')
if not path.isdir(pkg_src):
    shutil.copytree(repo_src, pkg_src)
```

So the generated `pyfiglet/fonts` directory is required at runtime, and the Makefile performs the necessary setup step before packaging.

## 3. Smushing rules and kerning

These rules are implemented in `FigletBuilder.smushChars()` and are gated by the font smush flags. The conditions are checked after the early guard cases:

- if `left == ' '` then return `right`
- if `right == ' '` then return `left`
- if `self.prevCharWidth < 2` or `self.curCharWidth < 2` then return `None`
- if `(self.font.smushMode & self.SM_SMUSH) == 0` then return `None`

After that, the six actual smush rules are evaluated in order.

### Kerning (`SM_KERN`, value 64)

Kerning is not one of the six smush rules; it is a separate spacing rule. In `smushAmount()`, if the font has `SM_KERN` but not `SM_SMUSH`, the code does not perform glyph merging; it only calculates tighter adjacent spacing. In other words, kerning is about spacing, not merging glyphs.

Example: there is no literal two-character replacement in the code for kerning; the operation is a width calculation, not a character-level merge.

### 1. Equal (`SM_EQUAL`, value 1)

Condition: `(self.font.smushMode & self.SM_EQUAL) != 0` and `left == right`.

Result: return `left` (equivalently `right`).

Example:

- `A` + `A` -> `A`

This is a true merge: identical edge characters collapse to a single instance.

### 2. Lowline (`SM_LOWLINE`, value 2)

Condition: `(self.font.smushMode & self.SM_LOWLINE) != 0` and one of these is true:

- `left == '_'` and `right in r'|/\[]{}()<>'`
- `right == '_'` and `left in r'|/\[]{}()<>'`

Result: return the non-underscore character.

Example:

- `_` + `|` -> `|`

This is exact: the code checks `left in a and right in b` or vice versa, with `a = '_'` and `b = r'|/\[]{}()<>'`.

### 3. Hierarchy (`SM_HIERARCHY`, value 4)

Condition: `(self.font.smushMode & self.SM_HIERARCHY) != 0` and the pair matches one of these sets:

- `('|', r'/\[]{}()<>')`
- `(r'\/','[]{}()<>')`
- `('[]','{}()<>')`
- `('{}','()<>')`
- `('()','<>')`

Result: return the dominant member from the other side.

Example:

- `|` + `/` -> `/`

This matches the first hierarchy tuple: `left in '|'` and `right in r'/\[]{}()<>'` => return `right`.

### 4. Pair (`SM_PAIR`, value 8)

Condition: `(self.font.smushMode & self.SM_PAIR) != 0`, and either `left + right` or `right + left` is one of `[]`, `{}`, or `()`.

Result: return `'|'`.

Examples:

- `[` + `]` -> `|`
- `{` + `}` -> `|`
- `(` + `)` -> `|`

This is exact: the code checks `for pair in [left+right, right+left]: if pair in ['[]', '{}', '()']: return '|'`.

### 5. BigX (`SM_BIGX`, value 16)

Condition: `(self.font.smushMode & self.SM_BIGX) != 0` and one of these is true:

- `left == '/' and right == '\\'`
- `left == '\\' and right == '/'`
- `left == '>' and right == '<'`

Result:

- `/` + `\` -> `|`
- `\` + `/` -> `Y`
- `>` + `<` -> `X`

Example:

- `/` + `\` -> `|`

### 6. Hardblank (`SM_HARDBLANK`, value 32)

This is special and must be handled carefully.

The code does:

```python
if self.font.smushMode & self.SM_HARDBLANK:
    if left == self.font.hardBlank and right == self.font.hardBlank:
        return left

if left == self.font.hardBlank or right == self.font.hardBlank:
    return
```

So the exact behavior is:

- if both sides are the font's `hardBlank`, collapse them to a single hardBlank value;
- if exactly one side is the hardBlank, do not smush at all;
- if neither side is the hardBlank, continue with the other smush rules.

This is not a visible glyph in the final output. Later, `replaceHardblanks()` does:

```python
string = string.replace(self.font.hardBlank, ' ')
```

So a hard blank is effectively treated as a space in the rendered text, but it remains a reserved font-character during smushing. That is why `hardBlank` is not a normal visible character in output.

Example (with a font whose hard blank is `.`):

- `.` + `.` -> `.` internally; final output prints as a space after hardblank replacement.

This is the exact behavior of the implementation; it is not a normal visible character merge.

## 4. Option meanings

From `pyfiglet/__init__.py` in `main()`:

- `-f`, `--font`: choose the font to render. Default is `standard`.
- `-w`, `--width`: set the width used for wrapping/justification. Default is `80` columns.
- `-j`, `--justify`: choose alignment: `left`, `center`, `right`, or `auto` (which follows print direction).
- `-D`, `--direction`: choose text flow direction: `auto`, `left-to-right`, or `right-to-left`.
- `-r`, `--reverse`: mirror the rendered output horizontally.
- `-F`, `--flip`: flip the output vertically.
- `-s`, `--strip-surrounding-newlines`: remove empty leading and trailing lines from the output.
- `-n`, `--normalize-surrounding-newlines`: force exactly one empty line before and after the rendered output.

## Summary

The key facts are verified directly against the source:

- The project is MIT-licensed.
- The font setup requires a generated `pyfiglet/fonts` directory; `make minimal` (or `make full`) is the repository fix, and `setup.py` also creates that directory when absent.
- Smushing is implemented by the exact flag checks and pair tables in `FigletBuilder.smushChars()`, with kerning kept separate from the six glyph-merging rules.
- Hardblanks are reserved font characters that collapse only when both sides are the same hardblank, and are later converted to spaces during final rendering.
