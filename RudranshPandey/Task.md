# Practice Task 2 — The Machine That Shouts in Giant Letters (7 hours)

You built one Harbor task already. This one has the same eight steps, with two changes:

1. **The Docker image is harder.** Last time the room was "Go plus a copy of the repo". This time you build the room in two stages, with no Python in it, as a normal (non-root) user, and you must fix a setup step the original project needs before it even works.
2. **There is a secret.** Two, in fact. One you hide for the next agent, and one this page hides for you. You only see yours if your work is right.

As before, you write no code by hand. The free Cursor agent writes it; you direct it and check it.

---

## The problem, explained simply

There is a tiny program that takes a short word, like `hello`, and draws it in giant letters made of slashes and underscores:

```
    __         ____    
   / /_  ___  / / /___ 
  / __ \/ _ \/ / / __ \
 / / / /  __/ / / /_/ /
/_/ /_/\___/_/_/\____/ 
```

This is called **FIGlet**, and a Python version of it is called **pyfiglet**. It is the most serious way ever invented to shout in a chat.

It looks silly. It is not simple. When two giant letters stand next to each other, the program squeezes them together ("smushing") using six different rules: two `|` become one, `/` and `\` meet as an `X`, an `_` gets eaten by a `/`, and so on. It also wraps long text, centres it, can write right-to-left, mirror it, and flip it upside down. Every one of those rules has to be copied exactly, space for space.

Your job has the same two halves as last time:

1. **Build a twin of the machine in Go.** Same giant letters, same spaces, for every input and every option. When you are done, the Python is gone.
2. **Build the exam that checks the twin.** Run the original thousands of times, freeze its answers, and grade any Go twin automatically.

Why is it fair but hard? Each smushing rule is small, but they stack. A twin that is right on `hello` can be wrong on `|_|` in a narrow window with right-to-left. Those are the lines your exam will catch.

---

## What is new about the Docker image

Last time your image was one line of "start from Go, copy the repo in". This time the room must meet six rules. Think of it as building a test kitchen where the cook (the agent) cannot sneak in the original recipe book.

| # | Rule | Why it matters |
|---|---|---|
| D1 | **Two stages.** Stage 1 fetches pyfiglet from GitHub at a pinned commit. Stage 2 is the room the agent works in, and copies only what it needs from stage 1. | Stage 1's tools (git and friends) never reach the agent's room. |
| D2 | **No Python anywhere in the final room.** `command -v python3` must find nothing. | If Python is there, the agent can just run the original and copy its answers. The task becomes easy and teaches nothing. |
| D3 | **Finish the original's own setup.** pyfiglet will not even run straight from GitHub: its fonts live in `fonts-standard/` but the code loads them from `fonts/`. The project's own `Makefile` copies them over. Your stage 1 must do the same. | A real repo is rarely ready to use. Reading its build files is part of the job. |
| D4 | **A normal user called `player`**, who owns `/workspace/repo`. Not root. | A non-root agent cannot `apk add python3`, so D2 stays true while the agent works. |
| D5 | **Pinned images** (a fixed tag; a digest `@sha256:...` is even better). | Same room every time, today and in a year. |
| D6 | **Harbor must be able to run inside it.** Harbor runs its own commands with `bash`. | Small images (like Alpine) do not include `bash`. Then Harbor fails before any test runs. |

---

## Your role, again

You are the boss, the agent is the worker. The same three rules apply:

1. **One step at a time.** Paste one prompt, wait, check, go on.
2. **Never trust "done".** Run every "You check" yourself.
3. **When something fails, paste the exact error back.** Not your summary of it.

---

## Overview

| Step | What | Time | End result |
|---|---|---|---|
| 1 | Understand pyfiglet | 30 min | `NOTES.md` with SHA, licence, the smushing rules |
| 2 | Build the two-stage room | 75 min | Image builds, passes all six D-rules |
| 3 | Write `instruction.md` | 45 min | A stranger knows exactly what "done" means |
| 4 | Make cases + freeze answers | 45 min | Visible and hidden answer files |
| 5 | Write the grader + hide your secret | 60 min | `reward.json` + a secret banner for a perfect twin |
| 6 | Write the solution (oracle) | 120 min | **oracle = 1.0**, and you see your secret |
| 7 | Nop and cheat check | 30 min | **nop = 0.0**, **cheat below 0.2** |
| 8 | Hand in + unlock this page's secret | 15 min | GitHub link, three rewards, one secret |

---

## Step 1 — Understand pyfiglet (30 min)

**What to do:**

```
git clone https://github.com/pwaller/pyfiglet
cd pyfiglet
git checkout 255827efbc09d662875dad50fa0edc1e1263bf45
git log -1 --format='%H %cI'
```

**Ask the agent:**

```
Read pyfiglet/__init__.py (especially the FigletBuilder class), the Makefile, setup.py and the LICENSE file. Tell me:
1. the licence name,
2. why "python -m pyfiglet hello" fails straight after cloning, and which Makefile step fixes it,
3. every smushing rule (there are six, plus kerning), each with a tiny example of two characters and the result,
4. what each of these options does: -f, -w, -j, -D, -r, -F, -s, -n.
Write all of this into NOTES.md. Do not change any other file.
```

**You check:**
- The licence is MIT. Copy the SHA and date into `NOTES.md` yourself.
- Do the font setup the agent found, then try it:
  ```
  mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
  python -m pyfiglet -f slant "hello"
  ```
  You should see giant slanted letters. If not, paste the error back.
- Test two of the agent's smushing examples yourself. Is it right?

**Hints:**
- Only use the fonts in `fonts-standard/`. The ones in `fonts-contrib/` come from many authors.
- The task covers the options above only. Colour (`-c`), font install (`-L`), font list (`-l`) and font info (`-i`) are out.
- Every place where the agent was wrong goes on your "rare cases" list.

**You should have:** `NOTES.md` with URL, SHA, date, MIT, the font setup step, six smushing rules with examples, and the eight options.

---

## Step 2 — Build the two-stage room (75 min)

This is the new, harder step. Run `harbor task init figlet-go` next to the pyfiglet folder and open `figlet-go` in Cursor.

**Ask the agent:**

```
This is a Harbor task folder. Write environment/Dockerfile with two stages.

Stage 1 (name it "source"): start from a pinned image that has git. Clone https://github.com/pwaller/pyfiglet, check out commit 255827efbc09d662875dad50fa0edc1e1263bf45, delete the .git folder, delete pyfiglet/fonts-contrib and test-fonts, then do the font setup step from the Makefile (copy fonts-standard into a new pyfiglet/fonts folder).

Stage 2 (the agent's room): start from golang:1.23-alpine (pinned). Install bash. Create a normal user "player" with uid 1000. Copy /src from stage 1 into /workspace/repo, owned by player. Set CGO_ENABLED=0 and GOTOOLCHAIN=local. Switch to USER player. WORKDIR /workspace/repo.

The final image must contain no python at all. Do not touch any other file. Tell me the docker build command.
```

**You check:** build it, then run all six checks in one line:

```
docker build -t figlet-room environment/
docker run --rm figlet-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; command -v python3 || echo NO_PYTHON; command -v bash; touch x && echo CAN_WRITE'
```

You must see: `player`, `/workspace/repo`, a font count above 50, `NO_PYTHON`, a path to bash, and `CAN_WRITE`.

**Hints (all three are real traps; we hit them while preparing this task):**
- **`golang:1.23-bookworm` contains Python 3.** It looks like a "Go image", but `command -v python3` finds `/usr/bin/python3`. That is why this task uses the Alpine image. If the agent picks bookworm, D2 fails.
- **Alpine has no `bash`.** Harbor runs its commands with bash. Without it, the run fails with `exec: "bash": executable file not found`, before any test.
- **The build can only see the `environment/` folder.** That is why stage 1 fetches pyfiglet from GitHub instead of copying it from your laptop.
- Optional: pin by digest. Run `docker inspect --format '{{index .RepoDigests 0}}' golang:1.23-alpine` and use the result in `FROM`.

**Then set task.toml.** Ask the agent:

```
In task.toml set: [agent] user = "player", timeout 3600 seconds; [environment] network_mode = "public", build_timeout_sec 900; [verifier] timeout 900 seconds. Leave verifier environment_mode at its default (shared). Do not change anything else.
```

Why these two choices:
- **`network_mode = "public"`:** Harbor 0.25 refuses `"no-network"` on a local Docker machine (`network_mode='no-network' is not supported by EnvironmentType.DOCKER`). Real tasks lock the network on a cloud runner; for practice we accept public.
- **Shared verifier:** the grading files are copied in only after the agent has finished, so the agent still never sees them. `harbor lint` will warn `shared-verifier`; that is expected here. A separate verifier needs an artifact hand-over that is a later lesson.

**You should have:** an image that passes all six checks, and a task.toml with `user = "player"`.

---

## Step 3 — Write `instruction.md` (45 min)

**Ask the agent:**

```
Write instruction.md for this Harbor task. The reader is another AI agent.
Sections:
1. What the repository is (one paragraph from NOTES.md).
2. What to build: a Go program in /workspace/repo. `go build -o /tmp/figlet .` run in /workspace/repo must produce a command-line program that accepts the same options as pyfiglet: -f FONT, -w COLS, -j auto|left|center|right, -D auto|left-to-right|right-to-left, -r, -F, -s, -n, followed by the text. It prints the same output as pyfiglet, byte for byte.
3. Fonts: the .flf files may move, but must stay in the repository; the program must work from any working directory.
4. Hard rules: Go standard library only; no .py files anywhere in the repository; `go build ./...` must pass.
5. Not checked: speed, memory, error messages, the options -c, -L, -l, -i.
6. Grading: hidden automatic cases, partial score.
Do not mention tests, answers, example cases, or any secret.
```

**You check:** read it twice.
- Is everything you will test written here? (build, no `.py`, exact output, the eight options)
- Is there anything you will not test? Remove it.
- Could the agent think `-j center` means "centre in 80 columns" when `-w 40` is also given? If the text is unclear, the twin will be wrong and it will be **your** fault, not the agent's.

**You should have:** a short instruction where every sentence can be checked.

---

## Step 4 — Make cases and freeze the answers (45 min)

**Ask the agent:**

```
Write tests/gen.py. It runs on MY laptop (not in Docker), with the pyfiglet folder from Step 1 next to the task folder.
- use random.seed(7),
- build 2000 visible cases into tests/data/visible.jsonl and a different 2000 into tests/data/hidden.jsonl,
- each case is one JSON line: {"args": [...pyfiglet command-line options and text...], "out": "..."},
- choose fonts from: standard, slant, small, big, mini, banner, shadow, script;
  widths from 10, 20, 40, 60, 80, 120; justify and direction from all their values; -r, -F, -s, -n each on in 1 of 4 cases,
- texts: mix normal words with these tricky ones: <paste your rare-cases list from NOTES.md>, plus | / \ _ [ ] { } ( ) < >, double spaces, long sentences that must wrap, Latin-1 letters like é ß ñ, and characters the font does not have,
- get "out" by running the ORIGINAL pyfiglet main() with those args and capturing stdout exactly.
Never write any "out" by hand.
```

**You check:**
- Run it twice, keep a copy in between, and `diff` the copies. They must be identical.
- `wc -l tests/data/*.jsonl`: 2000 each.
- Pick 3 lines, copy their `args`, run `python -m pyfiglet <args>` yourself, and compare.

**Hints:**
- The original is the truth. Never edit an `"out"` by hand.
- Check how many cases use `-D right-to-left` and narrow widths. If almost none, the hard rules are not tested. Ask for more.

**You should have:** two `.jsonl` files, the same every time you generate them.

---

## Step 5 — Write the grader and hide your secret (60 min)

Last time the grader was Python (Reward Kit). This room has no Python, so the grader is a small **Go** program. Bonus: it needs no internet.

**Ask the agent:**

```
Write tests/grade.go and tests/test.sh for this Harbor task. test.sh runs `cd /tests && go run grade.go`. grade.go:
1. build (weight 1): runs `go build -o /tmp/figlet .` in /workspace/repo; 1 if it works, else 0.
2. no_python (weight 1): 1 only if build passed AND no .py file exists under /workspace/repo AND at least one hidden case below matches; else 0.
3. behaviour (weight 4): for each case in /tests/data/hidden.jsonl, runs /tmp/figlet with its args (5 second timeout each); score = share of cases whose stdout equals "out" exactly.
4. Writes /logs/verifier/reward.json as {"reward": weighted mean, "build": .., "no_python": .., "behaviour": ..}.
5. SECRET: if behaviour is exactly 1.0, read /tests/secret.b64, base64-decode it, run /tmp/figlet -f slant with that text, and print the result after the line "*** SECRET UNLOCKED ***". If behaviour is below 1.0, print "No secret for you. Yet."
Use only the Go standard library. Show me the expected reward for: nothing done, build only, a perfect solution.
```

Then make your secret. Pick a short, friendly message for whoever solves your task (an agent, or a teammate):

```
printf 'nice port, human' | base64 > tests/secret.b64
```

**You check:**
- Your own maths: nothing = 0; build only = 1/6 ≈ 0.17; perfect = 1.0.
- Does `behaviour` read **hidden.jsonl**? Open the file and look.
- Is `no_python` tied to the build *and* to at least one right answer? (Step 7 explains why.)

**Hints:**
- The secret is printed in the verifier log (`verifier/test-stdout.txt` in the job folder), drawn by the twin itself. A broken twin draws a broken secret, which is the joke.
- Never put the secret in `instruction.md` or in the image.

**You should have:** `grade.go`, `test.sh`, `secret.b64`, and three rewards you worked out by hand.

---

## Step 6 — Write the solution, the oracle (120 min)

The longest step. The agent will get the big letters right quickly and the squeezing rules wrong for a long time.

**Ask the agent:**

```
Write solution/solve.sh. It must write a Go port of pyfiglet's FIGlet rendering into /workspace/repo (go.mod plus Go source, standard library only), move the .flf fonts it needs to a folder inside the repository (or embed them with go:embed), and then delete every .py file. The program must accept the options listed in instruction.md and match pyfiglet byte for byte.
Port the smushing rules from FigletBuilder one by one; do not invent your own.
```

Then:

```
harbor run -p ./figlet-go -a oracle
```

**You check:** the reward must be **1.0**, and the verifier log must show your secret drawn in giant letters. If not:
- Ask for a small script that runs `/tmp/figlet` on `visible.jsonl` and shows the **first 5 cases that differ**, with args, expected and actual (use `diff` on the two outputs).
- Paste those 5 back: "These are wrong. Fix them without breaking the others."
- Repeat until 0 differ.

**Hints:**
- Fix one rule at a time. Right-to-left and `-j right` both add spaces on the left; mixing them up is the most common mistake.
- If `go build` fails only inside Harbor, remember: the agent runs as `player`. Everything it writes must be in a folder `player` owns.
- Below 1.0 means the task is **broken**, not hard. Do not move on.

**You should have:** **oracle = 1.0**, your secret on screen, and a list of the rules the agent kept getting wrong.

---

## Step 7 — Nop and cheat check (30 min)

```
harbor run -p ./figlet-go -a nop
```

**Ask the agent:**

```
Make cheat/solve.sh. It must NOT port anything. It writes a tiny Go program that looks up its arguments in a copy of tests/data/visible.jsonl and prints the stored "out" if found, else nothing. It deletes the .py files. I want to prove my tests catch this.
```

Copy `cheat/solve.sh` over `solution/solve.sh` (keep the real one!), run the oracle command, put the real one back.

**You check:**
- nop: **0.0**.
- cheat: **below 0.2**. It builds (1/6) but knows none of the hidden cases, so `no_python` also pays 0.
- And the cheat gets **no secret**. Look at its verifier log: "No secret for you. Yet."

**Hints:**
- Cheat at about 0.33? Your `no_python` check pays for just deleting files. Go back to Step 5, rule 2.
- Cheat high? Your behaviour check reads the visible file.

**You should have:** **nop = 0.0**, **cheat below 0.2**, no secret for the cheat.

---

## Step 8 — Hand in, and unlock this page's secret (15 min)

- Push the task folder to GitHub, in your own folder.
- Add three real screenshots: oracle, nop, cheat rewards (real terminal captures, please; we check).
- Write two lines: which smushing rule broke the agent most, and which Docker trap you hit.

**Now the secret this page has been hiding.** It only shows up properly if your twin is right. Inside your built image, with your oracle's Go code:

```
cd /workspace/repo && go build -o /tmp/figlet . && /tmp/figlet -f slant "$(echo Z28gdG91Y2ggZ3Jhc3M= | base64 -d)"
```

If the letters come out clean, you earned it. If they come out mangled, so does your exam.

---

## Self-check before you hand in

- [ ] `NOTES.md`: URL, SHA, date, MIT, the font setup step, six smushing rules
- [ ] Image: two stages, Alpine Go, `bash`, user `player`, `NO_PYTHON`, pinned
- [ ] `instruction.md`: every sentence testable, no secret, no test names
- [ ] `gen.py`: seeded, answers from the original only, same output twice
- [ ] Grader in Go: build 1, no_python 1 (tied to a right answer), behaviour 4 on hidden cases
- [ ] oracle 1.0 with your secret drawn; nop 0.0; cheat below 0.2 with no secret
- [ ] Real screenshots and two honest lines
