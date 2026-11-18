# sanyam/figlet-go

A [Harbor](https://github.com/laude-institute/harbor) task by Sanyam Sehrawat.

The agent gets [pyfiglet](https://github.com/pwaller/pyfiglet) 1.0.4 (commit `255827e`) in a
room with Go and no Python, and must replace it with a Go program that prints the same giant
letters, byte for byte, for every combination of font, width, justification, direction,
mirror, flip and newline options, then delete the Python code. Grading is a weighted mean of
a build check (1), a "no Python left" check (1) and the share of 2000 hidden command lines
answered exactly like the original (4).

- [instruction.md](instruction.md): what the agent is told.
- [NOTES.md](NOTES.md): source, the six smushing rules with examples, options, rare cases, traps.
- [environment/Dockerfile](environment/Dockerfile): two stages, `alpine/git` → `golang:1.23-alpine`, user `player`, no python.
- [tests/gen.py](tests/gen.py): builds the answer sheets from the original (seeded, deterministic).
- [tests/grade.go](tests/grade.go): the grader, Go standard library only, offline.
- [solution/](solution/): the oracle, a 600-line Go port of `FigletBuilder`.
- [cheat/](cheat/): a lookup of the visible answers, to prove the hidden set catches it.

## Results

| Trial | What runs | Reward | Required |
|---|---|---|---|
| oracle | `solution/solve.sh` | **1.0**, secret drawn | 1.0 |
| nop | nothing | **0.0** | 0.0 |
| cheat | `cheat/solve.sh` | **0.1667**, no secret | below 0.2 |

Screenshots of the three verifier outputs belong in `screenshots/` (oracle.png, nop.png,
cheat.png).

## Reproduce

Docker running, `harbor` installed (`uv tool install harbor`), from this folder:

```
bash scripts/harbor_all.sh
```

Runs the three trials and prints the three `reward.json` files and the secret line of each
verifier log. One trial by hand: `harbor run -p . -a oracle`.

Without Harbor, inside the task image (about a minute):

```
bash scripts/check.sh          # oracle against visible and hidden
bash scripts/check.sh cheat    # the cheat
```

Rebuild the answer sheets (needs the pyfiglet checkout from NOTES.md next to this folder,
with `pyfiglet/fonts` filled; must leave `tests/data/` unchanged):

```
python tests/gen.py
```

## Two lines for the hand-in

**Smushing rule that broke the port most:** none of the six rules themselves; the traps were
around them. The end-of-line cut keeps `prevCharWidth` of the character that did not fit,
the first glyph on a line loses its leading blank columns, and `-j center`/`-j right` pad
to `-w` before `-r` mirrors the row, so the padding lands on the right. The one hidden
mismatch in the first run was not the port but the generator: a split text produced a bare
`--`, which optparse swallows.

**Docker trap hit:** none at build time (Alpine Go, `apk add bash`, `adduser -D player`,
two stages). The trap was in the grader instead: pyfiglet raises on a glyph wider than `-w`
and prints nothing, so a binary that prints nothing scored those cases for free and the
cheat landed at 0.36. The sheets now keep 1% of such cases and `no_python` needs a match
with real output.
