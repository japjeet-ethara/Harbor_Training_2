# FIGlet-Go: A Go Port of pyfiglet

## 1. What This Task Is

You must write a complete Go port of pyfiglet's FIGlet rendering engine. The reference implementation is [pyfiglet](https://github.com/pwaller/pyfiglet) at commit `255827efbc09d662875dad50fa0edc1e1263bf45`. The source code of this reference is available in the Docker image at `/workspace/repo/pyfiglet/`.

Your Go program must produce **byte-for-byte identical output** to the original pyfiglet for every input string and every option combination. There is no room for "close enough" — the output must match exactly, including whitespace, newlines, and character placement.

## 2. The Go Executable Requirements

- The program **must be written in Go using the standard library only**. No third-party packages are allowed. Your `go.mod` must not contain any `require` directives.
- It must build cleanly with:
  ```
  go build ./...
  ```
- The compiled binary must be called **`figlet`** and must accept command-line arguments exactly as pyfiglet does (see Section 3).
- **No `.py` files may exist anywhere** under `/workspace/repo` when grading runs. You must remove all Python files from the repository before grading.
- The fonts the program needs must be **stored inside the repository** (already present in the Docker image). Fonts must not be fetched from the network at runtime.

## 3. The Eight Options the Program Must Support

The program must support the following eight command-line options, matching pyfiglet's behaviour exactly:

| Flag | Argument | Description | Default |
|------|----------|-------------|---------|
| `-f` | `<fontname>` | Font to use for rendering. | `standard` |
| `-w` | `<width>` | Output width in characters. | `80` |
| `-j` | `<justify>` | Justification: `left`, `right`, or `center`. | Depends on direction (`left` for left-to-right, `right` for right-to-left). |
| `-D` | *(none)* | Set print direction to right-to-left. | Off (left-to-right). |
| `-r` | *(none)* | Reverse each line of output left-to-right. | Off. |
| `-F` | *(none)* | Flip the output upside down. | Off. |
| `-s` | *(none)* | Enable smush mode (smush characters together according to the font's smushing rules). | Off. |
| `-n` | *(none)* | Render each line of standard input as a separate FIGlet output. | Off (all input lines are concatenated into one). |

Any text remaining on the command line after the flags is treated as the input string. If no positional text is given, the program reads from standard input.

## 4. Output Rules

- Output must be **byte-for-byte identical** to pyfiglet for every test case. This includes trailing spaces, trailing newlines, and line ordering.
- Fonts come **only from `pyfiglet/fonts-standard/`** — these are the font files already present in the Docker image at `/workspace/repo/pyfiglet/fonts/`.
- The program must work correctly **from any working directory**. It must locate its font files relative to its own source or binary location, or via a known absolute path within the Docker image — not by assuming the current directory contains the font folder.

## 5. Grading and Scoring

- Grading is **automatic and hidden**. You will not see the test harness or its internals.
- Three grading components with the following weights:

  | Component | Weight | What it checks |
  |-----------|--------|----------------|
  | `build` | 1 | `go build ./...` succeeds and produces a `figlet` binary. |
  | `no_python` | 1 | No `.py` files exist anywhere under `/workspace/repo`. |
  | `behaviour` | 4 | The `figlet` binary produces byte-for-byte correct output across all test cases. |

- **Partial scoring applies.** A program that builds and has no Python files but renders nothing correctly scores 2/6 (≈ 0.33). A perfect solution scores **1.0**.
- A secret message is revealed **only when the `behaviour` component scores exactly 1.0** (all test cases pass).
- **Do not** disclose test case contents, expected answers, or the secret in any output or committed file.

## 6. Explicit Exclusions

- **Do not** use any Python interpreter or pyfiglet Python code at runtime. The Go binary must be entirely self-contained.
- **Do not** use any third-party Go packages. Standard library only.
- **Do not** fetch fonts or any other file from the internet at runtime. All required data must already be present in the repository.
- **Do not** read from `/workspace/repo/pyfiglet/__init__.py` at runtime. The Python source is a reference for your implementation, not a runtime dependency.
- The following pyfiglet options are **out of scope** and must not be implemented: colour options (`-c`), font install (`-L`), font list (`-l`), and font info (`-i`).
