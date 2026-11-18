# PyFiglet Port Documentation (FIGlet-Go)

## 1. Original PyFiglet Repository URL
- **Repository URL**: https://github.com/pwaller/pyfiglet

## 2. Pinned Commit Hash and Commit Date
- **Commit Hash**: `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Commit Date**: Sun Aug 2 22:51:18 2026 +0100 (`2026-08-02 22:51:18 +0100`)
- **Author**: Apoorv Darshan <ad13dtu@gmail.com>
- **Commit Subject**: `Return False from isValidFont for a missing font (#161)`

## 3. License Information
- **License**: MIT License (MIT)
- **Copyright**: Copyright © 2007–2018
  - Christopher Jones <cjones@insub.org>
  - Stefano Rivera <stefano@rivera.za.net>
  - Peter Waller <p@pwaller.net>
  - And various contributors (see git history).
- **Terms**: Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files, to deal in the Software without restriction, including rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies.

## 4. Standard Font Setup and Font Details
- **Font Directory Structure**:
  - Upstream PyFiglet organizes fonts into `pyfiglet/fonts-standard/` and `pyfiglet/fonts-contrib/`.
  - In standard packaging and benchmark environments, fonts from `fonts-standard/` are placed into `pyfiglet/fonts/`.
  - The standard test fonts include `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`, and `script`.
  - When loading fonts, canonical standard fonts (`fonts-standard`) take precedence over contributed fonts (`fonts-contrib`), avoiding glyph divergences (such as space character definitions in `shadow.flf`).
- **FIGlet Font Format (`.flf` / `.tlf`)**:
  - **Header Line**: `flf2a$ height baseline maxlen old_layout comment_lines [print_direction [full_layout [codetag_count]]]`
    - `flf2a`: Magic identifier for FIGlet 2 fonts.
    - `$`: Hardblank character (the character immediately following `flf2a`, typically `$`), displayed as empty space in output but preserved during smushing.
    - `height`: Number of lines high each glyph is.
    - `baseline`: Distance from top of glyph to baseline.
    - `maxlen`: Maximum line length of any character.
    - `old_layout`: Smushing control integer (`-1` = full width / no smush, `0` = kerning, `>0` = smush rules bitmask).
    - `comment_lines`: Number of comment lines following the header line before character glyph definitions.
    - `print_direction`: `0` for left-to-right (default), `1` for right-to-left.
    - `full_layout`: Extended smush mode bitmask, superseding `old_layout` when present.
  - **Character Glyphs**:
    - Lines end with an endmarker character (typically `@` at the end of each line and `@@` at the end of the final line of a character).
    - Standard characters cover ASCII 32 (` `) through ASCII 126 (`~`), followed by German characters and optional Unicode tags.

## 5. FIGlet Smushing Rules and Kerning
FIGlet defines layout modes controlling horizontal character spacing: full width, kerning, and smushing.

- **Kerning (`SMKern = 64`)**:
  - Moves the incoming character to the left as far as possible without overlapping any non-blank glyph characters.
  - Hardblanks are treated as visible characters and will not overlap or kern into other characters.

- **Smushing (`SMSmush = 128`)**:
  - Allows the right edge of the preceding glyph and the left edge of the current glyph to overlap and merge into a single column.
  - Controlled by 6 individual bitmask rules (summed in `smushMode` bits 1–32):

  1. **Rule 1: Equal-Character Smushing (`SMEqual = 1`)**:
     Two identical non-blank characters smush into that character (e.g., `|` + `|` -> `|`, `x` + `x` -> `x`).
  2. **Rule 2: Underscore Smushing (`SMLowline = 2`)**:
     An underscore (`_`) smushes with any of `|`, `/`, `\`, `[`, `]`, `{`, `}`, `(`, `)`, `<`, `>` into the other character.
  3. **Rule 3: Hierarchy Smushing (`SMHierarchy = 4`)**:
     Characters smush based on class hierarchy, where the higher-class character supersedes:
     `|` > `/\` > `[]` > `{}` > `()` > `<>`
     (e.g., `|` smushes with `(` to yield `|`; `[` smushes with `>` to yield `[`).
  4. **Rule 4: Opposite-Pair Smushing (`SMPair = 8`)**:
     Opposing symmetric bracket pairs smush into a vertical bar `|`:
     `[]` or `][` -> `|`
     `{}` or `}{` -> `|`
     `()` or `)(` -> `|`
  5. **Rule 5: Big-X Smushing (`SMBigX = 16`)**:
     Diagonal lines and angles smush:
     `/` + `\` -> `|`
     `\` + `/` -> `Y`
     `>` + `<` -> `X`
  6. **Rule 6: Hardblank Smushing (`SMHardblank = 32`)**:
     Two hardblank characters smush into a single hardblank character.

- **Universal Smushing / Universal Overlap (`smushMode & 63 == 0`)**:
  - When smushing is enabled (`SMSmush` set or `oldLayout > 0`) but none of the 6 specific rules are active (`smushMode & 63 == 0`):
  - Any non-hardblank character replaces a blank.
  - When two non-blank characters collide, the later character overrides the earlier character (in left-to-right, right character wins; in right-to-left, left character wins). Hardblanks are never overwritten by universal smushing.

## 6. Supported CLI Flags
The Go binary supports eight command-line flags matching PyFiglet:

| Flag | Full Option | Argument | Description |
| :--- | :--- | :--- | :--- |
| `-f` | `--font` | `FONT` | Font name or path (default: `standard`). Supports attached syntax (e.g. `-fslant`, `-f=slant`). |
| `-w` | `--width` | `COLS` | Output width in columns (default: `80`). Used for word wrapping and justification alignment. Supports attached syntax (e.g. `-w120`, `-w=120`). |
| `-j` | `--justify` | `auto\|left\|center\|right` | Text justification mode. `auto` infers justification from print direction (`left` for LTR, `right` for RTL). |
| `-D` | `--direction` | `auto\|left-to-right\|right-to-left` | Text rendering direction. `auto` uses the font's embedded print direction. |
| `-r` | `--reverse` | *(none)* | Reverses output horizontally (mirroring lines and swapping mirrored characters like `(`, `)`, `/`, `\`, `[`, `]`, etc.). |
| `-F` | `--flip` | *(none)* | Flips output vertically (upside-down inversion, swapping vertical symmetric characters like `_`, `^`, etc.). |
| `-s` | `--strip-surrounding-newlines` | *(none)* | Strips leading and trailing empty lines from rendered output. Takes precedence over `-n`. |
| `-n` | `--normalize-surrounding-newlines` | *(none)* | Normalizes surrounding newlines to exactly one blank line before and after. Ignored when `-s` is active. |

- **Argument Parsing Support**:
  - Single-hyphen clustered boolean flags (e.g., `-rF`, `-sF`, `-rn`).
  - Flag termination delimiter (`--`) to mark the end of options and beginning of text arguments.
  - All non-flag arguments are joined with spaces as the input text string.
