import io
import json
import multiprocessing
import random
import sys
from functools import lru_cache
from pathlib import Path
from typing import Iterable


TASK_DIR = Path(__file__).resolve().parents[1]
PYFIGLET_DIR = TASK_DIR.parent / "pyfiglet"
if not PYFIGLET_DIR.is_dir():
    raise FileNotFoundError(f"Expected the pyfiglet folder at {PYFIGLET_DIR}")

sys.path.insert(0, str(PYFIGLET_DIR))
from pyfiglet import CharNotPrinted, FigletFont, main as pyfiglet_main

TIMEOUT = 5  # seconds per case; a case that hangs is skipped

FONTS = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (20, 40, 60, 80, 120, 200)  # width 10 makes the original hang
JUSTIFICATIONS = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")

# TODO: replace/extend with the rare cases from YOUR NOTES.md.
RARE_CASES = ("ll", "l_", "_l", "|_|", "||", "/\\", "\\/", "[]", "_|", "|/")
NORMAL_WORDS = (
    "hello",
    "figlet",
    "terminal",
    "banner",
    "Go program",
    "bright morning",
    "open source",
    "friendly world",
)
SYMBOL_TEXT = r"|/\_[]{}()<>"
LONG_SENTENCES = (
    "A long sentence with many ordinary words is useful for checking how "
    "rendered text wraps when the available width is limited. "
)
TEXT_KINDS = ("normal", "rare", "symbols", "double-space", "long", "latin1", "missing")


class _StdoutCapture:
    def __init__(self) -> None:
        self.data = io.BytesIO()

    def write(self, value):
        if isinstance(value, str):
            self.data.write(value.encode("utf-8"))
            return len(value)
        return self.data.write(value)

    def flush(self) -> None:
        pass

    def detach(self) -> io.BytesIO:
        return self.data


def _balanced(values: Iterable, count: int) -> list:
    result = []
    values = tuple(values)
    while len(result) < count:
        block = list(values)
        random.shuffle(block)
        result.extend(block)
    return result[:count]


@lru_cache(maxsize=None)
def _font_widths(font: str) -> dict[int, int]:
    return FigletFont(font).width


def _text(kind: str) -> str:
    if kind == "normal":
        return " ".join(random.choice(NORMAL_WORDS) for _ in range(random.randint(1, 4)))
    if kind == "rare":
        return f"{random.choice(NORMAL_WORDS)} {random.choice(RARE_CASES)} {random.choice(NORMAL_WORDS)}"
    if kind == "symbols":
        return f"{random.choice(NORMAL_WORDS)} {SYMBOL_TEXT} {random.choice(NORMAL_WORDS)}"
    if kind == "double-space":
        return f"{random.choice(NORMAL_WORDS)}  {random.choice(NORMAL_WORDS)}   {random.choice(NORMAL_WORDS)}"
    if kind == "long":
        return LONG_SENTENCES * random.randint(2, 5)
    if kind == "latin1":
        return f"{random.choice(NORMAL_WORDS)} café Straße niño déjà vu ß ñ é"
    return f"{random.choice(NORMAL_WORDS)} 漢字 ☃ 🚀 {random.choice(NORMAL_WORDS)}"


def _capture_main(args: list[str]) -> str:
    old_argv = sys.argv
    old_stdout = sys.stdout
    capture = _StdoutCapture()
    try:
        sys.argv = ["pyfiglet", *args]
        sys.stdout = capture
        status = pyfiglet_main()
        if status not in (None, 0):
            raise RuntimeError(f"pyfiglet exited with status {status} for args {args!r}")
    finally:
        sys.argv = old_argv
        sys.stdout = old_stdout
    return capture.data.getvalue().decode("utf-8")


def _worker(conn) -> None:
    """Child process: renders cases one at a time so a hang can be killed."""
    while True:
        args = conn.recv()
        if args is None:
            break
        try:
            conn.send(("ok", _capture_main(args)))
        except CharNotPrinted:
            conn.send(("skip", None))
        except Exception:
            conn.send(("skip", None))


class Runner:
    def __init__(self) -> None:
        self._start()

    def _start(self) -> None:
        self.conn, child = multiprocessing.Pipe()
        self.proc = multiprocessing.Process(target=_worker, args=(child,), daemon=True)
        self.proc.start()

    def run(self, args: list[str]):
        try:
            self.conn.send(args)
            if self.conn.poll(TIMEOUT):
                return self.conn.recv()
        except (EOFError, OSError):
            pass
        print("SKIPPED (hang or crash):", args, flush=True)
        self.proc.terminate()
        self.proc.join()
        self._start()
        return ("skip", None)


def _make_cases(runner: Runner, count: int, seen: set[tuple[str, ...]], label: str) -> list[dict]:
    fonts = _balanced(FONTS, count)
    widths = _balanced(WIDTHS, count)
    justifications = _balanced(JUSTIFICATIONS, count)
    directions = _balanced(DIRECTIONS, count)
    text_kinds = _balanced(TEXT_KINDS, count)
    switches = {
        option: _balanced((True,) * (count // 4) + (False,) * (count - count // 4), count)
        for option in ("-r", "-F", "-s", "-n")
    }

    cases = []
    for index in range(count):
        font = fonts[index]
        width = widths[index]
        text = _text(text_kinds[index])
        while True:
            if any(_font_widths(font).get(ord(char), 0) >= width for char in text):
                font = random.choice(FONTS)
                width = random.choice(WIDTHS)
                text = _text(text_kinds[index])
                continue
            args = [
                "-f", font,
                "-w", str(width),
                "-j", justifications[index],
                "-D", directions[index],
            ]
            args.extend(option for option, values in switches.items() if values[index])
            args.append(text)
            while tuple(args) in seen:
                args[-1] = f"{args[-1]} {random.choice(NORMAL_WORDS)}"
            status, output = runner.run(args)
            if status != "ok":
                font = random.choice(FONTS)
                width = random.choice(WIDTHS)
                text = _text(text_kinds[index])
                continue
            seen.add(tuple(args))
            cases.append({"args": args, "out": output})
            if len(cases) % 200 == 0:
                print(label, len(cases), flush=True)
            break
    return cases


def _write_jsonl(path: Path, cases: list[dict]) -> None:
    with path.open("w", encoding="utf-8", newline="\n") as output:
        for case in cases:
            output.write(json.dumps(case, ensure_ascii=False, separators=(",", ":")))
            output.write("\n")


def main() -> None:
    random.seed(7)
    data_dir = TASK_DIR / "tests" / "data"
    data_dir.mkdir(parents=True, exist_ok=True)

    runner = Runner()
    seen: set[tuple[str, ...]] = set()
    _write_jsonl(data_dir / "visible.jsonl", _make_cases(runner, 2000, seen, "visible"))
    _write_jsonl(data_dir / "hidden.jsonl", _make_cases(runner, 2000, seen, "hidden"))
    print("done")


if __name__ == "__main__":
    main()