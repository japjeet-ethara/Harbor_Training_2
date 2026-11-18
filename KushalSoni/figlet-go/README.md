# practice/figlet-go

Harbor practice task 2: port **pyfiglet** (MIT, commit `255827ef`) to a byte-identical Go CLI.

| Path | What |
|---|---|
| `NOTES.md` | Source, licence, the font-setup step, the smushing rules, options, rare cases |
| `environment/Dockerfile` | Two stages, pinned `golang:1.23.12-alpine3.22@sha256:3833…0b5f`, bash, user `player`, no Python |
| `task.toml` | agent user `player`, 3600 s; public network; build 900 s; verifier 900 s (shared) |
| `instruction.md` | What the agent must build |
| `tests/gen.py` | Runs on the author's laptop; seeded; freezes pyfiglet's output (needs `../pyfiglet`) |
| `tests/data/{visible,hidden}.jsonl` | 2000 + 2000 frozen cases (10 fonts, all options) |
| `tests/grade.go`, `tests/test.sh` | Go-only grader: build 1, no_python 1, behaviour 4 (hidden cases) |
| `tests/secret.b64` | Drawn by the agent's own binary only if behaviour = 1.0 |
| `solution/solve.sh` | Oracle: full Go port, fonts embedded with `go:embed`, all `.py` deleted |
| `cheat/solve.sh` | Lookup table of the visible cases; must score below 0.2 |

## Expected rewards

| Run | reward | build | no_python | behaviour | secret |
|---|---|---|---|---|---|
| nothing (nop) | 0.0 | 0 | 0 | 0 | no |
| build only / cheat | 1/6 ≈ 0.167 | 1 | 0 | 0 | no |
| oracle | 1.0 | 1 | 1 | 1 | yes |

## Commands

```bash
# image checks (D1–D6)
docker build -t figlet-room environment/
docker run --rm figlet-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; command -v python3 || echo NO_PYTHON; command -v bash; touch x && echo CAN_WRITE'

# regenerate the frozen cases (optional; output is identical every time)
python3 tests/gen.py

# the three runs (from the folder that contains figlet-go/)
harbor run -p ./figlet-go -a oracle
harbor run -p ./figlet-go -a nop
cp figlet-go/solution/solve.sh /tmp/solve.real \
  && cp figlet-go/cheat/solve.sh figlet-go/solution/solve.sh \
  && harbor run -p ./figlet-go -a oracle ; cp /tmp/solve.real figlet-go/solution/solve.sh
```

## Notes

- **Extra fonts.** `ivrit` and `term` were added to the 8 fonts in the brief. `ivrit` is the only
  standard font whose header says right-to-left, so it tests `-D auto` / `-j auto`. `term` is
  full-width (no overlap at all).
- **Rules 5 and 6 can't be tested.** No standard font enables Big X (16) or hardblank (32)
  smushing. The port implements both, but no command-line input can reach them.
- **Excluded inputs.** Inputs that make pyfiglet crash (`CharNotPrinted` when a letter is
  wider than `-w`) or loop forever are left out of the cases (138 skipped). The instruction
  says these are not checked.
- **Expected lint warning.** `harbor lint` reports `shared-verifier`. That is expected for this
  practice task.
