# my-org/figlet-go

Harbor task: port pyfiglet (https://github.com/pwaller/pyfiglet, commit
`255827efbc09d662875dad50fa0edc1e1263bf45`, MIT) to Go, byte for byte, in a
two-stage Alpine room with no Python, as the non-root user `player`.

## Layout

| Path | Role |
|---|---|
| `instruction.md` | what the agent reads |
| `environment/Dockerfile` | stage 1 fetches pyfiglet + font setup; stage 2 = golang:1.23-alpine, bash, user `player`, no Python |
| `solution/` | oracle: `solve.sh` installs the Go port in `solution/go/` and deletes every `.py` |
| `tests/` | `test.sh` → `grade.go` (build 1, no_python 1, behaviour 4 on 2000 hidden cases) + secret |
| `tests/gen.py` | makes `tests/data/*.jsonl` from the original pyfiglet (Python 3.12, seed 7) |
| `cheat/` | answer-lookup cheat used to prove the grader catches it |
| `tools/` | helper scripts (dev loop, simulation, cheat run, pre-hand-in check); not used by Harbor |

## Results (Harbor 0.25.0)

| Run | Reward | Screenshot |
|---|---|---|
| oracle (`harbor run -p figlet-go -a oracle`) | **1.000** | [screenshots/oracle.png](screenshots/oracle.png) |
| nop (`harbor run -p figlet-go -a nop`) | **0.000** | [screenshots/nop.png](screenshots/nop.png) |
| cheat (`bash tools/run-cheat.sh`: oracle on a copy whose solution is `cheat/`) | **0.167** (behaviour 0, no secret) | [screenshots/cheat.png](screenshots/cheat.png) |

## Reproduce

```bash
harbor run -p figlet-go -a oracle -o jobs --job-name oracle-check
harbor run -p figlet-go -a nop    -o jobs --job-name nop-check
bash figlet-go/tools/run-cheat.sh
```

`harbor lint` reports `shared-verifier`; this is expected (the task uses the default
shared verifier; grading files are copied in only after the agent finishes).
