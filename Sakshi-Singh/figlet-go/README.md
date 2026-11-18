# practice/figlet-go

Harbor practice task 2: port pyfiglet (commit `255827efbc09d662875dad50fa0edc1e1263bf45`,
MIT) to Go, byte for byte, in a two-stage, Python-free, non-root Docker room.

| path | what |
|---|---|
| `NOTES.md` | Step 1 notes: licence, font setup, smushing rules, options, rare cases |
| `environment/Dockerfile` | two stages: `alpine/git` fetches pyfiglet + font setup; `golang:1.23-alpine` + bash + user `player` (both pinned by digest) |
| `task.toml` | agent user `player`, 3600 s; build 900 s; verifier 900 s, shared |
| `instruction.md` | what the solving agent is told |
| `tests/gen.py` | runs on the author's laptop; `random.seed(7)`; freezes 2000 visible + 2000 hidden cases from the original `pyfiglet.main()` |
| `tests/data/*.jsonl` | the frozen answers (visible and hidden never share a command line) |
| `tests/grade.go`, `tests/test.sh` | Go grader: build ×1, no_python ×1 (needs build + one right answer), behaviour ×4 on hidden cases |
| `tests/secret.b64` | the secret, printed in giant letters only for a perfect twin |
| `solution/solve.sh` | the oracle: a full Go port, fonts embedded with `go:embed` |
| `cheat/solve.sh` | a lookup table over `visible.jsonl`, used to prove the grader catches memorisation |

## Results (Harbor 0.25.0, Docker 29.9.2)

| agent | build | no_python | behaviour | reward |
|---|---|---|---|---|
| oracle | 1 | 1 | 2000/2000 | **1.000** (secret unlocked) |
| nop | 0 | 0 | 0/2000 | **0.000** |
| cheat | 1 | 0 | 0/2000 | **0.167** (no secret) |

Screenshots of the three runs are in `screenshot/`.

## Hand-in notes

- **Smushing rule that broke the agent most:** none broke the port; it matched all 4000 cases on the first run. The hardest part was copying Python's own quirks: `'' in s` is always true, and a smush result of `''` removes a column.
- **Docker trap hit:** `golang:1.23-alpine` has no `bash`, so Harbor would fail with `exec: "bash": executable file not found`. I added `apk add --no-cache bash`.

## Regenerate / rerun

```
# from the folder that holds both pyfiglet/ and figlet-go/
python figlet-go/tests/gen.py              # identical output every run
harbor run -p ./figlet-go -a oracle
harbor run -p ./figlet-go -a nop
```

To rerun the cheat: copy `cheat/solve.sh` over `solution/solve.sh`, run the oracle
command, then restore the real `solution/solve.sh`.
