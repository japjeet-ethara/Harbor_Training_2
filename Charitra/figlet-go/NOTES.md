# pyfiglet research notes

- Upstream: https://github.com/pwaller/pyfiglet
- Required commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: **2026-08-02T22:51:18+01:00** (from `git log -1 --format='%H %cI'`).
- Licence: MIT (confirmed in the source's embedded copyright notice and setup metadata).
- Font preparation: upstream fonts are stored in `pyfiglet/fonts-standard/`, while runtime font loading expects `pyfiglet/fonts/`. The setup step is `mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/` (the project's setup/build configuration performs the equivalent copy).

## Six horizontal smushing rules

1. **Equal character (1):** identical non-hardblank characters collapse: `||` → `|`.
2. **Underscore (2):** `_` yields to one of `| / \ [ ] { } ( ) < >`: `_/` → `/`.
3. **Hierarchy (4):** the later/higher member of `|`, `/\\`, `[]`, `{}`, `()`, `<>` wins: `|[` → `[`, `[]` → `]`.
4. **Opposite pair (8):** opposing brackets/braces/parentheses become `|`: `[]` or `][` → `|`.
5. **Big X (16):** `/\\` → `|`, `\\/` → `Y`, `><` → `X`; `<>` is not affected.
6. **Hardblank (32):** two hardblanks collapse to one hardblank; a single hardblank cannot be smushed by the controlled rules.

**Kerning (64)** is fitting only: characters move together until their visible edges touch, without overwriting either character. Universal smushing (128 with no controlled rule bits) lets the later character overwrite the earlier one, except visible characters take precedence over hardblanks.

## CLI options in scope

- `-f FONT`: select a FIGfont.
- `-w COLS`: terminal width and wrapping/justification width (default 80).
- `-j auto|left|center|right`: line justification; `auto` follows direction.
- `-D auto|left-to-right|right-to-left`: print direction; `auto` follows the font header.
- `-r`: mirror the rendered output.
- `-F`: flip the rendered output vertically with FIGlet's character translation map.
- `-s`: strip surrounding blank lines.
- `-n`: normalize to one blank line before and after (takes precedence only when `-s` is absent).

The task excludes colour (`-c`), font installation (`-L`), font listing (`-l`) and font information (`-i`).
