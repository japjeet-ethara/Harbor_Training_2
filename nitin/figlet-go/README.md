# figlet-go — Harbor task: port pyfiglet to Go

The agent gets a checkout of pyfiglet (pinned commit `255827efbc09d662875dad50fa0edc1e1263bf45`) in a room with Go and no Python,
and must produce a Go program whose output matches `python -m pyfiglet` byte for byte.

```
figlet-go/
  task.toml               agent user "player", timeouts, network_mode "public", shared verifier
  instruction.md          what the agent reads (no tests, answers or secret mentioned)
  NOTES.md                Step 1: SHA, date, MIT, font setup, six smushing rules + kerning, options, rare cases
  environment/Dockerfile  Step 2: two stages, no Python, bash, non-root "player", pinned tags
  tests/
    test.sh               runs `cd /tests && go run grade.go`
    grade.go              build (1) + no_python (1) + behaviour (4) -> /logs/verifier/reward.json, prints the secret
    secret.b64            the author's secret (base64 of the message)
    gen.py                Step 4: seeded generator; answers come only from the ORIGINAL pyfiglet main()
    data/visible.jsonl    2000 cases (for the author while debugging)
    data/hidden.jsonl     2000 other cases (what the grader uses)
  solution/solve.sh       Step 6: the oracle, a Go port (go:embed fonts, deletes every .py)
  cheat/solve.sh          Step 7: looks answers up in visible.jsonl; copy over solution/solve.sh to test
  dev/local_sim.sh        runs oracle / nop / cheat WITHOUT Docker (throw-away Linux box only)
  HANDIN.md               Step 8 checklist and the two honest lines
```

## Commands (Harbor + Docker machine)

```
docker build -t figlet-room environment/
docker run --rm figlet-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; command -v python3 || echo NO_PYTHON; command -v bash; touch x && echo CAN_WRITE'
harbor run -p ./figlet-go -a oracle     # expect reward 1.0 and "*** SECRET UNLOCKED ***" in verifier/test-stdout.txt
harbor run -p ./figlet-go -a nop        # expect 0.0
cp solution/solve.sh /tmp/real-solve.sh && cp cheat/solve.sh solution/solve.sh
harbor run -p ./figlet-go -a oracle     # expect below 0.2 and "No secret for you. Yet."
cp /tmp/real-solve.sh solution/solve.sh # put the real one back!
```

Regenerate the cases (needs the original pyfiglet next to this folder, with the Makefile font step done):

```
git clone https://github.com/pwaller/pyfiglet ../pyfiglet && git -C ../pyfiglet checkout 255827efbc09d662875dad50fa0edc1e1263bf45
mkdir -p ../pyfiglet/pyfiglet/fonts && cp ../pyfiglet/pyfiglet/fonts-standard/* ../pyfiglet/pyfiglet/fonts/
python3 tests/gen.py          # byte-identical output on every run (random.seed(7))
```

`harbor lint` prints one expected warning: `shared-verifier` (see the task page, Step 2).
