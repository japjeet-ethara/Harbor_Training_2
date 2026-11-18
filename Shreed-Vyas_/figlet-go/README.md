# figlet-go – Harbor task

Port pyfiglet (commit 255827e) to Go. See `instruction.md` for the agent-facing spec.

## Build order
```bash
# 1. clone + fonts
git clone https://github.com/pwaller/pyfiglet && cd pyfiglet
git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 && git log -1 --format='%H %cI'
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
cp pyfiglet/fonts-contrib/shadow.flf pyfiglet/fonts/shadow.flf
pip install -e .            # or just run from this directory

# 2. generate data from the REAL pyfiglet (writes tests/data/*.jsonl + environment/visible.jsonl)
python3 ../figlet-go/tests/gen.py
cd ../figlet-go

# 3. docker
docker build -t figlet-room environment/
docker run --rm figlet-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; command -v python3 || echo NO_PYTHON; command -v bash; touch x && echo CAN_WRITE'

# 4. debug the oracle locally against the VISIBLE set (prints first 3 mismatches)
docker run --rm -v $PWD/tests:/tests -v $PWD/solution:/solution figlet-room \
  bash -c 'bash /solution/solve.sh && GRADE_DATA=visible bash /tests/test.sh'

# 5. Harbor
harbor run -p . -a oracle       # expect 1.0
harbor run -p . -a nop          # expect 0.0
cp solution/solve.sh /tmp/real.sh && cp cheat/solve.sh solution/solve.sh
harbor run -p . -a oracle       # cheat: expect ~0.1667 (< 0.2)
cp /tmp/real.sh solution/solve.sh
```
Reward = (build + no-python + 4 x hidden-pass-rate) / 6. "No Python" credit needs >=1 correct case.

## Hand-in
LINE 1 (rules the agent got wrong): I did not run an AI agent on this task, so I have no measured list. What I can show from my own runs: even my reference solution scored 0.713 and 0.963 on the hidden set before it reached 1.0, so the task is not trivial. The actual fix was selecting `fonts-contrib/shadow.flf` for shadow because the standard shadow font did not match the frozen cases. I expect an agent to be most at risk on: mirroring and flipping characters for -r and -F, the -s/-n “last one wins” rule, word wrapping at narrow widths (10 and 20), and characters missing from a font.
LINE 2 (how to make the task harder): Remove visible.jsonl from the image so the agent must work only from the spec and the font files. Add more fonts from fonts-contrib, more code-tagged and Unicode characters, and more narrow-width and multi-line wrapping cases. Require -r and -F behaviour to be inferred from the pyfiglet source instead of being described.
