# NOTES - pyfiglet study notes (Harbor Practice Task 2)
- Repository: https://github.com/pwaller/pyfiglet
- Commit SHA: 255827efbc09d662875dad50fa0edc1e1263bf45
- Commit date: 2026-08-02T22:51:18+01:00
- License: MIT (see LICENSE)

## Font setup
mkdir -p pyfiglet/fonts
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
cp pyfiglet/fonts-contrib/shadow.flf pyfiglet/fonts/shadow.flf
(The shadow font comes from fonts-contrib; the Dockerfile does the same.)

## Files studied
pyfiglet/__init__.py (FigletFont, FigletBuilder, FigletString), Makefile, setup.py, LICENSE, pyfiglet/fonts-standard/

## Smushing rules (smushMode bits) and kerning
1 equal character | 2 underscore | 4 hierarchy | 8 opposite pair | 16 big X | 32 hardblank | 64 kerning | 128 smush
| 1 | Equal character | two identical (non-hardblank) characters become one |
| 2 | Underscore | `_` is replaced by any of `\| / \ [ ] { } ( ) < >` |
| 4 | Hierarchy | class order `\|`, `/ \`, `[ ]`, `{ }`, `( )`, `< >` – the higher class wins |
| 8 | Opposite pair | `[]`, `{}`, `()` (either order) become `\|` |
| 16 | Big X | `/\` → `\|`, `\/` → `Y`, `><` → `X` |
| 32 | Hardblank | two hardblanks become one hardblank |
| 64 | Kerning | glyphs are pushed together until they touch (no overlap) |
| 128 | Smush | glyphs may overlap by one column using the rules above (universal overlap if bits 1-32 are all 0) |

## Command-line options required
-f font, -w width, -j justify, -D direction, -r reverse (mirror), -F flip, -s smushing (font default), -n no smushing. If both -s and -n appear, the last one wins.

## Data note
tests/data/visible.jsonl is copied into the agent image for debugging only; hidden.jsonl is never in the image.
