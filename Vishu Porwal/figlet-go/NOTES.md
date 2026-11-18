# pyfiglet source notes (step 1)

Pinned checkout confirmed before writing:

- `git rev-parse HEAD` = `255827efbc09d662875dad50fa0edc1e1263bf45`
- `git log -1` commit date = `2026-08-02T22:51:18+01:00`
- `NOTES.md` was absent

Status of claims below:

- **Source-confirmed:** read from this tree (or from `git log` for the commit date). Line numbers refer to this checkout.
- **Traced, not executed:** control flow followed by hand. No pyfiglet render was run, so these are not runtime results.
- **Inference:** a conclusion that needs an extra fact beyond the lines cited.
- **Not confirmed:** listed in the last section. Do not treat those as observed behavior.

`doc/figfont.txt` was not used as the behavior source. Meanings below come from `pyfiglet/__init__.py`, `Makefile`, `setup.py`, `LICENSE`, `pyfiglet/__main__.py`, `tools/pyfiglet`, `MANIFEST.in`, and `pyfiglet/tests/test_cli.py`.

## 1. Source

- URL: https://github.com/pwaller/pyfiglet
- SHA: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: `2026-08-02T22:51:18+01:00` (source-confirmed with `git log -1 --format=%cI`; subject `Return False from isValidFont for a missing font (#161)`)

## 2. Licence

Exact name in `LICENSE` line 3: **The MIT License (MIT)**.

`setup.py` lines 28 and 33 use the shorter strings `MIT` and `License :: OSI Approved :: MIT License`. Those are packaging labels. The name confirmed from `LICENSE` is `The MIT License (MIT)`.

## 3. Why a render cannot find fonts immediately after clone

### Where standard fonts are stored

Source-confirmed: the shipped standard font files are in `pyfiglet/fonts-standard/` (72 files at this checkout, including `standard.flf` and a 2-byte `pyfiglet/fonts-standard/__init__.py` that contains only a CR+LF blank line; verified by user PowerShell checks matching all 72 filenames and SHA256 hashes between source and copied fonts). Contributed fonts are a separate tree, `pyfiglet/fonts-contrib/` (521 files). Neither directory is on the loader's search list.

`git ls-files pyfiglet/fonts/*` prints nothing, and `pyfiglet/fonts/` is not on disk after clone. `MANIFEST.in` lines 1–3 package `pyfiglet/fonts/*.flf`, `*.flc`, and `*.tlf`. That is the directory the build creates, not a directory stored in git.

### Where the code expects to load them

`FigletFont.preloadFont` (`pyfiglet/__init__.py` lines 146–157) tries extensions in this order: `tlf`, then `flf`. For each name it does:

1. `importlib.resources.files('pyfiglet.fonts').joinpath(fn)` and `path.exists()`
2. Only when that path does not exist: `./`, then `FONT_DIRECTORIES`

There is no `try` around the `importlib.resources.files('pyfiglet.fonts')` call. The directory fallback is inside the `else` of `path.exists()`, so it runs only after that package resource lookup returns.

`FONT_DIRECTORIES` (lines 61–70):

- Windows: `%APPDATA%\pyfiglet` only
- Other platforms: `/usr/local/share/pyfiglet/`, `/usr/local/share/figlet`, `/usr/share/figlet`

`./` means a file named `{font}.tlf` or `{font}.flf` in the process current directory. It does not mean `pyfiglet/fonts-standard/`.

`tools/pyfiglet` lines 3–5 set `PYTHONPATH` to the repo root and exec `python -m pyfiglet`. That imports the `pyfiglet` package. It does not create `pyfiglet.fonts`.

**Inference, not executed:** immediately after clone, the first loader call raises while importing the missing `pyfiglet.fonts` package, and the `./` plus `FONT_DIRECTORIES` fallback never runs. `main` only catches `FontNotFound` (`pyfiglet/__init__.py` lines 1016–1018). A missing-package error is not that handler. The exception class was not captured by running the command.

`setup.py` lines 57–58 declares packages `pyfiglet` and `pyfiglet.fonts`, with package data `*.flf`, `*.flc`, `*.tlf`.

### Makefile font setup, separated from other setup

`Makefile` lines 12–19:

```make
minimal:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	python3 -m build

full:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	cp pyfiglet/fonts-contrib/* pyfiglet/fonts
	python3 -m build
```

The font-copy commands are the `cp` lines.

- `minimal` copies only `pyfiglet/fonts-standard/*` into `pyfiglet/fonts`.
- `full` copies that same standard set, then also copies `pyfiglet/fonts-contrib/*`.

These are different from the other operations in the same file:

- `clean` (lines 7–10) deletes `build/` and `pyfiglet/fonts/`, then `mkdir pyfiglet/fonts/`. That creates an empty destination. It copies nothing.
- `python3 -m build` on `minimal` and `full` builds a distribution. It is not the copy.
- `publish` (lines 21–23) runs `twine check` and `twine upload`. It does not copy fonts.
- `all` (lines 1–5) only prints help text.

`minimal` and `full` both depend on `clean`, so the copy always starts from a newly created empty `pyfiglet/fonts/`.

Separate from the Makefile: `setup.py` lines 8–13, if executed, `shutil.copytree`s `pyfiglet/fonts-standard` onto `pyfiglet/fonts` when `pyfiglet/fonts` is not already a directory. Clone does not run `setup.py`. After `make clean`, the directory exists, so this `isdir` check is false and `setup.py` would not fill the empty directory. The `cp` in `minimal` or `full` is the step that actually populates it on the Makefile path.

## 4. Horizontal smushing

All of this is `FigletBuilder` in `pyfiglet/__init__.py`. No vertical smushing procedure exists in `pyfiglet/*.py` (search for `smush` / `kern` hits only this horizontal path). `baseLine` and `maxLength` are parsed (lines 296–297) and never read again.

### Enabling bits

Defined at lines 534–541. Comments on those lines are the source's own names. One comment disagrees with the code; that is called out under `SM_BIGX`.

| Name | Bit | Source comment |
| --- | --- | --- |
| `SM_EQUAL` | 1 | smush equal chars (not hardblanks) |
| `SM_LOWLINE` | 2 | smush `_` with any char in hierarchy |
| `SM_HIERARCHY` | 4 | hierarchy: `\|`, `/\`, `[]`, `{}`, `()`, `<>` |
| `SM_PAIR` | 8 | `[` + `]` -> `\|`, `{` + `}` -> `\|`, `(` + `)` -> `\|` |
| `SM_BIGX` | 16 | comment says `/` + `\` -> `X`, `>` + `<` -> `X` |
| `SM_HARDBLANK` | 32 | hardblank + hardblank -> hardblank |

`SM_KERN` is 64 and `SM_SMUSH` is 128. They are not two of the six rules. Kerning is in its own section.

`font.smushMode` is the header `fullLayout` integer (line 321), or the old-layout conversion below when the header has no full-layout field (lines 306–314):

- `oldLayout == 0` -> `64` (`SM_KERN` only)
- `oldLayout < 0` -> `0` (neither kern nor smush)
- otherwise -> `(oldLayout & 31) | 128` (bits `SM_EQUAL` through `SM_BIGX`, plus `SM_SMUSH`). The `& 31` drops bit 32, so this conversion does not carry `SM_HARDBLANK`.

`standard.flf` line 1 is `flf2a$ 6 5 16 15 46 0 24463 229`. Hardblank is `$`, print direction is `0`, full layout is `24463`. `24463 & 255 = 143 = 128+8+4+2+1`, so this font sets `SM_SMUSH | SM_PAIR | SM_HIERARCHY | SM_LOWLINE | SM_EQUAL`. It does not set `SM_KERN`, `SM_BIGX`, or `SM_HARDBLANK`. `24463 & 63 = 15`, so it is not in universal-smushing mode. This bitmask was computed from that header integer; the font was not rendered.

### Gates that run before any of the six rules

`smushChars` (lines 779–862), in order:

1. If `left == ' '` (ASCII space only; the comment on lines 785–786 says `isspace` is intentionally not used), return `right`.
2. If `right == ' '`, return `left`.
3. If `prevCharWidth < 2` or `curCharWidth < 2`, return `None`. Overlap is refused for a character whose stored width is 0 or 1.
4. If `(smushMode & SM_SMUSH) == 0`, return `None`. This is the kerning-only stop.
5. If `(smushMode & 63) == 0`, universal overlapping runs and returns. The six rules are not consulted. See below.

`63` covers bits 1, 2, 4, 8, 16, and 32. Any one of the six rule bits turns universal mode off for every pair. A pair that fails the enabled rules does not fall through into universal mode. It returns `None` at line 862.

The six-rule checks then run in this order: hardblank, equal, lowline, hierarchy, pair, big-X. The first successful `return` wins.

### The six rules

Examples are **traced return values of `smushChars`**, not rendered glyphs. Unless a case says otherwise, assume `SM_SMUSH` is set, both character widths are at least 2, neither side is an ASCII space, and only the bit under discussion is set besides `SM_SMUSH`. `None` means "do not overlap these two glyphs."

Hardblank examples use `HB` for `font.hardBlank`. In `standard.flf` that character is `$`, but the comparison is against the font header, not a fixed dollar sign (lines 295, 319, 817–824).

**`SM_HARDBLANK` (bit 32)** — lines 817–824. Checked first among the six.

- If the bit is set and both sides equal `hardBlank`, return `left` (the hardblank).
- After that `if`, whether or not the bit is set: if either side is `hardBlank`, return `None`.

```text
HB + HB   ->  HB     # bit 32 set
HB + A    ->  None   # bit 32 set; a single hardblank never overlaps
A  + HB   ->  None
HB + HB   ->  None   # bit 32 clear; the either-side return wins
```

Equal hardblanks therefore never reach `SM_EQUAL`.

**`SM_EQUAL` (bit 1)** — lines 826–828.

- If the bit is set and `left == right`, return `left`.
- Otherwise this rule does not return, and later rules can still match.

```text
A + A -> A
A + B -> None
_ + _ -> _          # bit 1 set; this wins even if bit 2 is also set
```

**`SM_LOWLINE` (bit 2)** — lines 832–833 and 844–848. The pair stored is `('_', r'|/\[]{}()<>')`. Membership is string `in`, not a regex. The class characters are `| / \ [ ] { } ( ) < >`.

- If one side is `_` and the other side is in that class, return the class character (the underscore is dropped).
- `_` is not in the class, so `_` + `_` does not match this rule.

```text
_ + | -> |
| + _ -> |
_ + / -> /
_ + _ -> None       # bit 2 only; EQUAL is a different bit
_ + A -> None
```

**`SM_HIERARCHY` (bit 4)** — lines 835–848. Appended after the lowline pair when both bits are set, so lowline is tried first. Hierarchy pairs, first match wins:

```text
('|',    r'/\[]{}()<>')
(r'\/',  '[]{}()<>')
('[]',   '{}()<>')
('{}',   '()<>')
('()',   '<>')
```

If `left` is in the first string and `right` is in the second, return `right`. If `right` is in the first and `left` is in the second, return `left`. The character from the later class replaces the character from the earlier class. Two characters from the same class do not match, because the later class string does not contain the earlier class.

```text
| + / -> /
/ + | -> /
/ + [ -> [
[ + { -> {
{ + ( -> (
( + < -> <
< + ( -> <
[ + ] -> None       # same class; PAIR is a later rule
/ + \ -> None       # same class; BIGX is a later rule
```

**`SM_PAIR` (bit 8)** — lines 850–853. For `left+right` or `right+left`, if the two-character string is `[]`, `{}`, or `()`, return `|`.

```text
[ + ] -> |
] + [ -> |
{ + } -> |
} + { -> |
( + ) -> |
) + ( -> |
( + [ -> None
```

With bits 4 and 8 both set, `[` + `]` still reaches this rule: hierarchy does not match a same-class pair. Traced result remains `|`.

**`SM_BIGX` (bit 16)** — lines 855–861. The code does not do what the comment on line 538 says.

```text
/ + \ -> |          # comment says X; code returns '|'
\ + / -> Y          # comment does not mention this order
> + < -> X
< + > -> None       # this order is not handled
```

Only those three ordered pairs return. This rule is last, so it is reached only when hardblank, equal, lowline, hierarchy, and pair did not return.

### Universal smushing

Lines 801–815. Condition: `(smushMode & 63) == 0`, and the earlier `SM_SMUSH` gate passed. `SM_KERN` (64) can be set or clear; it is outside the mask.

- If one side is `hardBlank`, return the other side.
- If direction is `right-to-left`, return `left`.
- Otherwise return `right`.

The comment on lines 809–812 says the dominant character should be the later character in the user's text, not necessarily the rightmost column. That lines up with the direction swap described next: `smushRow` swaps the two strings before calling `smushChars` when direction is `right-to-left` (lines 659–660), and universal mode then keeps `left`, which is the post-swap value.

```text
# smushChars arguments, SM_SMUSH only (value 128), LTR
A + B -> B
HB + B -> B
B + HB -> B

# same arguments, direction already 'right-to-left'
A + B -> A
```

These are traced `smushChars` results, not full output lines.

### Kerning, separate from the six rules

`SM_KERN` is bit 64 (line 540).

`smushAmount` (lines 728–776) returns `0` immediately when neither `SM_SMUSH` nor `SM_KERN` is set. Characters are then appended at full stored width. The amount is also `0` for a character that cannot move, because the per-row amounts are reduced to a minimum that starts at `curCharWidth`.

When `SM_KERN` is set and `SM_SMUSH` is not:

- `smushAmount` still computes a shift from ASCII-space borders. Each row uses `rstrip(' ')` on the current line and `lstrip(' ')` on the new character (lines 747–764). The extra `+ 1` that overlaps two visible edge glyphs is added only when `ch1` is empty or space, or when `smushChars(ch1, ch2)` returns a character (lines 768–772).
- `smushChars` hits the kerning stop at lines 797–799 and returns `None` for two non-space glyphs. That `+ 1` is therefore not added for two visible edges.
- Spaces still resolve inside `smushChars` before that stop: a space side returns the other side.

```text
# SM_KERN set, SM_SMUSH clear
smushChars('A', 'B') -> None
smushChars(' ', 'B') -> 'B'
smushChars('A', ' ') -> 'A'
```

Traced meaning: kerning can consume bordering ASCII spaces so glyphs move together until their non-space edges meet. It does not replace those edge glyphs. That replacement requires `SM_SMUSH`, and then either universal mode or one of the six rules.

`smushAmount` always passes `left=ch1, right=ch2` in stored-line order after the optional RTL swap (lines 744–745, 771). It does not implement a second set of rules.

### Direction-dependent behavior in this source

Source-confirmed in the loader and builder:

- `Figlet.direction` (lines 886–897): `auto` reads `Font.printDirection`. `0` -> `left-to-right`. `1` -> `right-to-left`. Any other value, including a missing header field (`None`), -> `left-to-right`. A non-`auto` setting is returned unchanged.
- `smushAmount` (lines 744–745) and `smushRow` (lines 659–660) swap the existing row and the new character when direction is `right-to-left`.
- After that swap, `addCurCharRowToBufferRow` (lines 669–671) stores `addLeft + addRight[maxSmush:]`. In RTL the new character is on the left of the accumulated row, and the overlapped columns are eaten from the old row. In LTR the old row stays on the left and the new character loses its overlapped prefix.
- Universal mode's LTR/RTL return, above, is the only branch inside `smushChars` that reads `self.direction`.
- `SM_BIGX` is order-sensitive. **Traced, not executed:** because RTL swaps the strings before `smushChars`, a user order of `/` then `\` is the `/`+`\` branch in LTR (`|`) and the `\`+`/` branch in RTL (`Y`). Pair, equal, lowline, hierarchy, and hardblank each handle both orders themselves or compare for equality, so the replacement character does not change when the arguments are swapped. The column that gets overwritten still changes, because the buffer assembly is swapped.
- `-r` is not this direction flag. It is a post-render mirror. See section 5.

`Figlet.justify` when `auto` follows direction (lines 900–908): LTR -> `left`, RTL -> `right`. If `auto` is combined with some other direction string, the property falls off the end and returns `None`. The CLI choices cannot produce that third direction. Explicit `left`, `center`, or `right` do not follow direction.

## 5. CLI behavior from this source

Entry points: `setup.py` lines 59–62 (`pyfiglet = pyfiglet:main`) and `pyfiglet/__main__.py` lines 3–7 (`from pyfiglet import main`). Parsing is `OptionParser` in `main` (lines 945–984).

With no text arguments, `main` prints help and returns 1 (lines 1002–1004). Text arguments are joined with single spaces (line 1009), then rendered. On Python 3, stdout is detached to binary and the result is written as UTF-8 followed by one extra `b'\n'` (lines 1030–1039). That extra newline is unconditional on the success path.

Post-render order (lines 1020–1028): `renderText`, then `-r`, then `-F`, then `-s`, else `-n` if `-s` was not set.

`Figlet` defaults used by the CLI (lines 870–871, 1012–1014): font `standard`, direction `auto`, justify `auto`, width `80`.

### `-f` / `--font`

- Default: `standard` (`DEFAULT_FONT`, line 52; option line 948).
- Passed through as the font name. Lookup appends `.tlf` first and `.flf` second (lines 146–151). A `.tlf` of the same name hides the `.flf`.
- Search order is the package `pyfiglet.fonts`, then `./`, then `FONT_DIRECTORIES` (section 3).
- `FontNotFound` prints `pyfiglet error: requested font {name!r} not found.` and returns 1 (lines 1016–1018).
- A font that fails to parse raises `FontError` from `loadFont` (lines 286–287, 292–293, 380–381). `main` does not catch `FontError`.

### `-w` / `--width`

- Integer option, default `80` (lines 960–962). The help text calls this a terminal width. The implementation never reads the terminal. It uses this integer.
- Before a character is considered, if `width <` that character's stored width, `CharNotPrinted` is raised (lines 556–557). `main` does not catch it.
- Otherwise projected width is `len(buffer[0]) + curCharWidth - maxSmush` (line 561). If that is `>= width`, the builder wraps instead of adding the character (lines 570–574). The comparison uses row 0's length only.
- Wrap (`handleNewLine`, lines 703–708): if a blank marker exists, restore the buffer saved at the last space or newline and set the iterator to that marker. The engine then increments the iterator (`goToNextChar`, lines 579–580), so that saved space is skipped and not reprinted. If no blank marker exists, the current buffer is flushed and the iterator is moved back one so the overflowing character is retried on the next line (lines 688–691).
- A newline in the input is handled before the width check (lines 549–552): the current buffer is stored as a blank marker, then `handleNewLine` flushes it and the following `goToNextChar` steps past the newline.
- Justification uses this same `width` value, per flushed row, after wrapping (section `-j`).

### `-j` / `--justify`

- Choices: `auto`, `left`, `center`, `right`. Default `auto` (lines 956–959).
- Help string: "set justification, defaults to print direction".
- `auto` is resolved by `Figlet.justify` before the builder sees it: LTR -> `left`, RTL -> `right` (lines 900–905). Explicit `left`, `center`, and `right` stay as given and do not follow `-D`.
- `justifyString` (lines 710–721), applied separately to each wrapped band in `formatProduct` (lines 606–608):
  - `left`: no spaces added. Any other unrecognized value, including a resolved failure that returned `None`, takes this same path.
  - `right`: prepend `' ' * (width - len(row) - 1)` spaces. The `- 1` is in the source. For a row shorter than `width`, the padded length is `width - 1`, not `width`.
  - `center`: prepend `' ' * int((width - len(row)) / 2)` spaces. Integer division, no trailing pad. A short row stays short of `width`, and an odd leftover column stays on the right.
- Padding is against the selected `-w` value, not against the longest rendered line and not against a measured terminal.
- In Python, `' ' * n` for `n < 0` is `''`. **Traced, not executed:** a row already longer than `width` gets no padding and raises no error inside `justifyString`.
- Hardblanks are turned into spaces only after this padding (lines 723–726). They still occupy columns while the pad count is computed.

### `-D` / `--direction`

- Choices: `auto`, `left-to-right`, `right-to-left`. Default `auto` (lines 951–955).
- `auto` uses the font header, not a global default of LTR, except that any print-direction other than `1` becomes LTR (lines 886–894). `standard.flf` has print direction `0`, so `auto` for that font is LTR. Source-confirmed from the header; not confirmed by a render.
- Explicit direction is independent of explicit justification. It changes auto justification only while `-j` remains `auto`.
- RTL changes assembly and universal overlap as in section 4. It does not call `FigletString.reverse`.

### `-r` / `--reverse`

- Flag, default false (lines 963–964). Help: "shows mirror image of output text".
- After rendering, `FigletString.reverse` (lines 433–438) reverses each line and applies `__reverse_map__` (lines 396–412).
- The map literals were compared with an identity table for codes 32–126. Codes outside that range are an identity run in the literal. Replacements:

```text
( -> )
) -> (
/ -> \
\ -> /
< -> >
> -> <
[ -> ]
] -> [
{ -> }
} -> {
```

Letters, digits, `|`, `_`, `-`, quotes, and the other ASCII punctuation are unchanged by the map. The line is also reversed end to end. This happens before `-F` and before `-s`/`-n`. It does not re-wrap or re-justify.

### `-F` / `--flip`

- Flag, default false (lines 969–970). Help: "flips rendered output text over".
- After `-r` if both were set, `FigletString.flip` (lines 440–445) reverses the row order, then applies `__flip_map__` (lines 415–431). It does not reverse columns.
- Same literal comparison, codes 32–126. Replacements:

```text
/ -> \
\ -> /
A -> V
V -> A
M -> W
W -> M
P -> b
R -> b
b -> P
m -> w
w -> m
^ -> v
v -> ^
_ -> -
```

`P` and `R` both become `b`, so this map is not a one-to-one flip. `[`, `]`, `{`, `}`, `<`, `>`, `(`, `)`, and `|` are unchanged by the flip map.

### `-s` / `--strip-surrounding-newlines` and `-n` / `--normalize-surrounding-newlines`

- Both default false (lines 965–968).
- If `-s` is set, `strip_surrounding_newlines` runs and `-n` does not, even when both flags are present (`if` / `elif`, lines 1025–1028).
- `-s` help says it "removes empty leading and trailing lines". The method (lines 447–459) does more than that sentence:
  - Leading rows with `row.strip() == ''` are dropped. A leading row of only spaces counts as empty.
  - Once a non-empty row has been seen, later empty rows are kept, including empty rows inside the glyph.
  - The joined text is then `rstrip()`ed. That removes trailing newlines and trailing spaces on the last remaining line, not only fully empty lines.
  - The comments on lines 447–448 say it avoids `strip()` / per-row `strip()` so it will not delete leading spaces on the first kept line or empty lines inside a character.
- `-n` help says "output has one empty line before and after". `normalize_surrounding_newlines` (lines 461–462) is `'\n' + strip_surrounding_newlines() + '\n'`. It reuses the strip method, including the trailing-space `rstrip`, then forces one leading newline and one trailing newline.
- **Traced, not executed:** `main` then writes another `b'\n'`. For `-n`, the bytes are: one leading newline, the stripped body, the normalize newline, and `main`'s newline. For `-s`, the stripped body has no trailing newline, and `main` adds one. For neither flag, each flushed band already ends in a newline (`replaceHardblanks`, line 724) and `main` adds one more.

`pyfiglet/tests/test_cli.py` contains expected stdout strings for `pyfiglet -f slant -s 0` (lines 14–26) and `pyfiglet -f slant -n 0` (lines 67–81). The comment on line 67 says normalize is strip with padding. Those are repository assertions. They were not executed here.

### Interactions

- **`-s` versus `-n`:** `-s` wins when both are set. `-n` is strip followed by one forced newline on each side. Both share the leading-blank skip and the trailing `rstrip`.
- **Direction versus justification:** `-D` selects LTR or RTL assembly, and the universal-smush winner. `-j auto` then left-aligns LTR and right-aligns RTL. Setting `-j left`, `-j center`, or `-j right` keeps that alignment even if `-D` is RTL or LTR. `-r` mirrors the finished lines and does not change the direction used during smush. `-F` flips rows after `-r`.
- **Justification versus width:** wrapping tries to keep the next projected row-0 width `< -w`. Padding then uses that same `-w`. Right padding targets `width - 1`. Center padding targets `len + floor((width - len) / 2)`. Left padding adds nothing, so a left-justified line is only as wide as the wrapped content.

## 6. Rare-case candidates for later testing

These are candidates derived from the source. They are **not verified runtime results**.

1. Clone, then render the default font, with neither `make minimal` nor `make full` nor `setup.py`. Expect failure while resolving `pyfiglet.fonts`, and expect `fonts-standard/standard.flf` not to be consulted. Exact exception text was not captured.
2. `make clean` alone, then render. `clean` leaves `pyfiglet/fonts/` as an empty directory (Makefile lines 9–10). `setup.py`'s copy runs only when that path is not a directory (`setup.py` lines 12–13). Candidate: an empty package directory still does not contain `standard.flf`.
3. `make minimal` versus a font that exists only under `fonts-contrib`. `minimal` does not copy contrib. `full` does.
4. Same font name as both `.tlf` and `.flf`. Loader prefers `.tlf` (`__init__.py` lines 146–151).
5. Font file whose header has no full-layout field. Modes: old layout `0` -> kerning only; negative -> no fitting; positive -> `(oldLayout & 31) | 128`, which cannot enable hardblank smushing (`__init__.py` lines 308–314).
6. Font whose `printDirection` is absent, or not `0` or `1`, with `-D auto`. Candidate result is LTR (`__init__.py` lines 887–894).
7. `standard.flf` layout `24463`: equal, lowline, hierarchy, and pair are enabled; big-X, hardblank smushing, kerning, and universal mode are not. Useful control font for "rule did not match, so glyphs must not overlap."
8. Universal mode (`SM_SMUSH` set, bits 1–32 all clear): any two wide non-space glyphs overlap; hardblank loses; LTR keeps the right argument and RTL keeps the left argument after the builder swap.
9. Kerning only (`smushMode == 64`): spaces can be consumed; `A` against `B` must not become a combined glyph.
10. Neither kern nor smush (`smushMode == 0`): `smushAmount` returns 0 (`__init__.py` lines 737–738). Candidate: full stored widths, including bordering spaces.
11. Character width 0 or 1 with `SM_SMUSH` set: `smushChars` returns `None` before the rules (`__init__.py` lines 792–795), so visible edges must not overlap.
12. Big-X comment/code mismatch, both directions: `/`+`\` -> `|`, `\`+`/` -> `Y`, `>`+`<` -> `X`, `<`+`>` -> no big-X replacement (`__init__.py` lines 538, 855–861).
13. All six rule bits set, precedence: `HB+HB` stays hardblank; `HB+A` does not overlap; `_`+`_` stays `_` via equal; `_`+`/` becomes `/` via lowline before hierarchy; `[`+`]` becomes `|` via pair; `/`+`\` becomes `|` via big-X because hierarchy treats them as the same class.
14. Hardblank replaced only at the end (`__init__.py` lines 723–726). Candidate: during layout a hardblank is not an ASCII space, so it blocks kerning space-skipping, then becomes a space in the output and can look like justification padding.
15. Codepoint not in the font, other than newline: `getCharAt` returns `None` and `addCharToProduct` returns without inserting a placeholder (`__init__.py` lines 554–555, 611–619). The character disappears.
16. Newline: flushes the current band and does not look the newline up in the font (`__init__.py` lines 549–552). A second newline check at lines 566–568 is unreachable because the first one returns.
17. Space: records a blank marker of the buffer from before the space (`__init__.py` lines 563–564). A later wrap restores that marker and the following iterator increment skips the space.
18. Input wider than `-w` with no space on the line: cut at the last character and retry it (`cutBufferAtLastChar`, lines 688–691).
19. Stored glyph width exactly equal to `-w`, on an empty band, with `maxSmush == 0`. The `>= width` test fires before the glyph is stored, and the no-blank path decrements the iterator that `goToNextChar` then increments. **Candidate infinite loop. Not run.**
20. Rows of one glyph have different lengths. Wrap and smush distance use `buffer[0]` (`__init__.py` line 561) while each row is smushed separately (lines 573–574).
21. First glyph whose every row begins with spaces, against an empty buffer. `smushAmount` can return a positive amount from those leading spaces (lines 759–776). Candidate: the first character is shifted.
22. `-w` smaller than a single glyph's stored width. Candidate: uncaught `CharNotPrinted` (`__init__.py` lines 556–557, 1011–1020).
23. `-j right` with content shorter than `-w`. Candidate pad length is `width - len(row) - 1`, so the line ends one column before `width`.
24. `-j center` with an odd gap. Candidate: the extra column is on the right, and the line is not padded out to `-w`.
25. `-j right` or `-j center` when a row is already longer than `-w`. Candidate: the negative repeat count adds no spaces and does not raise.
26. `-D right-to-left -j left`, and the opposite `-D left-to-right -j right`. Direction and justification should come apart: assembly follows `-D`, padding follows `-j`.
27. `-D right-to-left -j auto`. Candidate: RTL assembly plus right padding.
28. `-s` and `-n` together. Candidate: `-s` only.
29. `-s` on a font with trailing spaces on the last occupied row. Candidate: `rstrip` deletes those spaces, which the help text does not say (`__init__.py` lines 449–459 versus help lines 967–968).
30. `-n` on the same font. Candidate: those trailing spaces are removed and then exactly the normalize newlines are added, plus `main`'s extra newline.
31. Neither `-s` nor `-n`. Candidate: font rows are preserved, including leading blank rows inside the glyph height, and `main` adds one more trailing newline.
32. `-r` then `-F` together, because the order is fixed: mirror first, vertical flip second (`__init__.py` lines 1021–1024).
33. `-r` on parentheses, slashes, and brackets: the map swaps those glyphs and the line is reversed. Letters should be reversed in position only.
34. `-F` on `P` and `R`: both map to `b`. Candidate: flip is not reversible for those letters.
35. Several CLI words. `main` joins them with one space (`__init__.py` line 1009). `pyfiglet hello world` is the text `hello world`.
36. Empty string argument, as opposed to no arguments. No arguments print help and return 1 (lines 1002–1004). A present empty argument is still an argument. Candidate: empty render plus `main`'s extra newline. Not run.
37. `getFonts` validates `font.name` only (`__init__.py` lines 216–218), and `isValidFont` searches the cwd, `FONT_DIRECTORIES`, and the package, not the directory the name was listed from (lines 178–192). Candidate: a basename collision accepts or rejects the wrong file. `isValidFont("does_not_exist.flf")` is supposed to return `False` (lines 193–194 and the test at `pyfiglet/tests/test_cli.py` lines 58–64). That test was not run.
38. `getFonts` keeps `name.split('.', 2)[0]` (line 216). Candidate: a file `a.b.flf` is listed as `a`.
39. ZIP font: `preloadFont` and `isValidFont` read only `namelist()[0]` (`__init__.py` lines 162–165, 196–200).
40. Font body contains U+0085, U+2028, or U+2029. Those are rewritten to spaces before parsing (lines 278–280).
41. Extended glyph whose code line does not start with `0x`. The loader ignores the line and does not parse a decimal code (`__init__.py` lines 367–378). Later glyph rows can be consumed as code labels.
42. The seven characters `ÄÖÜäöüß` are loaded by position immediately after code 127, not by a code-point label (lines 358–364). A short or shifted font can attach those rows to the wrong characters, or raise `FontError` while popping lines.
43. `smushChars` returns `None` and `updateSmushedCharInLeftBuffer` still assigns that value into the row (lines 647–652, 662–666). `''.join` cannot accept `None`. Candidate `TypeError` if `maxSmush` ever overlaps two non-space glyphs that the rules refuse. Not run.
44. On Windows, `SHARED_DIRECTORY` is computed at import from `os.environ["APPDATA"]` (lines 61–63). A missing `APPDATA` would fail at import, before font lookup. Not run.
45. `infoFont(..., short=True)` indexes `infos[0]` (line 239). A font with no collected info lines is a candidate `IndexError`. Not part of the eight flags, but it is on the `-i` path.

## User-observed runtime checks (attributed, not agent-executed)

The following were observed by the user on this checkout, not executed by the agent that wrote or audited these notes:

- PowerShell file verification: 72 source files (`fonts-standard/`) and 72 copied files (`pyfiglet/fonts/`), all filenames and SHA256 hashes match; copied `__init__.py` is 2 bytes (containing a CR+LF newline, correcting the claim that it was empty).
- `python --version`: Python 3.12.10
- `PYTHONUTF8` (UTF-8 mode): `1`
- `sys.dont_write_bytecode`: `True`
- Imported module path: `C:\Users\VishuPorwal\Desktop\Track2\pyfiglet\pyfiglet\__init__.py`
- `python -m pyfiglet -f standard Hello` rendered successfully, exit code 0.

These checks confirm that the pinned checkout imports and renders with the default font. They do not validate other flags, exact output bytes, smushing rules, or edge cases described in these notes.

## Not confirmed

- Every character-pair result and every padding length above is traced from the source, not captured output.
- The exception type and message for a missing `pyfiglet.fonts` package were not captured.
- The suspected exact-width infinite loop was not run.
- The assignment's exact rendering command string is not in this tree. The notes cover `main`, `python -m pyfiglet`, and `tools/pyfiglet`, which all use `preloadFont`.
- `doc/figfont.txt` was not checked against these rules.
- Whether an empty `pyfiglet/fonts/` directory imports as a namespace package on the local Python was not tested. The directory is absent in the clone, which is the case section 3 describes.
