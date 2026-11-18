# pyfiglet notes

| | |
|---|---|
| Repo | https://github.com/pwaller/pyfiglet |
| Commit SHA | `255827efbc09d662875dad50fa0edc1e1263bf45` |
| Commit date | 2026-08-02T22:51:18+01:00 |
| Licence | **MIT** (`LICENSE`: "The MIT License (MIT)"; `setup.py`: `license='MIT'`) |

Sources read: `pyfiglet/__init__.py` (mainly `FigletBuilder`), `Makefile`, `setup.py`, `LICENSE`.
I checked every claim below by running it on this commit with Python 3.15.2 on Windows.

---

## 1. Licence

MIT.

## 2. Why `python -m pyfiglet hello` fails straight after cloning

Fonts load as package resources with
`importlib.resources.files('pyfiglet.fonts')` (`FigletFont.preloadFont`).
The repo has no `pyfiglet/fonts/` folder. It only has `pyfiglet/fonts-standard/`
and `pyfiglet/fonts-contrib/`, so a fresh clone fails with:

```
ModuleNotFoundError: No module named 'pyfiglet.fonts'
```

**Fix (Makefile):** The `minimal` target does this:

```make
clean:
	rm -rf build/
	rm -rf pyfiglet/fonts/
	mkdir pyfiglet/fonts/

minimal:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	python3 -m build
```

The step that matters is `cp pyfiglet/fonts-standard/* pyfiglet/fonts`, run after
`clean` has created the empty folder. `fonts-standard/` includes an `__init__.py`,
so the copied folder becomes the `pyfiglet.fonts` package. (`setup.py` does the same
copy with `shutil.copytree` when you install the package, but that doesn't help when
you run from the clone.)

**Font setup used here.** This copies only the standard fonts, because the
`fonts-contrib/` fonts come from many authors:

```sh
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
python -m pyfiglet -f slant "hello"
```

Result (verified):

```
    __         ____
   / /_  ___  / / /___
  / __ \/ _ \/ / / __ \
 / / / /  __/ / / /_/ /
/_/ /_/\___/_/_/\____/
```

## 3. Smushing rules

A font's layout value is a bitmask (`FigletBuilder.__init__`). Smushing joins two
neighbouring characters by overlapping their touching edge columns. `smushChars(left, right)`
decides what single character replaces the pair, or returns `None` if they can't merge.

General conditions (checked before the numbered rules):
- If either side is a space, the other character wins (`' '` + `b` → `b`).
- If either character is less than 2 columns wide, they never overlap.
- If bit 128 (`SM_SMUSH`) is off, only kerning happens.

| Bit | Rule | What it does | Example (left + right → result) |
|---|---|---|---|
| 1 | **Equal character** | Two identical characters merge into one. Hardblanks are excluded. | `\|` + `\|` → `\|`; `#` + `#` → `#` |
| 2 | **Underscore** | `_` merges with any of `\| / \ [ ] { } ( ) < >`, and that character wins. | `_` + `\|` → `\|`; `/` + `_` → `/` |
| 4 | **Hierarchy** | Classes in order: `\|` < `/\` < `[]` < `{}` < `()` < `<>`. If the two characters are in different classes, the one in the later class wins. | `\|` + `/` → `/`; `[` + `{` → `{`; `(` + `<` → `<` |
| 8 | **Opposite pair** | Opposite brackets become a vertical bar. This works in either order. `<>` is not included. | `[` + `]` → `\|`; `{` + `}` → `\|`; `)` + `(` → `\|` |
| 16 | **Big X** | `/` + `\` → `\|`, `\` + `/` → `Y`, `>` + `<` → `X`. Order matters: `<` + `>` does not merge. | `/` + `\` → `\|`; `\` + `/` → `Y`; `>` + `<` → `X` |
| 32 | **Hardblank** | Two hardblanks (the font's `$`-style placeholder) merge into one hardblank. A hardblank with anything else does not merge. | `$` + `$` → `$` |
| 64 | **Kerning** | Characters slide together until they touch, with no overlap. Only spaces are absorbed. | `a` + `b` → no merge (they just touch); `' '` + `b` → `b` |

Extra mode: if bit 128 is on but none of bits 1–32 are, **universal smushing** applies.
The later character wins (`a` + `b` → `b`), and a visible character beats a hardblank
(`$` + `x` → `x`, `x` + `$` → `x`). With `-D right-to-left`, the left one wins instead.

## 4. Command-line options

| Option | What it does |
|---|---|
| `-f FONT`, `--font` | Chooses the font by name (default `standard`). It looks for `FONT.tlf` / `FONT.flf` in the `pyfiglet.fonts` package first, then in `./`, then in the shared font folder (`%APPDATA%\pyfiglet` on Windows, `/usr/local/share/pyfiglet/` elsewhere). If none is found, it prints `pyfiglet error: requested font 'X' not found.` |
| `-w COLS`, `--width` | Sets the output width (default 80) for wrapping and justifying. Text wraps at the last space that fits, or mid-word if there is none (`-w 20 hello world` → `hell`/`o`/`wo`/`rld`). A single character wider than `COLS` raises `CharNotPrinted`. |
| `-j SIDE`, `--justify` | Sets alignment: `auto` / `left` / `center` / `right`. `auto` follows the direction (left-to-right → left, right-to-left → right). It pads within `-w`; `right` pads to `width - 1`. |
| `-D DIR`, `--direction` | Sets direction: `auto` / `left-to-right` / `right-to-left`. `auto` uses the font's print-direction header. `right-to-left` lays out characters from right to left (`ab` renders as `ba`), and `auto` justify becomes right. |
| `-r`, `--reverse` | Mirrors the output left to right. Each row is reversed, and direction-sensitive characters are swapped (`/`↔`\`, `(`↔`)`, `<`↔`>`, `[`↔`]`, `{`↔`}`). |
| `-F`, `--flip` | Flips the output upside down. Rows are put in reverse order, and characters are swapped through a flip map (`/`↔`\`; `_`→`-`, but `-` stays `-`). Brackets are not changed. |
| `-s`, `--strip-surrounding-newlines` | Removes blank lines before and after the art. Blank lines inside a character are kept. |
| `-n`, `--normalize-surrounding-newlines` | Strips like `-s`, then adds exactly one empty line before and after. |

Order of application (from `main()`): render, then `-r`, then `-F`, then `-s` **or** `-n`.
If both `-s` and `-n` are given, `-s` wins.

---

## Rare cases (places where a quick read gets it wrong)

1. **The rule 16 code comment is wrong.** `__init__.py:538` says `# / + \ -> X, > + < -> X`,
   but the code returns `|` for `/` + `\` and `Y` for `\` + `/`. Only `>` + `<` gives `X`.
   The code matches the original FIGlet spec. Trust the code.
2. **Rule 8 works in both orders.** `]` + `[` → `|` and `)` + `(` → `|`, not just
   `[` + `]`. `<` + `>` is not a pair.
3. **Rule 16 depends on order.** `<` + `>` does not merge. Only `>` + `<` gives `X`.
4. **Narrow characters never smush.** If either character is 1 column wide or less,
   `smushChars` returns `None` even when a rule would match.
5. **`-s` also removes trailing spaces from the last row.** It ends with `.rstrip()`,
   so the last art line loses its padding. `-n` does the same before adding its newlines.
6. **`-s` beats `-n`.** It's an `if/elif`, so passing both behaves like `-s` alone.
7. **`-F` is not a full swap.** `_` turns into `-`, but `-` stays `-`. Brackets
   `()[]{}<>` are not changed by a flip.
8. **The fonts folder is missing from the clone**, so `python -m pyfiglet` fails until
   `pyfiglet/fonts/` is populated (section 2).
