import argparse
from collections import Counter
from hashlib import sha256
from io import BytesIO, TextIOWrapper
from itertools import combinations, product
import json
from pathlib import Path
import random
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
REFERENCE = ROOT.parent / "pyfiglet"
SHA = "255827efbc09d662875dad50fa0edc1e1263bf45"
PYTHON = (3, 12, 12)
FONTS = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (10, 20, 40, 60, 80, 120)
JUSTIFY = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")
FLAGS = ("-r", "-F", "-s", "-n")
OPTION_NAMES = ("font", "width", "justify", "direction", *FLAGS)
OPTIONS = (FONTS, WIDTHS, JUSTIFY, DIRECTIONS, *((False, True),) * len(FLAGS))
SYMBOLS = ("|_|", "/\\", "\\/", "><", "[]{}", "()<>", "{[()]}", "][_/", "<|>\\")
LATIN = ("café", "Straße", "mañana", "cañón", "résumé", "größer", "Über", "señal", "déploiement")
MISSING = ("🛰", "雪", "λ", "\u0301", "🧪", "한")
TOOLS = (
    "git", "curl", "go", "bash", "tar", "jq", "rg", "rsync", "sed", "awk",
    "ssh", "ping", "dig", "openssl", "nmap", "strace", "tmux", "make", "docker",
    "netcat", "tcpdump", "iperf", "sqlite", "nginx", "redis",
)
SYSTEMS = (
    "API", "DNS", "VPN", "NAT", "TLS", "HTTP", "TCP", "UDP", "IPv6", "BGP",
    "edge", "router", "proxy", "cache", "worker", "queue", "replica", "resolver",
    "gateway", "relay", "pod", "build", "agent", "runner", "index", "pool",
)
EVENTS = (
    "up", "down", "ready", "slow", "retry", "drain", "reset", "offline", "stable",
    "drops packets", "fails open", "loses sync", "rejects traffic", "times out",
    "rotates keys", "restarts cleanly", "recovers state", "switches routes",
)
RELEASES = (
    "fix retry", "drop stale jobs", "add health check", "pin image", "trim queue",
    "roll back", "rotate cert", "flush cache", "sync peers", "ship patch",
    "close socket", "enable tracing", "split logs", "restore route", "replay journal",
    "merge release", "cap retries", "move shards", "warm standby", "verify lease",
)
LOCATIONS = (
    "at edge", "in prod", "during deploy", "on mesh", "from backup", "at noon",
    "on hot path", "after restart", "under load", "before cutover", "at ingress",
)
FEATURES = ("plain", "collision", "latin-1", "missing", "wrapping", "line breaks", "double space")


def reference_main(pyfiglet, args):
    previous_argv, previous_stdout = sys.argv, sys.stdout
    with BytesIO() as buffer:
        text = TextIOWrapper(buffer, encoding="utf-8", newline="\n")
        try:
            sys.argv = ["pyfiglet", *args]
            sys.stdout = text
            status = pyfiglet.main()
            if status != 0:
                raise RuntimeError(f"pyfiglet.main returned {status}")
            return buffer.getvalue().decode("utf-8")
        finally:
            sys.argv, sys.stdout = previous_argv, previous_stdout


def sample(feature):
    tool, system = random.choice(TOOLS), random.choice(SYSTEMS)
    event, release, location = random.choice(EVENTS), random.choice(RELEASES), random.choice(LOCATIONS)
    if feature == "plain":
        return [random.choice((f"{tool} {system}", f"{system} {event}", f"{release} {location}"))]
    if feature == "collision":
        return [f"{system} {random.choice(SYMBOLS)} {tool}"]
    if feature == "latin-1":
        return [f"{tool} {random.choice(LATIN)} {system}"]
    if feature == "missing":
        unknown = random.choice(MISSING)
        return [random.choice((f"{system}{unknown}{tool}", f"{tool} {unknown} {system}"))]
    if feature == "wrapping":
        return [f"{system} {event} {location}", f"{release} via {tool}"]
    if feature == "line breaks":
        return [random.choice((f"\n{system}\n{event}", f"{tool}\n\n{system} {event}\n",
                               f"{system} {event}\n{release}"))]
    return [f"{system}  {event}  {tool}"]


def option_args(config, mask):
    font, width, justify, direction = config
    groups = [["-f", font], ["-w", str(width)], ["-j", justify], ["-D", direction]]
    active_flags = [flag for bit, flag in enumerate(FLAGS) if mask >> bit & 1]
    random.shuffle(active_flags)
    groups.extend([flag] for flag in active_flags)
    random.shuffle(groups)
    return [argument for group in groups for argument in group]


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def make_split(pyfiglet, fonts, path, seen, rejected):
    settings = list(product(FONTS, WIDTHS, JUSTIFY, DIRECTIONS))
    random.shuffle(settings)
    slots = (settings * 4)[:2000]
    mask_counts = (634, 211, 70, 23, 10)
    masks = [mask for mask in range(16) for _ in range(mask_counts[mask.bit_count()])]
    random.shuffle(masks)
    rows = []
    columns = [Counter() for _ in OPTION_NAMES]
    pairs = {pair: Counter() for pair in combinations(range(len(OPTION_NAMES)), 2)}
    features = Counter()
    narrow_rtl = Counter()
    for index, (config, mask) in enumerate(zip(slots, masks), start=1):
        font, width, _, direction = config
        feature = FEATURES[(index - 1) % len(FEATURES)]
        options = option_args(config, mask)
        for _ in range(500):
            words = sample(feature)
            text = " ".join(words)
            key = (config, mask, text)
            args = [*options, *words]
            reason = None
            glyphs = fonts[font].width
            if key in seen:
                reason = "duplicate effective request"
            elif any(glyphs.get(ord(char), 0) >= width for char in text if char != " "):
                reason = "glyph width at or above wrap boundary"
            else:
                try:
                    out = reference_main(pyfiglet, args)
                except (Exception, SystemExit) as error:
                    reason = f"{type(error).__name__}: {error}"
                else:
                    if not out.strip():
                        reason = "no visible artwork"
            if reason is not None:
                rejected.write(encode({"split": path.stem, "args": args, "reason": reason}) + "\n")
                continue
            seen.add(key)
            rows.append(encode({"args": args, "out": out}))
            break
        else:
            raise RuntimeError(f"unable to fill {path.stem} slot {index}: {config}, {feature}")

        values = (*config, *(bool(mask >> bit & 1) for bit in range(len(FLAGS))))
        for counts, value in zip(columns, values):
            counts[value] += 1
        for (i, j), counts in pairs.items():
            counts[values[i], values[j]] += 1
        features[feature] += 1
        if direction == "right-to-left" and width in (10, 20):
            narrow_rtl[width] += 1

    data = ("\n".join(rows) + "\n").encode("utf-8")
    path.write_bytes(data)
    pair_report = {}
    for (i, j), counts in pairs.items():
        values = product(OPTIONS[i], OPTIONS[j])
        pair_report[f"{OPTION_NAMES[i]} × {OPTION_NAMES[j]}"] = {
            "observed": len(counts),
            "possible": len(OPTIONS[i]) * len(OPTIONS[j]),
            "counts": [[left, right, counts[left, right]] for left, right in values],
        }
    return {
        "rows": len(rows), "sha256": sha256(data).hexdigest(),
        "options": dict(zip(OPTION_NAMES, columns)),
        "text_families": features, "narrow_explicit_rtl_by_width": narrow_rtl,
        "option_pairs": pair_report,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=ROOT / "tests" / "data")
    parser.add_argument("--evidence", type=Path, default=ROOT / "evidence")
    args = parser.parse_args()
    if tuple(sys.version_info[:3]) != PYTHON:
        parser.error("use CPython 3.12.12")
    actual = subprocess.check_output(["git", "-C", str(REFERENCE), "rev-parse", "HEAD"], text=True).strip()
    if actual != SHA:
        parser.error(f"expected pyfiglet checkout at {SHA}, got {actual}")
    source_fonts = REFERENCE / "pyfiglet" / "fonts-standard"
    prepared = REFERENCE / "pyfiglet" / "fonts"
    if any(not (prepared / (font + ".flf")).is_file() or
           (prepared / (font + ".flf")).read_bytes() != (source_fonts / (font + ".flf")).read_bytes()
           for font in FONTS):
        parser.error("copy pyfiglet/fonts-standard/* into pyfiglet/fonts first")
    sys.path.insert(0, str(REFERENCE))
    import pyfiglet

    args.output.mkdir(parents=True, exist_ok=True)
    args.evidence.mkdir(parents=True, exist_ok=True)
    fonts = {font: pyfiglet.FigletFont(font) for font in FONTS}
    random.seed(7)
    seen = set()
    report = {"seed": 7, "python": ".".join(map(str, PYTHON)), "reference_sha": SHA}
    with (args.evidence / "rejected-generation.jsonl").open("w", encoding="utf-8", newline="\n") as rejected:
        for split in ("visible", "hidden"):
            report[split] = make_split(pyfiglet, fonts, args.output / f"{split}.jsonl", seen, rejected)
    report["distinct_effective_requests"] = len(seen)
    (args.evidence / "coverage.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n",
                                                  encoding="utf-8", newline="\n")


if __name__ == "__main__":
    main()
