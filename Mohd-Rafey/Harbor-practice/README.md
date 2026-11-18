# Mohd Rafey: figlet-go (Harbor practice task 2)

Port pyfiglet (https://github.com/pwaller/pyfiglet, commit
`255827efbc09d662875dad50fa0edc1e1263bf45`, 2026-08-02, MIT) to Go, byte for byte, in a
two-stage Alpine room that contains no Python. My notes on pyfiglet (licence, font setup,
the six smushing rules plus kerning, the eight options, rare cases) are in
[pyfiglet/NOTES.md](pyfiglet/NOTES.md).

## Results

| Run | Harbor result | Screenshot |
|---|---|---|
| `harbor run -p ./Harbor-practice -a oracle` | **1.000** (behaviour 1.000, build 1.000, no_python 1.000, secret drawn) | [screenshots/oracle.png](screenshots/oracle.png) |
| `harbor run -p ./Harbor-practice -a nop` | **0.000** ("No secret for you. Yet.") | [screenshots/nop.png](screenshots/nop.png) |
| oracle with `cheat/solve.sh` swapped in | **0.167** (build 1.000, behaviour 0.000, no_python 0.000, "No secret for you. Yet.") | [screenshots/cheat.png](screenshots/cheat.png) |

All three are real Harbor 0.25.0 runs (jobs `2026-11-18__07-19-24`, `07-21-06` and `07-22-50`), captured from the
console window they ran in. On my Windows PC, `harbor.exe` is blocked by an Application Control policy,
so Harbor ran in a small Linux container (`python:3.12-slim` + Docker CLI + `uv tool install harbor==0.25.0`)
that drives Docker Desktop through `/var/run/docker.sock`. The working folder is mounted at
`/run/desktop/mnt/host/c/...` so Harbor's bind mounts resolve to the same host folder. A second cheat that
always prints `"\n"` also scores 0.167.

Reward = (build × 1 + no_python × 1 + behaviour × 4) / 6.

| Scenario | build | no_python | behaviour | reward |
|---|---|---|---|---|
| nothing done | 0 | 0 | 0 | 0 |
| builds only | 1 | 0 | 0 | 1/6 ≈ 0.17 |
| perfect | 1 | 1 | 1 | 1.0 |

The secret (`tests/secret.b64`), drawn by the oracle's port, is identical to what the original pyfiglet draws:

```
          _                               __
   ____  (_)_______     ____  ____  _____/ /_
  / __ \/ / ___/ _ \   / __ \/ __ \/ ___/ __/
 / / / / / /__/  __/  / /_/ / /_/ / /  / /__
/_/ /_/_/\___/\___/  / .___/\____/_/   \__( )
                    /_/                   |/
    __
   / /_  __  ______ ___  ____ _____
  / __ \/ / / / __ `__ \/ __ `/ __ \
 / / / / /_/ / / / / / / /_/ / / / /
/_/ /_/\__,_/_/ /_/ /_/\__,_/_/ /_/
```

The page's hidden message (`Z28gdG91Y2ggZ3Jhc3M=` decodes to "go touch grass") also matches pyfiglet byte for byte.

## What is where

| Path | What it is |
|---|---|
| `environment/Dockerfile` | Stage 1 (`alpine/git`, pinned by digest) clones pyfiglet at the commit, deletes `.git`, `fonts-contrib` and `test-fonts`, then does the Makefile font setup. Stage 2 (`golang:1.23-alpine`, pinned by digest) adds bash and user `player` (uid 1000), sets `CGO_ENABLED=0` and `GOTOOLCHAIN=local`. No Python in the final image. |
| `task.toml` | agent user `player`, 3600 s; network `public`, build 900 s; verifier 900 s, shared. |
| `instruction.md` | What the agent must build. Every sentence can be checked. |
| `tests/grade.go`, `tests/test.sh` | Go grader (standard library only). build (1), no_python (1, only if the build passed, no `.py` files remain and at least one hidden case matches), behaviour (4, share of `tests/data/hidden.jsonl` matched byte for byte, 5 s per case). Writes `/logs/verifier/reward.json` and prints the secret only at behaviour 1.0. |
| `tests/gen.py` | Makes `tests/data/visible.jsonl` and `hidden.jsonl` (2000 each, `random.seed(7)`, no shared args) by running the original `pyfiglet.main()`. No `out` is written by hand. |
| `tests/secret.b64` | My message for whoever solves the task. |
| `solution/solve.sh` | Oracle: a hand-written Go port of `FigletBuilder` and the font loader, fonts embedded with `go:embed`, every `.py` deleted. |
| `cheat/solve.sh` | Cheat for the cheat run: looks up its args in a copy of `visible.jsonl`, ports nothing. |

## Bugs I found in my own exam

1. **pyfiglet loops forever on some inputs** (e.g. `-f slant -w 10 -r Harbor`: `handleNewLine`
   never makes progress when a character cannot fit on an empty line). The first generator run
   hung. `gen.py` now stops each case after a fixed budget of 3,000,000 executed Python lines.
   It counts lines, not seconds, so the same cases are skipped every run. The largest valid case
   needs about 73,000.
2. **Blank answers.** Texts like `é` or `€` alone draw nothing. The first data set had 34 blank
   hidden answers (17 of them exactly `"\n"`). A cheat printing only a newline would have
   matched them and unlocked no_python, for about 0.34. `gen.py` now skips blank drawings (71
   skipped), and that cheat scores 0.167.

## Regenerating the data

`gen.py` needs the original pyfiglet, with its font setup done, cloned next to this folder:

```sh
git clone https://github.com/pwaller/pyfiglet ../pyfiglet
git -C ../pyfiglet checkout 255827efbc09d662875dad50fa0edc1e1263bf45
mkdir -p ../pyfiglet/pyfiglet/fonts && cp ../pyfiglet/pyfiglet/fonts-standard/* ../pyfiglet/pyfiglet/fonts/
python tests/gen.py          # writes the same files every time
```

(`pyfiglet/NOTES.md` in this folder holds only my notes; the clone itself is not committed.)

## Hand-in lines

**Smushing rule that broke the agent most:** none broke it. The oracle port matched 2000/2000
hidden and 2000/2000 visible cases on its first run, and no AI agent has attempted the task yet.
The rule most likely to trip an agent is rule 16 (big X): the code comment says `/` + `\` → `X`,
but the code returns `|` (and `\` + `/` → `Y`).

**Docker trap I hit:** Git Bash on Windows rewrote container paths in `docker exec`
(`/tests` became `C:/Program Files/Git/tests`), fixed with `MSYS_NO_PATHCONV=1`. Running Harbor
from a container also needed the working folder mounted at Docker Desktop's own host path
(`/run/desktop/mnt/host/c/...`), otherwise Harbor's log bind mounts would point at empty folders. The planned traps
(Python inside `golang:1.23-bookworm`, Alpine without bash, a build context that cannot see the
clone) were avoided by design: `golang:1.23-alpine`, `apk add bash`, and stage 1 cloning from GitHub.
