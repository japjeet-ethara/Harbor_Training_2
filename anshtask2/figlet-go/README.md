# figlet-go (Harbor practice task 2)

Files: task.toml, instruction.md, NOTES.md (template), environment/Dockerfile,
solution/solve.sh (oracle), tests/{test.sh,grade.go,gen.py,secret.b64}, cheat/ (generated),
scripts/{check_room.sh,make_cheat.py,diff5.py}, run_all.sh

Order
1. Step 1: clone pyfiglet NEXT TO this folder, checkout 255827ef..., do the font step
   (mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/), fill NOTES.md.
2. ./scripts/check_room.sh              -> "ALL SIX D-RULES PASS"
3. python3 tests/gen.py (twice + diff)  -> tests/data/{visible,hidden}.jsonl (add your RARE list in gen.py first)
4. Oracle loop (needs Go locally):
     rm -rf /tmp/work && cp -r ../pyfiglet /tmp/work
     REPO=/tmp/work bash solution/solve.sh && ( cp -r ../pyfiglet /tmp/work2 2>/dev/null; true )
     python3 scripts/diff5.py /tmp/work
   Paste the 5 diffs to Cursor, fix the Go in solve.sh, repeat until all match.
5. ./run_all.sh   -> oracle 1.0 (+secret in verifier/test-stdout.txt), nop 0.0, cheat < 0.2 (no secret)
Weights: build 1 + no_python 1 + behaviour 4. Cheat = 1/6 = 0.17.
