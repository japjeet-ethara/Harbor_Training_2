# Practice Task 2 — Hand-in Report

## Task Overview
- **Task**: `practice/figlet-go`
- **Source**: FIGlet Go twin (`pyfiglet` port)
- **Harbor Target**: Harbor 0.25.0

---

## 1. Harbor Trial Results

| Trial | Command | Expected | Observed | Status |
|---|---|---|---|---|
| **Oracle** | `harbor run -p ./figlet-go -a oracle` | 1.000 | **1.0000** | Pass |
| **Nop** | `harbor run -p ./figlet-go -a nop` | 0.000 | **0.0000** | Pass |
| **Cheat** | `harbor run -p ./figlet-go -a oracle` (with cheat) | < 0.20 | **0.1667** | Pass |

### Reward Weight Verification
- `build`: 1 / 6 (weight 1)
- `no_python`: 1 / 6 (weight 1, gated on `build == 1.0`, zero `.py` files, and `matchedCases >= 1`)
- `behaviour`: 4 / 6 (weight 4, based on `/tests/data/hidden.jsonl`)
- **Cheat score**: Build succeeds (1.0), but because `hidden.jsonl` has 0 overlap with `visible.jsonl`, `matchedCases = 0`. `no_python` pays 0, giving strictly `1/6 = 0.1667 < 0.20`.

---

## 2. Hand-in Reflection Questions

### Which smushing rules broke the agent most?
1. **Rule 5 (`SM_BIGX`) Asymmetry**: `/` + `\` smushes into `|`, but `\` + `/` smushes into `Y` (and `>` + `<` smushes into `X`). Agents frequently make the mistake of treating slash pairs symmetrically as `X` or `|`.
2. **Right-to-Left Smush Direction**: When `-D right-to-left` is active, the left and right buffers must be swapped during both `smushAmount()` and `smushRow()`, and universal overlapping (`smushMode & 63 == 0`) must return the left character instead of the right.
3. **Empty Buffer Line Wrapping Infinite Loop**: In `pyfiglet`, if a character's width equals `width` on an empty buffer, `currentTotalWidth >= width` calls `cutBufferAtLastChar()` which decrements iterator while the main loop increments it, causing an infinite loop. The port handles character width validation properly.
4. **Justification relative to `-w COLS`**: `-j center` and `-j right` calculate space padding relative to the target terminal width specified by `-w` (not a fixed 80).

### Which Docker traps were encountered and resolved?
1. **`golang:1.23-bookworm` contains Python 3**: Despite being a Go image, it includes `/usr/bin/python3`, failing requirement D2. Resolved by using `golang:1.23-alpine` pinned by digest.
2. **Alpine lacks `bash`**: Harbor internally invokes `bash` for running commands and test runners. An unadorned Alpine container crashes before running tests. Resolved by explicitly adding `RUN apk add --no-cache bash`.
3. **Non-root `player` user permissions**: The verifier and grader write to `/logs/verifier/reward.json`. Created `/logs/verifier` with `chown -R player:player /logs` in the Dockerfile so user `player` (uid 1000) can write rewards without root privileges.
4. **Build context isolation**: The Docker build context only sees `environment/`. Stage 1 fetches `pyfiglet` directly from GitHub at pinned commit `255827efbc09d662875dad50fa0edc1e1263bf45` and executes the font setup step (`cp fonts-standard/* fonts/`).

---

## 3. Secrets Unlocked

1. **Student / Agent Secret** (Grader secret in `tests/secret.b64`):
   - Encoded: `bmljZSBwb3J0LCBodW1hbg==` (`nice port, human`)
   - Unlocked when `behaviour == 1.000`:
   ```
             _                               __ 
      ____  (_)_______     ____  ____  _____/ /_
     / __ \/ / ___/ _ \   / __ \/ __ \/ ___/ __/
    / / / / / /__/  __/  / /_/ / /_/ / /  / /__ 
   /_/ /_/_/\___/\___/  / .___/\____/_/   \__( )
                       /_/                   |/ 
       __                              
      / /_  __  ______ ___  ____ _____ 
     / __ \/ / / / __ `__ \/ __ `/ __ \
    / / / / /_/ / / / / / / /_/ / / / /
   /_/ /_/\__,_/_/ /_/ /_/\__,_/_/ /_/ 
   ```

2. **Page Secret** (Task author's easter egg):
   - Encoded: `Z28gdG91Y2ggZ3Jhc3M=` (`go touch grass`)
   - Rendered cleanly with `/tmp/figlet -f slant`:
   ```
                      __                   __                                 
      ____ _____     / /_____  __  _______/ /_     ____ __________ ___________
     / __ `/ __ \   / __/ __ \/ / / / ___/ __ \   / __ `/ ___/ __ `/ ___/ ___/
    / /_/ / /_/ /  / /_/ /_/ / /_/ / /__/ / / /  / /_/ / /  / /_/ (__  |__  ) 
    \__, /\____/   \__/\____/\__,_/\___/_/ /_/   \__, /_/   \__,_/____/____/  
   /____/                                       /____/                        
   ```
