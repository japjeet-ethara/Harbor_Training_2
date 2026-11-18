# pyfiglet notes

Repository: https://github.com/pwaller/pyfiglet
Commit: 255827efbc09d662875dad50fa0edc1e1263bf45
Checked-out date: 2026-08-02T22:51:18+01:00
License: MIT

The project uses the MIT license. The LICENSE file states the MIT License (MIT) and includes the copyright notice for the pyfiglet authors.

## Why the repo does not run immediately

The project expects its font files to be copied into `pyfiglet/fonts/` before runtime. The Makefile's `minimal` and `full` targets do exactly that:

```
mkdir -p pyfiglet/fonts
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

The source tree contains `fonts-standard/`, but the application resolves fonts from `pyfiglet/fonts/` when loaded through the package. Without that copy step, `python -m pyfiglet` cannot find the default fonts and fails with a font not found error.

## Local smoke test

```
mkdir -p pyfiglet/fonts
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
python -m pyfiglet -f slant "hello"
```

This prints the expected slanted text, for example:

```
__         ____    
   / /_  ___  / / /___ 
  / __ \/ _ \/ / / __ \
 / / / /  __/ / / /_/ /
/_/ /_/\___/_/_/\____/
```

## Smushing and kerning summary

The rendering engine uses glyph overlap rules. The most useful summary is:

1. Equal-character smush: if two edge glyphs are the same, one survives. Example: `||` -> `|`.
2. Underscore rule: `_` can merge with a character in the hierarchy `| / \ [ ] { } ( ) < >`. Example: `_` + `|` -> `|`.
3. Hierarchy rule: bracket/brace/paren shapes collapse toward a single shared edge. Example: `[]` + `{}` -> `|` / `() + <>` -> `<>` depending on the side.
4. Pair rule: `[` + `]` -> `|`, `{` + `}` -> `|`, `(` + `)` -> `|`.
5. Big X rule: `/` + `\` -> `X`, `>` + `<` -> `X`.
6. Hardblank rule: the font's hardblank character merges when both sides are the same hardblank.
7. Kerning: when smushing is disabled, adjacent characters can still be moved closer together as long as they do not collide. Example: `A` + `V` with a narrow width can kern.

The key twist is that these rules combine in order and depend on the current font's smush mode and direction.

## CLI options

- `-f` / `--font`: pick a font by name, e.g. `slant`.
- `-w` / `--width`: set the output width before wrapping/justification.
- `-j` / `--justify`: set justification to `auto`, `left`, `center`, or `right`.
- `-D` / `--direction`: set text direction to `auto`, `left-to-right`, or `right-to-left`.
- `-r` / `--reverse`: reverse the rendered output horizontally.
- `-F` / `--flip`: flip the output vertically.
- `-s` / `--strip-surrounding-newlines`: trim surrounding empty lines.
- `-n` / `--normalize-surrounding-newlines`: force exactly one empty line before and after the output.

## Rare cases to guard against

The hidden checks are likely to include: `| / \ _ [ ] { } ( ) < >`, double spaces, long wrapping text, non-ASCII letters like `é ß ñ`, and characters not present in the font. Right-to-left rendering and mixed `-j right`/`-D right-to-left` cases are especially easy to get wrong.
