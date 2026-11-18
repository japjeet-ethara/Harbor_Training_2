# Mohammad Adnan — figlet-go (Harbor practice task 2)

Port pyfiglet (commit `255827efbc09d662875dad50fa0edc1e1263bf45`, MIT) to Go, byte for byte, in a
two-stage Alpine room with no Python. My notes on pyfiglet are in [NOTES.md](NOTES.md).

| Run | Job | Reward | Screenshot |
|---|---|---|---|
| Oracle | `2026-11-18__12-28-55` | **1.0** (behaviour 2000/2000, secret drawn) | [screenshots/oracle.png](screenshots/oracle.png) |
| Nop | `2026-11-18__12-31-51` | **0.0** | [screenshots/nop.png](screenshots/nop.png) |
| Cheat (replays visible answers) | `2026-11-18__12-30-01` | **0.167** (behaviour 0/2000, no secret) | [screenshots/cheat.png](screenshots/cheat.png) |

The screenshots are from my first runs (16 Nov). After the data fix described below I ran all three
again on 18 Nov (the jobs above) and got the same rewards.

Page secret (Step 8), drawn by my oracle's Go code: **go touch grass**.

## What is where

- `environment/Dockerfile`: stage 1 (`alpine/git`, pinned by digest) clones pyfiglet at the commit,
  deletes `.git`, `fonts-contrib` and `test-fonts`, and does the Makefile font setup
  (`fonts-standard` → `pyfiglet/fonts`). Stage 2 (`golang:1.23-alpine`, pinned by digest) adds
  `bash` and user `player` (uid 1000), who owns `/workspace/repo`. It sets `CGO_ENABLED=0` and
  `GOTOOLCHAIN=local`, and has no Python.
- `task.toml`: agent runs as `player` for 3600 s; public network; build 900 s; verifier 900 s,
  shared.
- `instruction.md`: what the agent must build.
- `tests/grade.go`, `tests/test.sh`: the Go grader, run in the shared room after the agent stops.
  - **build** (weight 1): `go build -o /tmp/figlet .`, `go build ./...`, and no `require` lines in `go.mod`.
  - **no_python** (weight 1): paid only after a hidden case passes.
  - **behaviour** (weight 4): share of `tests/data/hidden.jsonl` matched byte for byte, 5 s per case,
    run from `/`.

  `tests/secret.b64` holds my message. The grader locks `/tests` and runs the agent's binary as
  `player`, so the binary cannot read the hidden answers or the secret.
- `tests/gen.py`: makes `tests/data/visible.jsonl` and `hidden.jsonl` (2000 each, seed 7, no
  overlap) by running the original pyfiglet `main()` once per case. It skips cases where pyfiglet
  crashes, hangs (a letter exactly as wide as `-w`) or draws nothing, and hidden drawings that also
  appear in the visible file. Running it twice gives identical files.
- `tests/spotcheck.py`: compares frozen cases with a real `python -m pyfiglet` run.
- `solution/`: the oracle, a hand-ported Go version of `FigletFont`, `FigletBuilder` and
  `FigletString` (standard library only). `solve.sh` copies the Go code in from `/solution`, moves
  the `.flf` fonts into `fonts/` (embedded with `go:embed`) and deletes every `.py`. The Go code is
  never built into the image.
- `cheat/`: the cheat for the cheat run. `make_cheat.sh` builds `solve.sh`, which looks its
  arguments up in a copy of `visible.jsonl`.
- `tools/diffcases.go`: fix-loop helper that shows the first N cases where `/tmp/figlet` differs.

Grading by hand: `reward = (build + no_python + 4·behaviour) / 6`, so nothing = 0, build only = 1/6 ≈ 0.17,
perfect = 1.0.

## Bugs I found in my own exam

1. **Blank answers.** My first data set had 20 blank answers (9 hidden), because texts like `©` or `€`
   alone draw nothing. 6 hidden answers were a lone `\n`, so a cheat that only prints a newline
   would match them, unlock `no_python` and score about 0.335. `gen.py` now skips blank drawings.
2. **Same drawing, different options.** 5 hidden answers were the exact same picture as a visible
   answer under different options (for example `-j auto` and `-j left`). A cheat that ignores the
   options could replay them. `gen.py` now drops hidden cases whose drawing is in the visible file.

After both fixes: 0 blank answers, 0 shared drawings, and the cheat scores 0.167.

## Regenerating the data

`gen.py` needs the original next to this folder, with its font setup done:

```
git clone https://github.com/pwaller/pyfiglet ../pyfiglet
git -C ../pyfiglet checkout 255827efbc09d662875dad50fa0edc1e1263bf45
mkdir -p ../pyfiglet/pyfiglet/fonts && cp ../pyfiglet/pyfiglet/fonts-standard/* ../pyfiglet/pyfiglet/fonts/
uv run --no-project --python 3.12 python tests/gen.py
bash cheat/make_cheat.sh
```

## Hand-in lines

**Smushing rule that broke the agent most:** none failed the oracle; the port matched 2000/2000
visible and hidden on the first run. The rule that needed the most care was **Big X**: the task
page says `/` and `\` meet as an `X`, but pyfiglet turns `/\` into `|`, `\/` into `Y`, and only
`><` into `X`. Right-to-left in the universal-overlap fonts (`mini`, `shadow`), where the left
character wins, was the other one.

**Docker trap I hit:** `golang:1.23-bookworm` contains Python 3, so I used `golang:1.23-alpine`.
Alpine has no `bash`, so stage 2 runs `apk add bash`, or Harbor cannot start.
