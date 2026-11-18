# figlet-go — port pyfiglet to Go (Harbor task)

The agent gets the Python [pyfiglet](https://github.com/pwaller/pyfiglet)
repository (commit `255827ef`, MIT) in a Go-only container and must rewrite
it as a Go command-line program whose output matches the original
**byte for byte**: FIGfont parsing, kerning, the six smushing rules,
wrapping, justification, direction, and the `-r -F -s -n` post-processing.

## Layout

```
figlet-go/
├── .gitattributes        forces LF line endings on every checkout
├── .gitignore
├── README.md
├── instruction.md        what the agent reads
├── task.toml             agent user = player, timeouts, network, shared verifier
├── environment/
│   └── Dockerfile        alpine/git (clone + fonts) → golang:1.23-alpine + bash + user player, no Python
├── solution/             the oracle: the real Go port, scores 1.0
│   ├── solve.sh          copies the port into /workspace/repo, deletes all .py, builds
│   └── go/               go.mod (module figlet), main.go, font.go, render.go, figstring.go, pystr.go
├── tests/                the exam; the agent never sees it
│   ├── test.sh           entry point: builds and runs grade.go
│   ├── grade.go          build 1 + no_python 1 + behaviour 4 → /logs/verifier/reward.json
│   ├── secret.b64        shown on a perfect score
│   ├── gen.py            makes the case files from the ORIGINAL pyfiglet (seed 7)
│   ├── spotcheck.py      re-checks single cases against the original
│   └── data/
│       ├── visible.jsonl 2000 cases, for developing the port
│       └── hidden.jsonl  2000 different cases, used for grading
├── cheat/                looks answers up, ports nothing; scores 0.167
│   ├── solve.sh
│   └── visible.jsonl     the cheat's copied answer sheet
└── tools/                helper scripts, not used by Harbor
    ├── devloop.sh        build the port in Docker, diff against the visible cases
    ├── diffcases/main.go shows the first cases that differ
    ├── simulate.sh       nop / solution / cheat + the grader, without Harbor
    ├── make-cheat-task.sh builds ../figlet-go-cheat (solution = cheat)
    ├── run-cheat.sh      one-command cheat run through Harbor
    └── precheck.sh       pre-hand-in checks
```

The original pyfiglet checkout (`../pyfiglet`), the notes (`../NOTES.md`) and
Harbor results (`../jobs`) live beside this folder and are not part of the
task.

## Scoring

| Criterion | Weight | Passes when |
|---|---|---|
| build | 1 | `go build -o /tmp/figlet .` succeeds in `/workspace/repo` |
| no_python | 1 | build passes, no `.py` file anywhere in the repo, ≥ 1 hidden case passes |
| behaviour | 4 | fraction of the 2000 hidden cases whose stdout matches exactly (exit 0) |

`reward = (build + no_python + 4 × behaviour) / 6`. Hidden and visible cases
never share an argument list, so memorising the visible set earns nothing.

## Results

| Run | Agent | Reward |
|---|---|---|
| oracle-1, oracle-2 | oracle (`solution/`) | 1.0 (2000/2000 hidden cases) |
| nop-1, nop-2 | nop | 0.0 (no build) |
| cheat-1, cheat-2 | oracle on `../figlet-go-cheat` | 0.1667 (build only, 0/2000 hidden) |

## Commands

Run from this folder (WSL or Linux with Docker running):

```bash
harbor run -p . -a oracle -o ../jobs --job-name oracle-1   # expect 1.0
harbor run -p . -a nop    -o ../jobs --job-name nop-1      # expect 0.0
tools/run-cheat.sh cheat-1                                 # expect 0.1667

tools/simulate.sh      # all three scenarios without Harbor
tools/devloop.sh       # build solution/go and diff against the visible cases
tools/precheck.sh      # run before pushing
```

Regenerating or checking the data needs Python 3 and `../pyfiglet` on the
host (never inside the task image):

```bash
python3 tests/gen.py                        # rewrites tests/data/*.jsonl (deterministic)
python3 tests/spotcheck.py --random 50      # compare stored cases with the original
cp tests/data/visible.jsonl cheat/          # keep the cheat's sheet in sync
```
