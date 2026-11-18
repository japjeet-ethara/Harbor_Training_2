# Task: figlet-go

## 1. What the repository is

The repository at `/workspace/repo` is a prepared snapshot of **pyfiglet** — a full port of FIGlet (<https://github.com/pwaller/pyfiglet>) into pure Python, licensed under the MIT License. It takes ASCII text and renders it using ASCII-art fonts stored in `.flf` (FIGlet font) files. The project supports kerning and smushing in the same way as the original FIGlet program, with output intended to be byte-for-byte identical. Font files are located under `pyfiglet/fonts/` inside the repository (copied there from `pyfiglet/fonts-standard/` as the Makefile's `minimal` target requires). The snapshot is pinned to commit `255827efbc09d662875dad50fa0edc1e1263bf45` (2026-08-03) of <https://github.com/pwaller/pyfiglet>.

---

## 2. What to build

Write a Go program whose root package lives at `/workspace/repo`. Running the following command from that directory must succeed and produce a working binary:

```
go build -o /tmp/figlet .
```

The resulting binary must accept the same command-line options as pyfiglet, followed by the text to render as positional arguments:

| Flag | Description |
|------|-------------|
| `-f FONT` | Font name or path to use |
| `-w COLS` | Output width in columns |
| `-j auto\|left\|center\|right` | Horizontal justification |
| `-D auto\|left-to-right\|right-to-left` | Text direction |
| `-r` | Reverse the text (mirror image) |
| `-F` | Flip the output upside-down |
| `-s` | Strip surrounding blank lines |
| `-n` | Normalize surrounding blank lines (one before, one after) |

The program's output must be **byte-for-byte identical** to pyfiglet's output for the same invocation.

---

## 3. Fonts

The `.flf` font files are located under `pyfiglet/fonts/` inside the repository. You may reorganise or move them to any path you choose, **provided they remain inside the repository** (i.e., under `/workspace/repo`). The binary must locate its fonts correctly regardless of the working directory from which it is invoked — do not rely on relative paths from the current directory.

---

## 4. Hard rules

- **Go standard library only.** No third-party modules or packages may be added. The `go.mod` file must not list any external dependencies.
- **No Python files.** There must be no `.py` files anywhere in the repository tree when the grader runs.
- **`go build ./...` must pass** without errors.

---

## 5. Not checked

The following are **not evaluated** and you do not need to implement or match them:

- Execution speed or memory usage
- Error messages or exit codes on invalid input
- The options `-c`, `-L`, `-l`, and `-i`

---

## 6. Grading

Grading is performed by hidden automatic test cases that invoke the binary with various inputs and compare its output against pyfiglet's output. **Partial scores are possible** — a correct implementation for a subset of fonts or options will earn partial credit. Aim for correctness across all supported flags and the full set of fonts present in `pyfiglet/fonts/`.
