# figlet-go: The Machine That Shouts in Giant Letters

A Harbor task. The agent ports the Python FIGlet engine (pyfiglet) to Go, standard library only, inside a non-root Alpine container with no Python.

## Metadata

- Source: https://github.com/pwaller/pyfiglet
- Pinned commit: 255827efbc09d662875dad50fa0edc1e1263bf45
- Licence: MIT
- Image: golang:1.23-alpine, user player (uid 1000), bash installed, no Python
- Verifier: shared (default). harbor lint may warn "shared-verifier"; this is expected for local practice.

## Scoring

reward = (1 x build + 1 x no_python + 4 x behaviour) / 6

- build: go build -o /tmp/figlet . succeeds.
- no_python: build passed, no .py file left, and at least one hidden case matches.
- behaviour: share of 2000 hidden cases whose output equals pyfiglet exactly.

| Run | build | no_python | behaviour | reward | secret |
|---|---|---|---|---|---|
| oracle | 1.0 | 1.0 | 1.0 | 1.000 | shown |
| nop | 0.0 | 0.0 | 0.0 | 0.000 | not shown |
| cheat | 1.0 | 0.0 | 0.0 | 0.167 | not shown |

## The cheat

cheat/solve.sh does not port anything. It reads a copy of visible.jsonl and prints the stored output when the arguments match, then deletes the .py files. It builds, so build = 1. The hidden cases are different from the visible ones, so behaviour = 0, and no_python also pays 0 because it needs at least one correct hidden answer. Reward = 1/6 = 0.167.

## Layout

       figlet-go/
      task.toml
      instruction.md
      NOTES.md
      README.md
      environment/Dockerfile
      tests/ (gen.py, go.mod, grade.go, test.sh, secret.b64, data/visible.jsonl, data/hidden.jsonl)
      solution/solve.sh
      cheat/ (solve.sh, visible.jsonl)

## Run it

    harbor run -p figlet-go -a oracle     (expect 1.000)
    harbor run -p figlet-go -a nop        (expect 0.000)

For the cheat, copy cheat/solve.sh over solution/solve.sh and cheat/visible.jsonl into solution/, run the oracle command (expect 0.167), then restore the real solve.sh and delete solution/visible.jsonl.

## Reflection

Rule that broke the agent most: skipping characters the font does not draw. Real pyfiglet draws printable ASCII and the letters A/O/U with umlauts and the sharp s, and silently skips everything else (e acute, i diaeresis, n tilde). The first Go port drew glyphs for them, so 211 of 2000 visible cases failed until the port skipped those characters.

Docker trap I hit: Alpine plus a non-root user. When I tried to install an agent (claude-code) into the image, npm could not write to /usr/local/lib/node_modules as the player user, and the golang:alpine image has no Node at all.