# figlet-go: a Harbor task

This [Harbor](https://harborframework.com) task asks an AI agent to **port
[pyfiglet](https://github.com/pwaller/pyfiglet) to Go**. pyfiglet is MIT licensed; the task pins commit
`255827efbc09d662875dad50fa0edc1e1263bf45` from 2026-08-02. The Go program must reproduce pyfiglet's
command-line output byte for byte for `-f -w -j -D -r -F -s -n`. It must use only the Go standard library,
and the agent's room contains no Python.

## Results

| Run | Behaviour | Build | No_Python | **Reward** | Target | Secret | Job |
|-----|-----------|-------|-----------|------------|--------|--------|-----|
| oracle (`solution/solve.sh`) | 1.000 | 1.000 | 1.000 | **1.000** | 1.0 | unlocked | `jobs/2026-11-16__18-55-49` |
| nop (agent does nothing) | 0.000 | 0.000 | 0.000 | **0.000** | 0.0 | no | `jobs/2026-11-16__19-08-09` |
| cheat (`cheat/solve.sh`) | 0.000 | 1.000 | 0.000 | **0.167** | < 0.2 | no | `jobs/2026-11-17__00-04-37` |

Earlier runs with the same results: `18-38-39` (oracle), `18-39-57` (nop), `18-40-43` (cheat).
The `jobs/` folders are Harbor's local run output and are not pushed (`.gitignore`). The job IDs above
match the paths shown in the screenshots.

The screenshots in [`screenshots/`](screenshots/) are real terminal captures:
- `oracle-reward.png` and `oracle-secret.png`: the verifier log with the hidden secret drawn by the twin;
- `nop-reward.png`;
- `cheat-reward.png`;
- `page-secret.png`: the page's own secret.

**Page secret:** inside the built image with the oracle's Go code,
`/tmp/figlet -f slant "$(echo Z28gdG91Y2ggZ3Jhc3M= | base64 -d)"` draws **"go touch grass"** cleanly.
This output is byte-identical to pyfiglet.

## Hand-in: two honest lines

- **Smushing rule that caused the most trouble:** none failed in practice. The oracle matched 2000/2000 on its
  first build because `FigletBuilder` was ported line by line rather than from the FIGlet spec. The rule that
  needed the most care was **right-to-left**. Both `smushAmount` and `smushRow` swap the new glyph to the left,
  and `-j auto` then right-justifies with `width − len − 1` spaces. A close second: the first glyph on each line
  loses its leading blank columns.
- **Docker trap:** `golang:1.23-alpine` has **no bash** (Harbor would fail with
  `exec: "bash": executable file not found`) and no git. A probe caught both before the first build. The fix was
  `apk add bash` plus a separate `alpine/git` stage 1. The bigger trap was Harbor's shared verifier, which runs
  `test.sh` as the image's `USER` (`player`). The agent's twin could then read `/tests/data/hidden.jsonl` and
  score 1.0 (tested). The fix: run the verifier as root and the twin as `player`.

## Layout

```
KeshavKumar_pyfiglet/
├── pyfiglet/                 NOT pushed (.gitignore): local clone of the original at the pinned commit
├── figlet-go/                the Harbor task
│   ├── instruction.md        what the agent is told
│   ├── NOTES.md              study notes: SHA, licence, font setup, 6 smush rules + kerning, options, 13 rare cases
│   ├── task.toml             agent user = player; shared verifier running as root
│   ├── environment/Dockerfile  2 stages: alpine/git (clone + Makefile font setup) -> golang:1.23-alpine
│   ├── solution/             oracle: solve.sh + figlet/*.go (Go port, standard library only)
│   ├── cheat/                cheat: solve.sh + visible.jsonl (looks answers up, ports nothing)
│   └── tests/                test.sh, grade.go (Go grader), secret.b64, gen.py, data/{visible,hidden}.jsonl
├── screenshots/
└── jobs/                     NOT pushed (.gitignore): Harbor's local run results
```

## The room (`environment/Dockerfile`)

| Rule | How it is met | Checked with |
|------|---------------|--------------|
| D1 two stages | `alpine/git` clones and prepares `/src`; only `/src` is copied into stage 2 | `command -v git` → none |
| D2 no Python | `golang:1.23-alpine` (bookworm ships `/usr/bin/python3`) | `command -v python3` → `NO_PYTHON` |
| D3 font setup | Makefile `clean` + `minimal`: `cp fonts-standard/* fonts/` | 72 fonts in `pyfiglet/fonts` |
| D4 non-root | user `player` (uid 1000) owns `/workspace/repo` | `whoami`; `apk add python3` fails |
| D5 pinned | both `FROM` lines pinned by `@sha256:` digest; the commit is checked after checkout | |
| D6 bash | `apk add --no-cache bash` | `command -v bash` → `/bin/bash` |

## Grading (`tests/grade.go`, standard library only)

| Check | Weight | Scores 1 when |
|-------|--------|---------------|
| build | 1 | `go build -o /tmp/figlet .` and `go build ./...` pass |
| no_python | 1 | the build passes, there is no `.py` under `/workspace/repo`, **and** at least one hidden case is right |
| behaviour | 4 | share of the 2000 **hidden** cases whose stdout matches exactly (5 s timeout each) |

`reward = (build + no_python + 4 × behaviour) / 6` is written to `/logs/verifier/reward.json`.
If behaviour is exactly 1.0, the twin draws the secret (`tests/secret.b64`) in slant after
`*** SECRET UNLOCKED ***`. Otherwise the grader prints `No secret for you. Yet.`

Anti-cheat design:
- Only `hidden.jsonl` is graded. It never overlaps the visible set.
- `gen.py` drops cases where pyfiglet crashes, hangs or prints only whitespace. Otherwise a program
  that prints nothing, or only a newline, would match some cases.
- `no_python` depends on real behaviour, so deleting the `.py` files is not enough.
- The verifier runs as **root**. `/tests` is locked to root only, and the twin runs as `player`, so it
  cannot read the answers or the secret.
- `test.sh` uses `/usr/local/go/bin/go` by absolute path, because `/go/bin` is world-writable and first on
  PATH. It also sets `GOENV=off` (ignores `go env -w GOFLAGS=-toolexec…`), a fresh GOCACHE/GOPATH and
  `GOPROXY=off` (which enforces the standard-library-only rule).
- All `player` processes are killed before grading and again before writing the reward.
- In local tests an attacker that combined all of these tricks scored 0.167. With the verifier running as
  `player`, as the original task page set it up, the same attacker scored 1.0.

## Test data (`tests/gen.py`)

`random.seed(7)` produces 2000 visible and 2000 hidden cases, all different. They use 8 fonts, widths
10–120, every justify and direction value, and `-r -F -s -n` each in about 1 of 4 cases. The texts include
smush characters, rare cases from NOTES.md, Latin-1, missing glyphs, newlines and multi-argument text.
Every `out` is the real pyfiglet `main()` stdout. Two runs give byte-identical files, and all 4000 answers
were re-checked by calling `main()` in-process.

## How to run

From `KeshavKumar_pyfiglet/`, in PowerShell, with Docker running:

```powershell
$env:PYTHONUTF8 = "1"; $env:PYTHONIOENCODING = "utf-8"   # Windows: Harbor otherwise crashes printing
harbor lint figlet-go          # expected warning only: shared-verifier
harbor run -p .\figlet-go -a oracle
harbor run -p .\figlet-go -a nop
```

To run the cheat:
1. Back up `solution\solve.sh`.
2. Copy `cheat\solve.sh` **and** `cheat\visible.jsonl` into `solution\`.
3. Run the oracle command.
4. **Restore the real `solve.sh`** and delete `solution\visible.jsonl`.

To regenerate the test data, you need the original pyfiglet next to `figlet-go/`. It is not in this
repository, because it is someone else's project and can be re-created exactly. Clone it at the pinned
commit and do its font setup (the Makefile step from NOTES.md):

```powershell
git clone https://github.com/pwaller/pyfiglet
git -C pyfiglet checkout 255827efbc09d662875dad50fa0edc1e1263bf45
New-Item -ItemType Directory -Force pyfiglet\pyfiglet\fonts | Out-Null
Copy-Item pyfiglet\pyfiglet\fonts-standard\* pyfiglet\pyfiglet\fonts\
python figlet-go\tests\gen.py
```

The Harbor task itself does not need this: `environment/Dockerfile` fetches pyfiglet from GitHub on its own.
