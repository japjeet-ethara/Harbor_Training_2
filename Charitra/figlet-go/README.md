# Harbor Practice 2 — FIGlet Go port

This folder is a task scaffold assembled from the supplied Practice Task 2 instructions and Day 3 Harbor guide.

## Folder layout

```text
figlet-go/
├── task.toml
├── instruction.md
├── NOTES.md
├── environment/
│   └── Dockerfile
├── solution/
│   ├── solve.sh
│   ├── main.go
│   └── go.mod
├── cheat/
│   └── solve.sh
├── screenshot/
│   ├── Screenshot 2026-11-16 154714.png
│   ├── Screenshot 2026-11-16 154726.png
│   └── Screenshot 2026-11-16 155017.png
├── tests/
│   ├── gen.py
│   ├── grade.go
│   ├── test.sh
│   ├── secret.b64
│   └── data/
│       ├── visible.jsonl
│       └── hidden.jsonl
├── docs/
└── jobs/  # local Harbor run logs; ignored by Git
```

The Go source and module live in `solution/`, where the Harbor Oracle script copies them from. `docs/` is retained for supporting materials; `jobs/` holds local validation logs.

## Required local setup

- Docker Engine/Desktop
- Harbor 0.25 compatible installation
- Git and network access to GitHub (the Docker build fetches the pinned pyfiglet commit)
- Python 3 on the host for the reference-case generator
- Go 1.23 (included in the task image)

## Workflow

1. `git clone https://github.com/pwaller/pyfiglet ../pyfiglet`
2. `cd ../pyfiglet && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 && git log -1 --format='%H %cI'`
3. Record the exact commit date in `NOTES.md`; verify the MIT licence and run `python -m pyfiglet -f slant hello` after the font setup.
4. From this folder, build the image: `docker build -t figlet-room environment/`
5. Run the six image checks from the practice instructions.
6. Generate reference data: `python tests/gen.py --pyfiglet-dir ../pyfiglet`
7. Build the Go oracle using the source in `solution/`; compare its output to both visible and hidden reference cases. Fix all mismatches before running Harbor.
8. For the cheat check, copy `tests/data/visible.jsonl` to `solution/visible.jsonl`, then run the cheat version as directed by the handout.
9. Run `harbor run -p . -a oracle`, then `harbor run -p . -a nop`, then the visible-set cheat as directed by the handout.

## Validation results

All requested local checks passed against pyfiglet commit `255827efbc09d662875dad50fa0edc1e1263bf45`:

- The Docker image passed all six checks: user `player`, `/workspace/repo`, 72 prepared fonts, no `python3`, Bash present, and writable workspace.
- `go build ./...` and `go build -o /tmp/figlet .` passed in the task image; no `.py` files remained in `/workspace/repo` after either solution script.
- The 2,000 visible references matched the Go port exactly. Harbor matched all 2,000 hidden references.
- Harbor rewards: Oracle `1.0` (`build=1`, `no_python=1`, `behaviour=1`), Nop `0.0`, and visible lookup Cheat `0.166667` (`build=1`, `no_python=0`, `behaviour=0`). Only Oracle unlocked the secret.
- Both reference files contain 2,000 unique cases and have no argument overlap. A repeat generation produced byte-identical SHA-256 hashes.

Harbor lint reports the `shared-verifier` warning; the practice instructions explicitly expect this for the shared local verifier. Harbor job logs are under `jobs/` and ignored by Git. The three Harbor reward screenshots are in `screenshot/`.
