# Upstream behavior at the pinned revision

The reference is [pwaller/pyfiglet](https://github.com/pwaller/pyfiglet), commit [255827efbc09d662875dad50fa0edc1e1263bf45](https://github.com/pwaller/pyfiglet/commit/255827efbc09d662875dad50fa0edc1e1263bf45), with Git committer date 2026-08-02 22:51:18 +01:00. The project is MIT-licensed. Its notice credits Christopher Jones, Stefano Rivera, Peter Waller, and other contributors. The original notice is in `LICENSE.pyfiglet`. The image retains the upstream source tree's license and the individual font headers and notices. The Go port follows the original builder and CLI semantics.

The checkout contains `pyfiglet/fonts-standard/`. The loader first looks for packaged fonts in `pyfiglet/fonts/`. The Makefile's `minimal` target runs `clean` to create that directory and copies `pyfiglet/fonts-standard/*` into it. `setup.py` also copies the standard directory if the package directory is absent. The image runs the copy step directly. Python packaging and external Python libraries are unnecessary for the host reference.

## Horizontal layout

The full-layout bits take precedence over the old layout value. Old layout 0 selects kerning. A negative old layout selects full width. A positive old layout selects smushing with its low five bits. Bits 64 and 128 select kerning and smushing respectively, and bits 1 through 32 select collision rules. Compute one permitted overlap per glyph by taking the minimum across its art rows. Ordinary leading and trailing spaces can overlap. Widths and cursor positions count code points, following the Python strings.

| Rule | Bit | Checked example |
| --- | ---: | --- |
| Equal | 1 | Two pipes merge into one pipe. Hardblanks are excluded. |
| Lowline | 2 | An underscore and a slash yield a slash. Either order works for any member of the hierarchy. |
| Hierarchy | 4 | A pipe and a closing square bracket yield the bracket. The ordered classes are pipe, slashes, square brackets, braces, parentheses, and angle brackets. The higher class survives. |
| Opposite pair | 8 | A closing brace and opening brace yield a pipe. Either order works for square brackets, braces, and parentheses. |
| Large X | 16 | `/` then `\` yields a pipe. `\` then `/` yields `Y`. `>` then `<` yields `X`. `<` then `>` cannot overlap. |
| Hardblank | 32 | Two hardblank cells yield one hardblank when enabled. A hardblank stays distinct from ordinary space until layout is complete. |
| Kerning | 64 | A space and pipe yield a pipe. Two pipes cannot overlap in kerning-only mode. |

`validation/reference-notes.py` ran these exact tiny examples using the original `smushChars`; its output and the reference slant smoke test are in `evidence/reference-runtime.log`. Universal smushing applies when bit 128 is set and all six rule bits are clear. Visible cells take precedence over hardblanks, and the later input glyph takes precedence when both are visible. Non-space collisions require both glyph widths to be at least two.

## CLI and line breaks

| Option | Actual behavior |
| --- | --- |
| `-f` | Font name. Default `standard`. |
| `-w` | Width for wrapping and justification, default 80. An insertion reaching the width boundary triggers wrapping. |
| `-j` | `auto`, `left`, `center`, or `right`. Auto selects left for left-to-right, right for right-to-left. Padding is computed per art row. Right justification adds width minus row length minus one spaces to the left. Centering adds half the difference, truncated toward zero. |
| `-D` | `auto`, `left-to-right`, or `right-to-left`. Auto uses the FLF direction bit and otherwise falls back to left-to-right. Right-to-left inserts the next glyph on the left and uses that visual order for directional collision rules. |
| `-r` | Mirrors each row and maps asymmetric characters. |
| `-F` | Flips the order of the rows and maps vertically asymmetric characters. |
| `-s` | Skips leading whitespace-only rows, then trims trailing whitespace from the result. |
| `-n` | Uses the same stripping operation and adds surrounding line feeds. |

The CLI joins positional arguments with a single space. Flag order can vary. The last repeated value option wins. Transformations always run mirror, flip, then strip or normalize. If `-s` and `-n` both occur, stripping wins. The CLI appends one final LF even when rendering produces no characters. Unsupported glyphs are skipped. Explicit newlines flush the current row buffers immediately. A wrap at a saved space restores the rows before that space and reprocesses the remaining input. With no saved space, wrapping flushes the current rows and retries the glyph. A glyph wider than the width raises in the reference. A non-space glyph exactly as wide as a fresh line can prevent reference progress, so the generator screens such cases and the Go port returns an error if it encounters that stalled boundary.

The font reader accepts the original FLF and TLF signatures, skips header-defined comments, and discovers an endmark per glyph. It loads the ASCII glyphs, the seven fixed-position German glyphs when present, and hexadecimal tagged glyphs. The original loader skips decimal codes at this revision. Some Latin-1 characters consequently render only in fonts with the corresponding glyph. The paired unsupported-character cases retain supported text nearby so this distinction affects the expected output.
