# figlet-go — Harbor practice task 2 (complete task folder)

Everything the eight steps ask you to produce:

| Step | File(s) |
|---|---|
| 1 | `NOTES.md` |
| 2 | `environment/Dockerfile`, `task.toml` |
| 3 | `instruction.md` |
| 4 | `tests/gen.py`, `tests/data/visible.jsonl`, `tests/data/hidden.jsonl` (2000 cases each) |
| 5 | `tests/grade.go`, `tests/test.sh`, `tests/secret.b64` |
| 6 | `solution/solve.sh` (the oracle: a complete Go port) |
| 7 | `cheat/solve.sh` (memorises `visible.jsonl`, ports nothing). Not part of the task; copy it over `solution/solve.sh` to try it, then put the real one back |

## Run it

```
bash setup-pyfiglet.sh                       # only needed to re-run gen.py (clones pyfiglet next to figlet-go/)
harbor run -p ./figlet-go -a oracle          # expect 1.0 and the secret in verifier/test-stdout.txt
harbor run -p ./figlet-go -a nop             # expect 0.0
cp figlet-go/solution/solve.sh /tmp/real.sh && cp figlet-go/cheat/solve.sh figlet-go/solution/solve.sh
harbor run -p ./figlet-go -a oracle          # expect 0.1667, "No secret for you. Yet."
cp /tmp/real.sh figlet-go/solution/solve.sh
```

## What was checked here, and what was not

Checked in the sandbox (no Docker available there): the Go port against the original on all 4000 exam cases
(0 differences) and on 1911 extra random cases over all 67 fonts (0 differences); `gen.py` run twice gives identical
files; the Dockerfile's stage-1 commands run for real (72 fonts, no `.git`); the grader/oracle/nop/cheat flow was
simulated with real `test.sh`: **nop 0.0, oracle 1.0 + secret, cheat 0.1667 and no secret**; `harbor lint` (0.26.0)
reports only the expected `shared-verifier` warning.

**Not** done: `docker build` and `harbor run` themselves (needs Docker), the Step 2 six-check one-liner, the real
screenshots, and the two honest lines for Step 8. Those must come from your own machine.
