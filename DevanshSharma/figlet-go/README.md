# Practice Task 2 — figlet-go (Devansh Sharma)

Port of **pyfiglet 1.0.4** (`pwaller/pyfiglet`, commit `255827efbc09d662875dad50fa0edc1e1263bf45`, 2026-08-02, MIT) to a Go 1.23 standard-library-only program, graded by Harbor 0.26.0.

Notes on the original (licence, the missing-fonts failure and its fix, the six smushing rules with examples, the eight options, rare cases) are in [`NOTES.md`](NOTES.md).

## Results (real Harbor runs on my machine, `results/`)

| Run | behaviour | build | no_python | **reward** | secret |
|---|---|---|---|---|---|
| oracle | 1.000 | 1.000 | 1.000 | **1.000** | drawn (`nice port, human`) |
| nop | 0.000 | 0.000 | 0.000 | **0.000** | no |
| cheat (memorised visible answers) | 0.000 | 1.000 | 0.000 | **0.167** | "No secret for you. Yet." |

Files: `results/{oracle,nop,cheat}.{result.json,reward.json,verifier-log.txt}`.  
The page's own secret (Step 8) came out clean from the oracle's Go code in the built image (`go touch grass`).

## What is in the task

| Step | File | Notes |
|---|---|---|
| 2 | `environment/Dockerfile`, `task.toml` | two stages (`alpine:3.20.3` fetches pyfiglet at the pinned commit and does the Makefile font setup; `golang:1.23-alpine3.20` is the room); `bash`; normal user `player` (uid 1000) owns `/workspace/repo`; no Python (`NO_PYTHON` verified); `[agent] user = "player"`, `network_mode = "public"`, shared verifier |
| 3 | `instruction.md` | every sentence is checked by the grader; no test names, no answers, no secret |
| 4 | `tests/gen.py`, `tests/data/{visible,hidden}.jsonl` | seeded (`random.Random(7)`), every `out` comes from original `main()`; 2000 + 2000 cases with zero shared argument combinations; handles pyfiglet `char_width >= width` newline wrap trap; byte-identical on repeated runs |
| 5 | `tests/grade.go`, `tests/test.sh`, `tests/secret.b64` | Go standard library only; weights **1 / 1 / 4** as specified; base64-encoded secret unlocked only on `behaviour = 1.0` |
| 6 | `solution/solve.sh` | Go port structured directly after `FigletBuilder`; `.flf` fonts embedded with `go:embed`; all `.py` files deleted |
| 7 | `cheat/solve.sh` | memorises `visible.jsonl`, achieves 0 hidden matches, earns no secret |

## Evidence that the exam is right

- Oracle: 2000/2000 visible and 2000/2000 hidden identical to original pyfiglet output byte for byte.
- Zero argument collision between `visible.jsonl` and `hidden.jsonl` (verified by set intersection).
- The cheat scores exactly **0.1667** (1/6 for build only, 0 for behaviour, 0 for `no_python`), demonstrating that memorising visible answers fails against the hidden evaluation set.

## Where I went beyond the page (additive enhancements)

1. **Deterministic dataset disjointness in `gen.py`:** Guaranteed zero overlap between `visible.jsonl` and `hidden.jsonl` argument tuples, eliminating accidental cheat score leakage.
2. **Infinite loop guard in test generator:** `pyfiglet`'s original builder enters an infinite loop when a character's width equals or exceeds line width (`char_width >= width`) instead of raising `CharNotPrinted`. Added validation to reject such malformed parameter pairs, ensuring generator stability.
3. **Exact translation table parity:** Horizontal mirror (`reverseArt`) and vertical flip (`flipArt`) implemented with byte-accurate character mapping matching `FigletString.__reverse_map__` and `FigletString.__flip_map__`.

The reward formula is strictly: `(1*build + 1*no_python + 4*behaviour) / 6`.

## The two lines

- **Smushing rule:** Right-to-left layout interacting with character smushing and `-j right` padding. In right-to-left mode, the new character is placed to the left of the buffer (`addLeft, addRight = addRight, addLeft`), and universal overlapping priority inverts to the leftmost character.
- **Docker/Harbor traps I actually hit:** `golang:1.23-bookworm` bundles Python 3 (violating D2), whereas Alpine Go images omit `bash` (breaking Harbor runners). The solution was `golang:1.23-alpine3.20` with an explicit `apk add --no-cache bash`.

## Reproduce

```bash
git clone https://github.com/pwaller/pyfiglet && cd pyfiglet && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/ && cd ..
python figlet-go/tests/gen.py          # produces 2000 visible + 2000 hidden cases
harbor run -p ./figlet-go -a oracle    # 1.0000
harbor run -p ./figlet-go -a nop       # 0.0000
# copy cheat/solve.sh over solution/solve.sh, run oracle, restore real solution (cheat = 0.1667)
```
