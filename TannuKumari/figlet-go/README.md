# practice/figlet-go

Harbor task: port **pyfiglet**'s FIGlet rendering engine to Go, byte for byte,
in a container that has no Python in it.

* Upstream: https://github.com/pwaller/pyfiglet @ `255827efbc09d662875dad50fa0edc1e1263bf45` (MIT)
* Study notes for the port: `../NOTES.md`

## Layout

```
environment/Dockerfile       two-stage room (D1-D6)
instruction.md               what the agent is told
task.toml                    agent runs as "player", verifier as root
tests/gen.py                 host-side generator: freezes the original's answers
tests/data/visible.jsonl     2000 cases for my own debugging (never graded)
tests/data/hidden.jsonl      2000 graded cases, arg-lists disjoint from visible
tests/grade.go               the grader (Go: the room has no Python)
tests/test.sh                `cd /tests && go run grade.go`
tests/secret.b64             message the grader draws with a perfect twin
solution/solve.sh            oracle: writes the full Go port, deletes every .py
cheat/solve.sh               lookup-table cheat, to prove the grader catches it
cheat/visible-answers.jsonl  the visible answers the cheat ships with
```

## Image checks (all six D-rules)

```
docker build -t figlet-room environment/
docker run --rm figlet-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; \
  command -v python3 || echo NO_PYTHON; command -v bash; touch x && echo CAN_WRITE'
```

Observed: `player`, `/workspace/repo`, `72`, `NO_PYTHON`, `/bin/bash`, `CAN_WRITE`.
Both base images are pinned by tag **and** `@sha256:` digest. The build itself
fails if `python3`/`python` ever appears in the final stage.

## Regenerating the frozen answers

Needs the pinned pyfiglet checkout next to this folder, with the Makefile's font
step done (`mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/`):

```
python3 tests/gen.py          # ~7 s, deterministic (seed 7)
```

Run it twice and `diff` the files: identical. Every `"out"` comes from the
original `pyfiglet.main()`; three lines were re-checked by hand against
`python -m pyfiglet <args>`.

## Grading

| Component | Weight | Rule |
|---|---|---|
| `build` | 1 | `go build -o /tmp/figlet .` in `/workspace/repo` |
| `no_python` | 1 | build passed **and** no `.py` under the repo **and** at least one hidden case matches |
| `behaviour` | 4 | share of the 2000 hidden cases whose stdout matches byte for byte (5 s each, graded from `/tmp`) |

`reward = (build + no_python + 4*behaviour) / 6`

Hand-computed expectations: nothing done `0.0`, build only `1/6 ≈ 0.167`,
perfect `1.0`. If `behaviour` is exactly `1.0` the grader base64-decodes
`tests/secret.b64` and draws it with the twin itself under
`*** SECRET UNLOCKED ***`; otherwise it prints `No secret for you. Yet.`

## Measured results

| Run | Command | reward | build | no_python | behaviour | secret |
|---|---|---|---|---|---|---|
| oracle | `harbor run -p ./figlet-go -a oracle` | **1.0** | 1 | 1 | 1.0 (2000/2000) | drawn |
| nop | `harbor run -p ./figlet-go -a nop` | **0.0** | 0 | 0 | 0.0 | none |
| cheat | cheat `solve.sh` copied over `solution/` | **0.1667** | 1 | 0 | 0.0 (0/2000) | none |

The cheat builds, deletes the Python and still scores only the build point: its
table holds the *visible* answers, and no hidden arg-list appears there
(`gen.py` keeps the two sets disjoint), so `no_python` stays unpaid.

## Two honest lines

* **The rule that is hardest to copy:** not one of the six smushing rules on its
  own — it is `smushRow`/`updateSmushedCharInLeftBuffer`. Python replaces a
  character with the *empty string* when `smushChars` returns one, so the row
  gets shorter; and it silently skips the write when the index is out of range,
  while an in-range `None` crashes instead. Get that wrong and `|_|` or `/\` in
  a narrow window drifts by one column while `hello` still looks perfect.
* **The Docker trap I hit:** `bash -l` runs `/etc/profile`, which resets `PATH`
  and drops `/usr/local/go/bin` that the Go base image had set — `go: command
  not found` inside an image that obviously has Go. Fixed with
  `/etc/profile.d/go-path.sh`. (The documented traps are real too:
  `golang:*-bookworm` ships `/usr/bin/python3`, and Alpine ships no `bash`.)
