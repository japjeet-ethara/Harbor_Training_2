# figlet-go (Harbor practice task 2)

```
figlet-go/
├── task.toml                 agent user "player", public network, shared verifier
├── instruction.md            what the agent sees
├── NOTES.md                  your notes on pyfiglet (SHA, licence, smushing rules, options, rare cases)
├── environment/Dockerfile    two-stage room: Alpine+git → golang:1.23-alpine, no Python, user player, bash
├── tests/
│   ├── test.sh               runs `cd /tests && go run grade.go`
│   ├── grade.go              the grader (Go, stdlib only): build 1, no_python 1, behaviour 4
│   ├── secret.b64            your secret (base64) — drawn by a perfect twin only
│   ├── gen.py                makes the cases with the ORIGINAL pyfiglet (runs on your laptop)
│   └── data/{visible,hidden}.jsonl
├── solution/solve.sh         the oracle: writes the Go port, deletes every .py
├── cheat/{solve.sh,run_cheat.sh}   Step 7 cheat (lookup table) and a helper to run it safely
└── dev/check.py              shows the first failing cases of a built /tmp/figlet
```

Run order (from the folder that contains `figlet-go/` and `pyfiglet/`):

```
docker build -t figlet-room figlet-go/environment/
docker run --rm figlet-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; command -v python3 || echo NO_PYTHON; command -v bash; touch x && echo CAN_WRITE'

harbor run -p ./figlet-go -a oracle              # expect reward 1.0 and the secret in verifier/test-stdout.txt
harbor run -p ./figlet-go -a nop                 # expect 0.0
bash figlet-go/cheat/run_cheat.sh                # expect < 0.2, "No secret for you. Yet."; restores solution/ itself
```

Regenerate the cases (same output every time): `python3 figlet-go/tests/gen.py`
