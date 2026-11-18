# Walkthrough: figlet-go (Practice Task 2)

| Step | Result (what was actually run) |
|---|---|
| 1 Understand pyfiglet | `NOTES.md`: URL, SHA 255827ef…, 2026-08-02, MIT, font-setup fix, smushing rules (examples computed from the original), 8 options, rare cases. |
| 2 Two-stage room | `environment/Dockerfile`, `task.toml`. NOT built here (no Docker). Run the build/check line from the task page yourself. |
| 3 instruction.md | Written. |
| 4 Cases | `tests/gen.py` -> `tests/data/visible.jsonl`, `hidden.jsonl` (2000 each, seeded, identical on re-run). |
| 5 Grader | `tests/grade.go`, `test.sh`, `secret.b64`. |
| 6 Oracle | `solution/solve.sh`. Simulated room: reward 1.0 (build 1, no_python 1, behaviour 1.0), secret drawn. |
| 7 Nop / cheat | nop 0.0. cheat 0.1667 (build 1 only), no secret. `cheat/solve.sh` + `cheat/visible.jsonl`. |
| 8 Hand in | You: push to GitHub, take real screenshots of the three harbor runs, write the two honest lines. |

Honest limits: Harbor and Docker were not available, so rewards come from a local simulation of the room
(/workspace/repo, /tests, /logs) using the same grader. Re-run with `harbor run -p ./figlet-go -a oracle|nop`
and capture your own screenshots.
