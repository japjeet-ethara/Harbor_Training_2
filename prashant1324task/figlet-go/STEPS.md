# figlet-go: all 8 steps, with the files that go with them

Folder layout (run everything from the folder that holds both `pyfiglet/` and `figlet-go/`):

```
pyfiglet/                  Step 1: original, pinned, font setup done
figlet-go/
  NOTES.md                 Step 1
  task.toml                Step 2
  environment/Dockerfile   Step 2
  instruction.md           Step 3
  tests/gen.py             Step 4 (laptop only)
  tests/data/visible.jsonl Step 4
  tests/data/hidden.jsonl  Step 4
  tests/grade.go           Step 5
  tests/test.sh            Step 5
  tests/secret.b64         Step 5
  solution/solve.sh        Step 6 (oracle)
  cheat/solve.sh           Step 7
  dev/check_visible.go     Step 6 helper: first 5 differing cases
  dev/make_cheat.py        Step 7 helper: rebuilds cheat/solve.sh
  STEPS.md                 this file
```

---

## Step 1: Understand pyfiglet (30 min) -> `NOTES.md`

```
git clone https://github.com/pwaller/pyfiglet
cd pyfiglet
git checkout 255827efbc09d662875dad50fa0edc1e1263bf45
git log -1 --format='%H %cI'
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
python3 -m pyfiglet -f slant "hello"
```

Result for this commit: `255827efbc09d662875dad50fa0edc1e1263bf45 2026-08-02T22:51:18+01:00`, licence MIT.
Without the font setup the original fails with `ModuleNotFoundError: No module named 'pyfiglet.fonts'`; the Makefile target `minimal` fixes it with `cp pyfiglet/fonts-standard/* pyfiglet/fonts`.

Prompt for the agent (if you want it to redo the notes): the one in the task page (read `__init__.py`, Makefile, setup.py, LICENSE; write NOTES.md only).

Check yourself: licence MIT; two smushing examples (for example `|`+`/` -> `/`, `[`+`]` -> `|`).
Correction found while reading the code: `/`+`\` -> `|`, `\`+`/` -> `Y`, only `>`+`<` -> `X`.

## Step 2: Build the two-stage room (75 min) -> `environment/Dockerfile`, `task.toml`

`harbor task init figlet-go` creates the folder; replace its Dockerfile and task.toml with these.

- Stage 1 `source` (`alpine:3.20` + git): clone, checkout the pinned SHA, delete `.git`, `pyfiglet/fonts-contrib`, `test-fonts`, then copy `fonts-standard` into a new `pyfiglet/fonts`.
- Stage 2 (`golang:1.23-alpine`): `bash`, user `player` (uid 1000) owning `/workspace/repo`, `COPY --from=source`, `CGO_ENABLED=0`, `GOTOOLCHAIN=local`, `USER player`, `WORKDIR /workspace/repo`. No Python anywhere.
- `task.toml`: `[agent] user = "player"`, `timeout_sec = 3600`; `[environment] network_mode = "public"`, `build_timeout_sec = 900`; `[verifier] timeout_sec = 900`; no verifier `environment_mode` (shared).

You run (needs Docker):

```
docker build -t figlet-room environment/
docker run --rm figlet-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; command -v python3 || echo NO_PYTHON; command -v bash; touch x && echo CAN_WRITE'
```

Expected: `player`, `/workspace/repo`, a number above 50 (72), `NO_PYTHON`, a bash path, `CAN_WRITE`.
Optional pin by digest: `docker inspect --format '{{index .RepoDigests 0}}' golang:1.23-alpine`, then use it in `FROM`.
`harbor lint` warns `shared-verifier`; that is expected.

## Step 3: Write `instruction.md` (45 min)

Written to the six sections of the task page. Read it twice: every sentence must be testable (build, no `.py`, byte-exact output, the eight options). `-w` is stated to apply to wrapping **and** to `-j`, so `-j center -w 40` means "centre inside 40 columns".

## Step 4: Make cases and freeze the answers (45 min) -> `tests/gen.py`

```
python3 tests/gen.py
```

Takes about 10 seconds. Seeded with `random.seed(7)`. Every `"out"` comes from the original `pyfiglet.main()`.

- 2000 visible + 2000 hidden cases, `args` of the two files never overlap.
- Cases where the original raises an error, or loops for ever (a char exactly as wide as `-w`), are skipped, deterministically.
- About a third of the cases use `-D right-to-left`, about 30% have width 10 or 20, about 10% have both. `gen.py` asserts these minimums.

Checks: run twice and `diff -r` the data folders (identical); `wc -l tests/data/*.jsonl` (2000 each); run 3 `args` lines through `python3 -m pyfiglet` and compare.

## Step 5: Write the grader and hide your secret (60 min) -> `tests/grade.go`, `tests/test.sh`, `tests/secret.b64`

```
printf 'nice port, human' | base64 > tests/secret.b64
```

`test.sh` runs `cd /tests && go run grade.go`. Scores:

| Part | Weight | Rule |
|---|---|---|
| build | 1 | `go build -o /tmp/figlet .` in `/workspace/repo` works |
| no_python | 1 | build passed AND no `.py` under `/workspace/repo` AND at least one hidden case matches |
| behaviour | 4 | share of `hidden.jsonl` cases with exactly equal stdout (5 s timeout each) |

Reward = weighted mean, written to `/logs/verifier/reward.json` with the three parts.
By hand: nothing = 0; build only = 1/6 = 0.1667; perfect = 1.0.
Perfect behaviour prints `*** SECRET UNLOCKED ***` and the secret drawn in `-f slant` by the solver's own program; otherwise `No secret for you. Yet.`

## Step 6: Write the solution, the oracle (120 min) -> `solution/solve.sh`

`solve.sh` moves the `.flf` fonts to `/workspace/repo/fonts/`, writes `go.mod` and one `main.go` (standard library only, fonts embedded with `go:embed`), deletes all `.py`, and builds. The port follows `FigletFont.loadFont`, `FigletBuilder` (all six smushing rules, in the original order), `FigletString` and `main()` one by one.

```
harbor run -p ./figlet-go -a oracle
```

Expected: reward 1.0 and the secret in the verifier log.
If it ever drops below 1.0, find the first differing cases:

```
go build -o /tmp/figlet .        # inside the repo
go run dev/check_visible.go      # prints the first 5 differing cases as diff -u
```

Paste those 5 back to the agent: "These are wrong. Fix them without breaking the others." Repeat until 0 differ.

## Step 7: Nop and cheat check (30 min) -> `cheat/solve.sh`

```
harbor run -p ./figlet-go -a nop                 # expect 0.0
cp solution/solve.sh /tmp/real-solve.sh          # keep the real one!
cp cheat/solve.sh solution/solve.sh
harbor run -p ./figlet-go -a oracle              # expect below 0.2, no secret
cp /tmp/real-solve.sh solution/solve.sh          # put the real one back
```

`cheat/solve.sh` ports nothing: it embeds a copy of `visible.jsonl`, looks its arguments up, prints the stored answer, deletes the `.py` files. Rebuild it with `python3 dev/make_cheat.py` if you regenerate the cases.
Expected: build 1, no_python 0 (no hidden case matches), behaviour 0 -> **0.1667**, and "No secret for you. Yet."

## Step 8: Hand in, and unlock this page's secret (15 min)

- Push the task folder to GitHub, in your own folder.
- Three real screenshots: oracle, nop, cheat rewards (terminal captures).
- Two lines: the smushing rule that broke the agent most, and the Docker trap you hit.

The page's secret, inside your built image with the oracle's code:

```
cd /workspace/repo && go build -o /tmp/figlet . && /tmp/figlet -f slant "$(echo Z28gdG91Y2ggZ3Jhc3M= | base64 -d)"
```

Clean letters mean your twin is right.

---

## What was checked while building this (and what was not)

Checked in a sandbox that mirrors the image layout (`/workspace/repo`, `/tests`, `/logs`, running the real `test.sh`):

| Check | Result |
|---|---|
| nop (nothing done) | 0.0 |
| oracle | 2000/2000 hidden cases, reward 1.0, secret drawn |
| cheat | 0.1667, no secret |
| `gen.py` run twice | identical files |
| Go port vs original, all 67 fonts, 794 extra random cases | 0 mismatches |
| oracle from another working directory; 0 `.py` left; `go build ./...` | all pass |
| Mutation test (one rule broken at a time) | caught: equal, lowline, hierarchy, pair, `-j right`, `-j center`, wrap `>=`, `-r`, `-F`, `-n` |

Not checked here (no Docker daemon, no Harbor in the sandbox): `docker build`, the six `docker run` checks, `harbor run ... oracle|nop`, the `harbor lint` output, the screenshots and the GitHub push. The `task.toml` keys come from the Harbor docs; `harbor task init` may add or rename fields, so compare once.

Known limit of the exam: three code paths never change any output in the hidden cases: Big X (bit 16), the hardblank rule (bit 32), and the "width below 2" no-smush guard. A port that gets those wrong still scores 1.0. For Big X and the hardblank rule this cannot be fixed by choosing other fonts: none of the 67 fonts in `fonts-standard/` turns those bits on (checked), so they are dead code for this task. Porting them anyway is harmless and matches the original.
