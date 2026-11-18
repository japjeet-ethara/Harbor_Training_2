# What the hidden exam can and cannot tell apart

Method: the working Go port (`dev/oracle_src`) was broken one behaviour at a time and run against
`tests/data/hidden.jsonl` (2000 cases). "Caught" = at least one case differs.

| Mutation | Hidden cases that differ |
|---|---|
| hardblank not replaced by space | 1303 |
| smushAmount without RTL swap | 745 |
| smushRow without RTL swap | 666 |
| wrap ignores blanks (always cuts at last char) | 631 |
| reverse (-r) without character map | 425 |
| flip (-F) without character map | 416 |
| universal smush: hardblank not preferred | 165 |
| equal / lowline / hierarchy / pair rule off, kern treated as smush, `>=` vs `>` wrap, center rounding, right pad, `-n` vs `-s` precedence, joined args, default width, RTL default justify, missing final newline, missing umlaut glyph block | all caught (> 0) |
| **bigx rule (`Y`/`X`/`|`) changed or removed** | **0 - unreachable with these fonts** |
| **hardblank-smush rule (32) removed** | **0 - unreachable** |
| **universal smush "later char wins" / RTL winner flipped** | **0 - unreachable** (hardblank always involved) |
| hierarchy class `{}` vs `()<>` dropped | 0 (never meets at a glyph edge) |
| lowline `_`+`_` added | 0 |
| `-r` map for `{` `}` removed; `-F` map for `R` removed | 0 (those characters never appear in the art) |
| code-tagged (`0x...`) glyph loading removed | 0 (none of the 8 fonts has any) |
| `strip()` only trimming ASCII space | 0 |

Meaning: a twin that gets those 0-rows wrong still scores 1.0. They are properties of the fonts, not of the
generator. Adding fonts outside the eight does not help either (no font in `fonts-standard/` uses them);
only `fonts-contrib/` might, and the task deletes it on purpose.
