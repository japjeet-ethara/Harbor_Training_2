# Practice Task 2 — FIGlet in Go (Harbor Task)

A complete, production-grade Harbor task that ports Python's [pyfiglet](https://github.com/pwaller/pyfiglet) (commit `255827efbc09d662875dad50fa0edc1e1263bf45`) to a pure Go twin, tested against 4,000 frozen cases, and graded automatically in a locked-down, multi-stage Docker environment without Python.

---

## 📊 Evaluation Summary

| Test Run | Expected Score | Actual Score | Result | Status |
| :--- | :---: | :---: | :---: | :---: |
| **Oracle (`solve.sh`)** | `1.0` | **`1.000`** | **PASS** | ✅ Unlocks Secret Banner |
| **Nop Check (`nop`)** | `0.0` | **`0.000`** | **PASS** | ✅ No build / no output |
| **Cheat Check (`cheat/solve.sh`)** | `< 0.2` | **`0.167`** | **PASS** | ✅ Caught by hidden cases |

---

## 📸 Real Terminal Screenshots

### 1. Oracle Run (`Reward = 1.000`)
The Go twin builds with zero third-party dependencies, matches all 2,000 hidden test cases byte-for-byte, confirms zero `.py` files exist in `/workspace/repo`, and unlocks the secret banner:

![Oracle Run](screenshot/oracle.png)

---

### 2. Nop Check (`Reward = 0.000`)
Verifies that an empty/unimplemented repository receives a score of exactly `0.000`:

![Nop Run](screenshot/nop.png)

---

### 3. Cheat Check (`Reward = 0.167`)
Verifies that a cheat agent attempting to look up inputs from `visible.jsonl` fails all hidden cases. It earns only the build score (`1/6 ≈ 0.167`), scoring `0` on behaviour and `no_python`:

![Cheat Run](screenshot/cheat.png)

---

### 4. Page Secret Output (`go touch grass`)
Running the twin on the page's hidden base64 string `Z28gdG91Y2ggZ3Jhc3M=` renders the giant ASCII-art letters cleanly:

![Output Secret](screenshot/output.png)

```text
                   __                   __                                 
   ____ _____     / /_____  __  _______/ /_     ____ __________ ___________
  / __ `/ __ \   / __/ __ \/ / / / ___/ __ \   / __ `/ ___/ __ `/ ___/ ___/
 / /_/ / /_/ /  / /_/ /_/ / /_/ / /__/ / / /  / /_/ / /  / /_/ (__  |__  ) 
 \__, /\____/   \__/\____/\__,_/\___/_/ /_/   \__, /_/   \__,_/____/____/  
/____/                                       /____/                        
```

---

## 📝 Hand in Files

> 1. **Smushing rule:**  
>    *Rule 5 (Big-X slash smushing where `/ + \ -> |` and `\ + / -> Y`) and right-to-left layout universal smushing, where the dominant foreground character depends on the user's input text entry order rather than physical screen coordinates.*
>
> 2. **Docker trap:**  
>    *Alpine base image (`golang:1.23-alpine`) does not include `bash` by default (causing Harbor to fail before any tests run without `apk add bash`), and on Windows, the Docker Desktop context required explicit `DOCKER_HOST="npipe:////./pipe/docker_engine"` configuration.*

---

---

## 🚀 How to Run with Harbor

Make sure Docker is running. From the parent directory or inside `figlet-go`:

### 1. Run Oracle (Expect 1.000)
```bash
harbor run -p ./figlet-go -a oracle
```

### 2. Run Nop Check (Expect 0.000)
```bash
harbor run -p ./figlet-go -a nop
```

### 3. Run Cheat Check (Expect 0.167)
```bash
cp ./figlet-go/cheat/solve.sh ./figlet-go/solution/solve.sh
harbor run -p ./figlet-go -a oracle
cp ./figlet-go/solution/solve.sh.real ./figlet-go/solution/solve.sh
```

---

## ✅ Self-Check Before Hand-In Checklist

- [x] **NOTES.md:** URL, commit SHA (`255827efbc09d662875dad50fa0edc1e1263bf45`), date (`2026-08-02T22:51:18+01:00`), MIT license, font setup step, all 6 smushing rules + kerning.
- [x] **Image:** Two stages (`source` + agent room), `golang:1.23-alpine` (pinned), `bash` installed, user `player` (uid 1000), `NO_PYTHON` verified (`command -v python3` returns nothing).
- [x] **instruction.md:** Every sentence testable, no secrets, no internal test names.
- [x] **gen.py:** Seeded (`random.seed(7)`), answers captured from original pyfiglet, exactly 2000 visible + 2000 hidden test cases.
- [x] **Grader in Go:** Pure standard library, `build` weight 1, `no_python` weight 1 (tied to build AND correct hidden answer), `behaviour` weight 4.
- [x] **Verification:** Oracle = 1.0 (with secret banner), Nop = 0.0, Cheat = 0.167 (< 0.2, no secret).
- [x] **Hand-in:** Real screenshots included in `screenshot/` and two honest lines documented.
