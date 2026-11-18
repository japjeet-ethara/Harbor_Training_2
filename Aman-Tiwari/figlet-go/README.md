# practice/figlet-go

Harbor practice task 2: port **pyfiglet** (commit `255827efbc09d662875dad50fa0edc1e1263bf45`, MIT)
to Go, byte for byte, and grade the port automatically.

## Layout

| path | what |
|---|---|
| `NOTES.md` | Step 1: SHA, date, licence, font setup, the six smushing rules (verified), options, rare cases |
| `environment/Dockerfile` | Step 2: two stages (`alpine/git` → `golang:1.23-alpine`), pinned by digest, no Python, `bash`, user `player` |
| `task.toml` | agent user `player`, 3600 s; public network; build 900 s; verifier 900 s, shared |
| `instruction.md` | Step 3: what "done" means |
| `tests/gen.py` | Step 4: seeded (7) generator, run on the laptop against the original pyfiglet |
| `tests/data/{visible,hidden}.jsonl` | 2000 + 2000 frozen cases (no overlap, no empty outputs) |
| `tests/grade.go`, `tests/test.sh` | Step 5: Go grader → `/logs/verifier/reward.json` |
| `tests/secret.b64` | Step 5: secret drawn by the agent's own binary when behaviour = 1.0 |
| `solution/solve.sh`, `solution/src/*.go` | Step 6: oracle (Go port, fonts embedded with `go:embed`) |
| `cheat/solve.sh` | Step 7: lookup-table cheat over `visible.jsonl` |
| `screenshot/{oracle,nop,cheat}.png` | Step 8: real terminal captures of the three Harbor runs |

## Regenerating the answers

```
# next to this folder: git clone pyfiglet, checkout the SHA, then
mkdir -p pyfiglet/pyfiglet/fonts && cp pyfiglet/pyfiglet/fonts-standard/* pyfiglet/pyfiglet/fonts/
python figlet-go/tests/gen.py          # identical output on every run
```

## Results

| agent | build | no_python | behaviour | reward |
|---|---|---|---|---|
| oracle | 1 | 1 | 1.000 | **1.000** (secret drawn) |
| nop | 0 | 0 | 0.000 | **0.000** |
| cheat | 1 | 0 | 0.000 | **0.167** ("No secret for you. Yet.") |

## Two lines

- Hardest smushing rule: Big X: the practice page says `/` + `\` → `X`, but pyfiglet gives `|` (and `\` + `/` → `Y`; only `>` + `<` → `X`). Right-to-left was a close second, because the buffer is glued to the right of each new character and universal smushing then keeps the *left* character.
- Docker trap: the trap I planned around was `golang:1.23-bookworm`, which ships `python3`, so I used Alpine + `apk add bash`. The traps I actually hit were on the Windows host: Git Bash rewrites `/tmp/...` paths passed to `docker run -e` (fixed with `MSYS_NO_PATHCONV=1`), and Harbor crashes printing `✓` in a cp1252 console (fixed with `PYTHONIOENCODING=utf-8`).
