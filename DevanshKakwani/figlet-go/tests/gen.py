#!/usr/bin/env python3
"""Generate the visible and hidden test cases for the pyfiglet -> Go port.

Expected outputs come only from the original Python pyfiglet at the pinned
commit, invoked as `python -m pyfiglet ARGS...` in a subprocess. The oracle
tree is built the same way environment/Dockerfile builds the agent's repo:
checkout the pinned commit, then copy pyfiglet/fonts-standard/* into a new
pyfiglet/fonts/ folder.

Usage:
    python3 tests/gen.py [--source REPO] [--jobs N]

--source defaults to ../pyfiglet next to the task folder (the original
repository). If that is not a git repository containing the pinned commit,
the upstream repository is cloned into a temporary directory instead.

Writes tests/data/visible.jsonl and tests/data/hidden.jsonl by default,
2000 cases each. Each line
is {"args": [...], "out": "<exact stdout>"}. Only cases for which the oracle
exits with status 0 and writes nothing to stderr are kept.
"""

import argparse
import collections
import concurrent.futures
import json
import os
import random
import shutil
import subprocess
import sys
import tempfile

SEED = 7
N_VISIBLE = 2000
N_HIDDEN = 2000
REPO_URL = "https://github.com/pwaller/pyfiglet"
COMMIT = "255827efbc09d662875dad50fa0edc1e1263bf45"
# Normal runs take well under a second; anything slower is the known hang.
ORACLE_TIMEOUT = 10

HERE = os.path.dirname(os.path.abspath(__file__))
TASK_DIR = os.path.dirname(HERE)
# The original pyfiglet checkout lives beside the task folder.
DEFAULT_SOURCE = os.path.join(os.path.dirname(TASK_DIR), "pyfiglet")

UPPER = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
LOWER = "abcdefghijklmnopqrstuvwxyz"
DIGITS = "0123456789"
# Printable ASCII punctuation. Words never start with "-" (see make_word),
# so the option parser can never mistake text for an option.
PUNCT = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
GERMAN = "ÄÖÜäöüß"           # loaded by pyfiglet right after ASCII 126
UNSUPPORTED = "éñ€ž"          # usually missing from fonts -> silently skipped
COMMON_WORDS = [
    "Hello", "World", "hello", "world", "Go", "FIGlet", "pyfiglet", "Test",
    "OK", "abc", "XYZ", "The", "quick", "brown", "fox", "jumps", "over",
    "lazy", "dog", "Port", "to", "Go!", "2024", "v1.0.4", "C3PO", "R2-D2",
]


# --------------------------------------------------------------------------
# Oracle setup
# --------------------------------------------------------------------------

def run(cmd, **kw):
    return subprocess.run(cmd, check=True, stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE, **kw)


def export_commit(source, dest):
    """Write the files of COMMIT into dest. Prefer a local git repo."""
    try:
        run(["git", "-C", source, "cat-file", "-e", COMMIT + "^{commit}"])
        archive = run(["git", "-C", source, "archive", "--format=tar", COMMIT])
        run(["tar", "-x", "-C", dest], input=archive.stdout)
        return "local repository " + source
    except (subprocess.CalledProcessError, FileNotFoundError):
        pass
    clone = os.path.join(dest, "_clone")
    run(["git", "clone", "--quiet", REPO_URL, clone])
    run(["git", "-C", clone, "checkout", "--quiet", COMMIT])
    shutil.rmtree(os.path.join(clone, ".git"))
    for name in os.listdir(clone):
        shutil.move(os.path.join(clone, name), os.path.join(dest, name))
    os.rmdir(clone)
    return REPO_URL


def build_oracle(source, workdir):
    """Recreate the Dockerfile's /src tree and return its path."""
    src = os.path.join(workdir, "src")
    os.makedirs(src)
    origin = export_commit(source, src)
    # Same steps as stage 1 of environment/Dockerfile.
    shutil.rmtree(os.path.join(src, "pyfiglet", "fonts-contrib"), ignore_errors=True)
    shutil.rmtree(os.path.join(src, "test-fonts"), ignore_errors=True)
    fonts = os.path.join(src, "pyfiglet", "fonts")
    os.makedirs(fonts)
    std = os.path.join(src, "pyfiglet", "fonts-standard")
    for name in os.listdir(std):
        shutil.copy2(os.path.join(std, name), fonts)
    return src, origin


class Oracle:
    def __init__(self, src, workdir):
        self.src = src
        # An empty working directory: pyfiglet also looks for fonts in "./".
        self.cwd = os.path.join(workdir, "cwd")
        os.makedirs(self.cwd)
        self.env = {
            "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
            "PYTHONPATH": src,
            "PYTHONDONTWRITEBYTECODE": "1",
            "PYTHONIOENCODING": "utf-8",
            "PYTHONUTF8": "1",
            "LC_ALL": "C.UTF-8",
            "LANG": "C.UTF-8",
        }

    def __call__(self, args):
        """Run `python -m pyfiglet ARGS` against the oracle tree.

        Returns (returncode, stdout, stderr); returncode is None on timeout.
        The original never terminates when a glyph is exactly as wide as -w
        (cutBufferAtLastChar steps back onto the same character forever),
        so such cases have no expected output and are rejected.
        """
        try:
            p = subprocess.run([sys.executable, "-m", "pyfiglet"] + list(args),
                               cwd=self.cwd, env=self.env, timeout=ORACLE_TIMEOUT,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        except subprocess.TimeoutExpired:
            return None, b"", b""
        return p.returncode, p.stdout, p.stderr


def font_names(src):
    fonts = os.path.join(src, "pyfiglet", "fonts")
    return sorted(n[:-4] for n in os.listdir(fonts) if n.endswith(".flf"))


# --------------------------------------------------------------------------
# Case generation
# --------------------------------------------------------------------------

def make_word(rng):
    kind = rng.choices(
        ["common", "upper", "lower", "mixed", "digits", "punct", "alnum",
         "german", "unsupported"],
        weights=[20, 12, 12, 14, 8, 8, 16, 5, 5])[0]
    if kind == "common":
        w = rng.choice(COMMON_WORDS)
    else:
        n = rng.randint(1, 8)
        pool = {
            "upper": UPPER,
            "lower": LOWER,
            "mixed": UPPER + LOWER,
            "digits": DIGITS,
            "punct": PUNCT,
            "alnum": UPPER + LOWER + DIGITS + PUNCT,
            "german": LOWER + GERMAN,
            "unsupported": LOWER + UNSUPPORTED,
        }[kind]
        w = "".join(rng.choice(pool) for _ in range(n))
    if w.startswith("-"):
        w = "x" + w
    return w, kind


def make_text_args(rng, stats):
    """Return the list of text words (positional arguments)."""
    n_words = rng.choices([1, 2, 3, 4, 6], weights=[40, 25, 15, 12, 8])[0]
    words = []
    for _ in range(n_words):
        w, kind = make_word(rng)
        stats["word:" + kind] += 1
        words.append(w)
    style = rng.choices(["separate", "joined", "newline"], weights=[60, 32, 8])[0]
    stats["text:" + style] += 1
    if style == "separate" or n_words == 1:
        return words
    if style == "joined":
        return [" ".join(words)]
    # An explicit newline inside one argument.
    cut = rng.randint(1, n_words - 1)
    return [" ".join(words[:cut]) + "\n" + " ".join(words[cut:])]


def make_case(rng, fonts):
    stats = collections.Counter()
    opts = []

    if rng.random() < 0.9:
        opts.append(["-f", rng.choice(fonts)])
    if rng.random() < 0.55:
        opts.append(["-w", str(rng.choice(
            [rng.randint(10, 39), rng.randint(40, 100), rng.randint(101, 200)]))])
    if rng.random() < 0.5:
        opts.append(["-j", rng.choice(["auto", "left", "center", "right"])])
    if rng.random() < 0.3:
        opts.append(["-D", rng.choice(["auto", "left-to-right", "right-to-left"])])
    for flag, p in (("-r", 0.2), ("-F", 0.2), ("-s", 0.25), ("-n", 0.25)):
        if rng.random() < p:
            opts.append([flag])

    rng.shuffle(opts)
    text = make_text_args(rng, stats)

    # Usually options first; sometimes after or between the text words.
    placement = rng.choices(["before", "after", "mixed"], weights=[80, 10, 10])[0]
    flat_opts = [x for o in opts for x in o]
    if placement == "before" or not opts:
        args = flat_opts + text
        placement = "before"
    elif placement == "after":
        args = text + flat_opts
    else:
        k = rng.randint(1, len(opts))
        head = [x for o in opts[:k] for x in o]
        tail = [x for o in opts[k:] for x in o]
        args = head + text[:1] + tail + text[1:]
    stats["placement:" + placement] += 1
    return args, stats


def generate(rng, fonts, count, oracle, jobs, seen, batch=256):
    """Draw candidates in RNG order and keep the first `count` valid ones.

    The result depends only on the RNG sequence and the (deterministic)
    oracle, so running the oracle in parallel does not affect it.
    """
    kept, rejected = [], collections.Counter()
    with concurrent.futures.ThreadPoolExecutor(jobs) as pool:
        while len(kept) < count:
            cands = []
            while len(cands) < batch:
                args, stats = make_case(rng, fonts)
                key = tuple(args)
                if key in seen:
                    rejected["duplicate"] += 1
                    continue
                seen.add(key)
                cands.append((args, stats))
            for (args, stats), (rc, out, err) in zip(
                    cands, pool.map(oracle, [a for a, _ in cands])):
                if len(kept) == count:
                    break
                if rc is None:
                    rejected["timeout (pyfiglet hang)"] += 1
                    continue
                if rc != 0 or err:
                    rejected["exit %d" % rc] += 1
                    continue
                kept.append((args, out.decode("utf-8"), stats))
    return kept, rejected


# --------------------------------------------------------------------------
# Output and reporting
# --------------------------------------------------------------------------

def write_jsonl(path, cases):
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for args, out, _ in cases:
            f.write(json.dumps({"args": args, "out": out}, ensure_ascii=False) + "\n")


OPTION_VALUES = {"-f", "-w", "-j", "-D"}


def summarize(name, cases, rejected):
    c = collections.Counter()
    fonts = collections.Counter()
    for args, _, stats in cases:
        c.update(stats)
        i, used = 0, set()
        while i < len(args):
            a = args[i]
            if a in OPTION_VALUES and i + 1 < len(args):
                used.add(a)
                if a == "-f":
                    fonts[args[i + 1]] += 1
                elif a in ("-j", "-D"):
                    c["%s %s" % (a, args[i + 1])] += 1
                elif a == "-w":
                    w = int(args[i + 1])
                    c["-w " + ("10-39" if w < 40 else "40-100" if w <= 100 else "101-200")] += 1
                i += 2
                continue
            if a in ("-r", "-F", "-s", "-n"):
                used.add(a)
            i += 1
        for o in ("-f", "-w", "-j", "-D", "-r", "-F", "-s", "-n"):
            if o in used:
                c["uses " + o] += 1
        if "-f" not in used:
            fonts["(default standard)"] += 1
        if "-s" in used and "-n" in used:
            c["-s and -n together"] += 1
    print("== %s: %d cases, %d fonts used (+%d with no -f)" % (
        name, len(cases), len([f for f in fonts if not f.startswith("(")]),
        fonts["(default standard)"]))
    print("   rejected candidates: %s" % dict(rejected))
    for k in sorted(c):
        print("   %-24s %5d" % (k, c[k]))
    return fonts


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--source", default=DEFAULT_SOURCE,
                    help="git repo containing the pinned commit (default: ../pyfiglet)")
    ap.add_argument("--jobs", type=int, default=os.cpu_count() or 4)
    ap.add_argument("--outdir", default=os.path.join(HERE, "data"))
    opts = ap.parse_args()

    workdir = tempfile.mkdtemp(prefix="pyfiglet-oracle-")
    try:
        src, origin = build_oracle(opts.source, workdir)
        print("oracle: pyfiglet %s from %s" % (COMMIT, origin))
        oracle = Oracle(src, workdir)
        fonts = font_names(src)
        print("available .flf fonts: %d" % len(fonts))

        rng = random.Random(SEED)
        seen = set()
        visible, rej_v = generate(rng, fonts, N_VISIBLE, oracle, opts.jobs, seen)
        hidden, rej_h = generate(rng, fonts, N_HIDDEN, oracle, opts.jobs, seen)

        os.makedirs(opts.outdir, exist_ok=True)
        write_jsonl(os.path.join(opts.outdir, "visible.jsonl"), visible)
        write_jsonl(os.path.join(opts.outdir, "hidden.jsonl"), hidden)

        fv = summarize("visible", visible, rej_v)
        fh = summarize("hidden", hidden, rej_h)
        all_fonts = {f for f in list(fv) + list(fh) if not f.startswith("(")}
        print("== unique fonts across both files: %d of %d" % (len(all_fonts), len(fonts)))
        missing = sorted(set(fonts) - all_fonts)
        if missing:
            print("   never used: %s" % ", ".join(missing))
    finally:
        shutil.rmtree(workdir, ignore_errors=True)


if __name__ == "__main__":
    main()
