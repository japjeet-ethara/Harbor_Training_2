# practice/figlet-go

Harbor task: port pyfiglet (`255827ef…`, MIT) to Go, byte for byte, in a two-stage
Alpine room with no Python, as the non-root user `player`.

| Path | What |
|---|---|
| `NOTES.md` | Step 1: SHA, date, licence, font setup, smushing rules, options, rare cases |
| `environment/Dockerfile` | Step 2: stage `source` (git clone + Makefile font step), stage 2 room |
| `task.toml` | agent user `player`, 3600 s; public network; build 900 s; verifier 900 s, shared |
| `instruction.md` | Step 3: what the agent must build |
| `tests/gen.py` | Step 4: seeded case generator, answers from the original `main()` only |
| `tests/data/{visible,hidden}.jsonl` | 2000 + 2000 frozen cases, no overlap |
| `tests/grade.go`, `tests/test.sh` | Step 5: Go grader → `/logs/verifier/reward.json` |
| `tests/secret.b64` | secret drawn by the twin when behaviour = 1.0 |
| `solution/solve.sh` | Step 6: the oracle (full Go port, stdlib only, fonts embedded) |
| `cheat/solve.sh` | Step 7: lookup-table cheat on a copy of visible.jsonl |
| `run_checks.sh` | runs every check on your machine (Docker + Harbor) |

Rewards: build 1, no_python 1 (needs build + no `.py` + at least one right hidden case),
behaviour 4 → `reward = (build + no_python + 4·behaviour) / 6`.

## Results

| Run | reward | build | no_python | behaviour | secret |
|---|---|---|---|---|---|
| oracle | **1.0** | 1 | 1 | 2000/2000 | drawn |
| nop | **0.0** | 0 | 0 | 0 | no |
| cheat | **0.167** | 1 | 0 | 0/2000 (2000/2000 on visible) | no |
| build only (sanity) | 0.167 | 1 | 0 | 0 | no |

The oracle port also matches pyfiglet on 1470 fuzz cases across all 67 standard `.flf` fonts.

## Two lines

* Smushing rule that bites: none of the six rules alone. It is how they combine with right-to-left, where
  pyfiglet builds the line leftwards, universal smushing keeps the *left* char, and
  `prevCharWidth` keeps the width of the glyph that caused a wrap. Also, the Big-X rule gives
  `\`+`/` → `Y`, not `X`.
* Docker trap: Alpine has no `bash` (Harbor needs it), and `golang:*-bookworm` ships `python3`.
