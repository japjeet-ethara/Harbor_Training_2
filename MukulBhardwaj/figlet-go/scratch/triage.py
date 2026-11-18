import json, subprocess, sys, collections

EXE = sys.argv[1]
cases = [json.loads(l) for l in open("tests/data/visible.jsonl", encoding="utf-8")]
stats = collections.defaultdict(lambda: [0, 0])
fails = []
for c in cases:
    a = c["args"]
    try:
        got = subprocess.run([EXE, *a], capture_output=True, timeout=5).stdout
    except subprocess.TimeoutExpired:
        got = b"TIMEOUT"
    ok = got == c["out"].encode("utf-8")
    keys = ["font=" + a[1], "width=" + a[3], "just=" + a[5], "dir=" + a[7]]
    keys += [f for f in ("-r", "-F", "-s", "-n") if f in a[8:-1]]
    text = a[-1]
    if not text.isascii():
        keys.append("non-ascii")
    if "  " in text:
        keys.append("double-space")
    if len(text) > 60:
        keys.append("long-text")
    for k in keys:
        stats[k][1] += 1
        stats[k][0] += ok
    if not ok:
        fails.append((len(c["out"]), c, got))

print(len(cases) - len(fails), "/", len(cases), "pass")
for k in sorted(stats):
    print(f"{k:20} {stats[k][0]:4}/{stats[k][1]:4}")
fails.sort(key=lambda x: x[0])
for _, c, got in fails[:3]:
    print("\nARGS:", c["args"])
    print("EXPECTED:", repr(c["out"]))
    print("GOT:     ", repr(got.decode("utf-8", "replace")))