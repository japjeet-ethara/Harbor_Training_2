# practice/figlet-go

Port pyfiglet (commit `255827efbc09d662875dad50fa0edc1e1263bf45`, MIT) to Go,
byte for byte. See `NOTES.md` for the source analysis.

## Layout

| Path | What |
|---|---|
| `environment/Dockerfile` | two-stage room: Alpine+git fetches pyfiglet and does the Makefile font step; `golang:1.23-alpine` (digest-pinned) + bash, user `player`, no Python |
| `instruction.md` | what the agent must build |
| `tests/gen.py` | author-side generator (runs on the host, next to `../pyfiglet`), `random.seed(7)` |
| `tests/data/{visible,hidden}.jsonl` | 2000 + 2000 frozen pyfiglet outputs |
| `tests/grade.go`, `tests/test.sh` | Go grader: build 1, no_python 1, behaviour 4 (hidden cases) |
| `tests/secret.b64` | secret drawn by the twin when behaviour = 1.0 |
| `solution/` | oracle: Go port in `solution/figlet/`, installed by `solve.sh` |
| `cheat/solve.sh` | lookup-table cheat over `visible.jsonl` (grader self-test) |

## Results (local Docker, Harbor 0.26)

| Run | build | no_python | behaviour | reward | secret |
|---|---|---|---|---|---|
| oracle | 1 | 1 | 1.000 (2000/2000) | **1.000** | drawn |
| nop | 0 | 0 | 0.000 | **0.000** | — |
| cheat | 1 | 0 | 0.000 (0/2000) | **0.167** | "No secret for you. Yet." |

`harbor task lint` only reports the expected `shared-verifier` warning.

## Re-running

```
harbor run -p ./figlet-go -a oracle
harbor run -p ./figlet-go -a nop
cp figlet-go/solution/solve.sh /tmp/real_solve.sh
cp figlet-go/cheat/solve.sh figlet-go/solution/solve.sh
harbor run -p ./figlet-go -a oracle          # cheat run
cp /tmp/real_solve.sh figlet-go/solution/solve.sh
```

Regenerating the data (needs `../pyfiglet` with `pyfiglet/fonts` populated):
`python tests/gen.py` — two runs give identical files.

## Two lines

- Smushing rule that is the biggest trap: pyfiglet's "big X" rule is not
  FIGlet's — `/`+`\` becomes `|` and `\`+`/` becomes `Y`; together with
  right-to-left swapping which side wins, that is where a naive port drifts.
- Docker/host trap hit: git on Windows (`core.autocrlf=true`) checked the fonts
  out with CRLF; the generator checkout was switched to LF so the host matches
  the Linux room exactly. (Alpine's missing `bash` is handled with `apk add bash`.)
