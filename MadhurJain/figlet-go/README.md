# Madhur Jain — figlet-go (Harbor practice task 2)

Port pyfiglet (commit 255827efbc09d662875dad50fa0edc1e1263bf45, MIT) to Go, byte for byte, in a two-stage Alpine room with no Python. My notes on pyfiglet are in NOTES.md.

| Run | Job | Reward | Screenshot |
| --- | --- | --- | --- |
| Oracle | oracle-2 | 1.0 (behaviour 2000/2000, secret drawn) | screenshots/oracle.png |
| Nop | nop-3 | 0.0 | screenshots/nop.png |
| Cheat (replays visible answers) | cheat-2 | 0.167 (behaviour 0/2000, no secret) | screenshots/cheat.png |

## What is where

- environment/Dockerfile: stage 1 (alpine, pinned by digest) clones pyfiglet at the commit and does the Makefile font setup; stage 2 (golang:1.23-alpine, pinned by digest) adds bash and user player.
- instruction.md: what the agent must build.
- tests/grade.go, tests/test.sh: Go grader run in the shared room after the agent stops. build 1 (go build, go build ./..., no require lines), no_python 1 (only after a hidden case passes), behaviour 4 (share of tests/data/hidden.jsonl matched byte for byte). tests/secret.b64 holds my message.
- tests/gen.py: makes tests/data/visible.jsonl and hidden.jsonl (2000 each, seed 7, no overlap) by running the original pyfiglet once per case. It skips cases where pyfiglet crashes, hangs, or draws nothing.
- solution/: the oracle, a hand-written Go port with the fonts embedded.
- cheat/: the cheat for the cheat run; it looks up its arguments in a copy of visible.jsonl.
- tools/compare.py: fix-loop helper that compares solution/go against a data file.

## A bug I found in my own exam

My first data set had 31 blank answers (texts like é or € alone draw nothing), so a cheat that only prints a newline scored 0.337. gen.py now skips blank drawings (blank=33 in its summary), and that cheat scores 0.167.

## Regenerating the data

gen.py needs the original next to this folder, with its font setup done:

    git clone https://github.com/pwaller/pyfiglet ../pyfiglet
    git -C ../pyfiglet checkout 255827efbc09d662875dad50fa0edc1e1263bf45
    mkdir -p ../pyfiglet/pyfiglet/fonts && cp ../pyfiglet/pyfiglet/fonts-standard/* ../pyfiglet/pyfiglet/fonts/
    uv run --no-project --python 3.12 python tests/gen.py

## Hand-in lines

**Smushing rule that broke the agent most:** None broke it: the port matched 2000/2000 visible and hidden on the first run. The rule that needed the most care in the prompt was right-to-left, which swaps left and right in smushing, so in universal smushing the left character wins.

**Docker trap I hit:** golang:1.23-bookworm contains Python 3, so I used golang:1.23-alpine; Alpine has no bash, so stage 2 runs apk add bash or Harbor cannot start.
