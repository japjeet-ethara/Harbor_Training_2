# Task-2: FIGlet in Go

Task-2 is a Harbor practice task to reproduce [pyfiglet](https://github.com/pwaller/pyfiglet), a Python FIGlet renderer, as a Go command-line program. The implementation is expected to match the reference renderer byte for byte, including spacing and line endings.

## Requirements

The program is built in `/workspace/repo` and supports:

- Font selection (`-f`) and output width (`-w`)
- Justification (`-j`) and rendering direction (`-D`)
- Horizontal reversal (`-r`) and vertical flipping (`-F`)
- Surrounding newline options (`-s` and `-n`)

The implementation must use only the Go standard library, keep its font assets in the repository or embed them, contain no `.py` files, and pass `go build ./...`. Color, font installation, font listing, and font information options (`-c`, `-L`, `-l`, `-i`) are out of scope.

## Environment

The Docker image uses two stages. The source stage checks out pyfiglet commit `255827efbc09d662875dad50fa0edc1e1263bf45` and prepares its standard fonts. The final stage is based on a pinned Alpine Go image and provides Bash, a non-root `player` user, and no Python runtime.

Build the image from the task directory:

```sh
docker build -t task2-room environment/
```

Check the runtime user, workspace, fonts, and required shell:

```sh
docker run --rm task2-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; command -v python3 || echo NO_PYTHON; command -v bash'
```

## Run with Harbor

With Docker and Harbor installed, run these commands from the task directory:

```sh
harbor lint
harbor run --path . --agent oracle
harbor run --path . --agent nop
```

Harbor stores run reports in `jobs/`. Treat those reports as generated output rather than task source files.

## Repository layout

```text
environment/   Dockerfile for the task image
tests/         Case generator, frozen data, and Go verifier
solution/      Oracle Go implementation and setup script
cheat/         Visible-case lookup baseline
instruction.md Agent-facing implementation requirements
NOTES.md       pyfiglet research and implementation notes
task.toml      Harbor task configuration
screenshots/   Step 8 baseline screenshots
```

The upstream pyfiglet source is available at [github.com/pwaller/pyfiglet](https://github.com/pwaller/pyfiglet). See [NOTES.md](NOTES.md) for the pinned revision, license, font setup, layout behavior, and edge cases.
