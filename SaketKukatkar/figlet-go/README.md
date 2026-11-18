# Harbor Practice Task 2: FIGlet in Go

This task asks an agent to replace the pinned pyfiglet implementation with a
Go command-line program that preserves its rendering behavior and uses the
provided standard fonts.

The pinned source and commit metadata are recorded in [NOTES.md](NOTES.md).
The agent-facing requirements are in [instruction.md](instruction.md). The
task image is built from two pinned stages in `environment/Dockerfile`; dataset
generation and grading utilities are under `tests/`.

The source checkout used for reference-data generation is expected beside this
task directory as `pyfiglet/`. Run `tests/gen.py` with Python after preparing
its standard fonts as described in `NOTES.md`.

## Reward arithmetic

The grader computes `(build + no_python + 4 * behaviour) / 6`:

- Nothing implemented: `0`.
- Build only: `1/6` (approximately `0.166667`).
- Perfect build, Python removal, and behavior: `1`.

## Local verification

- The pinned two-stage image built locally and passed the non-root, writable
  workspace, standard-font, Bash, and no-Python checks.
- Harbor local oracle: `1.0` with `2000/2000` hidden outputs; the banner
  unlocked. Nop: `0.0`. Visible-answer cheat: `0.16666666666666666`, with no
  hidden matches and no banner.
- No Harbor job was uploaded. Terminal screenshot files are not included.
