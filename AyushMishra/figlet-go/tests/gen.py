import io
import importlib
import json
import random
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


EXPECTED_SOURCE_COMMIT = "255827efbc09d662875dad50fa0edc1e1263bf45"
FONT_NAMES = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (10, 20, 40, 60, 80, 120)
JUSTIFICATIONS = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")
TRANSFORM_FLAGS = ("-r", "-F", "-s", "-n")
CASE_COUNT = 2000

RARE_TEXTS = (
    "|| |||",
    "/\\ \\/",
    "/\\/\\",
    "_| |_",
    "[] {} ()",
    ">< <>",
    "__ ___",
    "||--||",
    "/|\\",
    "\\/|",
    "hello  world",
    "A  B  C",
    "1234567890",
)

TEXTS = tuple(dict.fromkeys((
    "hello",
    "world",
    "FIGlet",
    "Go language",
    "ASCII art",
    "hello world",
    "The quick brown fox",
    "Pack my box with five dozen liquor jugs",
    "Bright vixens jump; dozy fowl quack.",
    "Small words make useful banners.",
    "HELLO pyfiglet",
    "one two three four five six seven eight",
    "The quick brown fox jumps over the lazy dog while curious people watch.",
    "A long sentence should wrap across a narrow terminal while keeping each word readable.",
    "When the width is small, this longer sentence continues on several lines for comparison.",
    "caf\u00e9 Stra\u00dfe ni\u00f1o",
    "Latin-1: \u00e9 \u00df \u00f1 \u00e0 \u00fc \u00e7",
    "unsupported \U0010fffd character",
    "hello \U0010fffd world",
    "| / \\ _ [ ] { } ( ) < >",
    "HELLO || ||| WORLD",
    "go /\\ \\/ now",
    "word  gap [ ]",
    "rare symbols /|\\ and \\/|",
    *RARE_TEXTS,
)))

REQUIRED_TEXTS = set(RARE_TEXTS) | {
    "| / \\ _ [ ] { } ( ) < >",
    "A  B  C",
    "word  gap [ ]",
    "The quick brown fox jumps over the lazy dog while curious people watch.",
    "A long sentence should wrap across a narrow terminal while keeping each word readable.",
    "When the width is small, this longer sentence continues on several lines for comparison.",
    "caf\u00e9 Stra\u00dfe ni\u00f1o",
    "Latin-1: \u00e9 \u00df \u00f1 \u00e0 \u00fc \u00e7",
    "unsupported \U0010fffd character",
    "hello \U0010fffd world",
}


class CapturedStdout:
    def __init__(self):
        self.binary = io.BytesIO()
        self.text = io.StringIO()

    def write(self, value):
        if isinstance(value, str):
            return self.text.write(value)
        return self.binary.write(value)

    def flush(self):
        return None

    def detach(self):
        return self.binary


def prepare_original_source(task_root):
    source_root = task_root.parent / "pyfiglet"
    package_root = source_root / "pyfiglet"
    standard_fonts = package_root / "fonts-standard"
    if not (package_root / "__init__.py").is_file() or not standard_fonts.is_dir():
        raise FileNotFoundError(
            "Expected the original pyfiglet checkout with pyfiglet/fonts-standard "
            f"next to the task folder: {source_root}"
        )

    try:
        revision = subprocess.run(
            ["git", "-C", str(source_root), "rev-parse", "HEAD"],
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
        changes = subprocess.run(
            ["git", "-C", str(source_root), "diff", "--quiet", "HEAD", "--"],
            check=False,
        )
    except (OSError, subprocess.CalledProcessError) as error:
        raise RuntimeError(f"Unable to verify the original pyfiglet checkout: {error}") from error
    if revision != EXPECTED_SOURCE_COMMIT or changes.returncode != 0:
        raise RuntimeError(
            "Expected a clean pyfiglet checkout at "
            f"{EXPECTED_SOURCE_COMMIT}; found revision {revision}"
        )

    temporary_root = Path(tempfile.mkdtemp(prefix="pyfiglet-original-"))
    temporary_package = temporary_root / "pyfiglet"
    temporary_package.mkdir()
    for filename in ("__init__.py", "version.py"):
        shutil.copy2(package_root / filename, temporary_package / filename)
    shutil.copytree(standard_fonts, temporary_package / "fonts")

    for module_name in tuple(sys.modules):
        if module_name == "pyfiglet" or module_name.startswith("pyfiglet."):
            del sys.modules[module_name]
    sys.path.insert(0, str(temporary_root))
    try:
        original_module = importlib.import_module("pyfiglet")
    except Exception:
        sys.path.remove(str(temporary_root))
        shutil.rmtree(temporary_root)
        raise

    return temporary_root, original_module


def capture_main(main, args):
    original_argv = sys.argv
    original_stdout = sys.stdout
    capture = CapturedStdout()
    sys.argv = ["pyfiglet", *args]
    sys.stdout = capture
    try:
        status = main()
        if status != 0:
            raise RuntimeError(f"Original pyfiglet main() returned {status} for {args!r}")
        return capture.binary.getvalue().decode("utf-8")
    finally:
        sys.argv = original_argv
        sys.stdout = original_stdout


def validate_cases(cases, count):
    if len(cases) != count:
        raise RuntimeError(f"Expected {count} cases, generated {len(cases)}")

    argument_sets = [tuple(case["args"]) for case in cases]
    if len(set(argument_sets)) != count:
        raise RuntimeError("Generated duplicate argument combinations")

    selected = {
        "font": set(),
        "width": set(),
        "justification": set(),
        "direction": set(),
    }
    for case in cases:
        args = case["args"]
        selected["font"].add(args[args.index("-f") + 1])
        selected["width"].add(int(args[args.index("-w") + 1]))
        selected["justification"].add(args[args.index("-j") + 1])
        selected["direction"].add(args[args.index("-D") + 1])
    expected_options = {
        "font": set(FONT_NAMES),
        "width": set(WIDTHS),
        "justification": set(JUSTIFICATIONS),
        "direction": set(DIRECTIONS),
    }
    if selected != expected_options:
        raise RuntimeError(f"Missing required option values: {selected!r}")

    transform_counts = {
        flag: sum(flag in case["args"] for case in cases)
        for flag in TRANSFORM_FLAGS
    }
    expected_transform_count = count // 4
    if any(value != expected_transform_count for value in transform_counts.values()):
        raise RuntimeError(f"Incorrect transform coverage: {transform_counts!r}")

    included_texts = {case["args"][-1] for case in cases}
    missing_texts = REQUIRED_TEXTS - included_texts
    if missing_texts:
        raise RuntimeError(f"Missing required tricky texts: {sorted(missing_texts)!r}")


def make_cases(figlet_fonts, original_main, count, used_args):
    transform_positions = {
        flag: set(random.sample(range(count), count // 4))
        for flag in TRANSFORM_FLAGS
    }
    cases = []
    attempts = 0

    while len(cases) < count:
        attempts += 1
        if attempts > count * 1000:
            raise RuntimeError(f"Could not generate {count} unique cases")

        index = len(cases)
        font_name = random.choice(FONT_NAMES)
        text = random.choice(TEXTS)
        font = figlet_fonts[font_name]
        required_width = max(
            (font.width[ord(character)] for character in text if ord(character) in font.width),
            default=0,
        )
        allowed_widths = [candidate for candidate in WIDTHS if candidate > required_width]
        if not allowed_widths:
            raise ValueError(f"No requested width can render {text!r} in {font_name!r}")
        width = random.choice(allowed_widths)

        args = [
            "-f", font_name,
            "-w", str(width),
            "-j", random.choice(JUSTIFICATIONS),
            "-D", random.choice(DIRECTIONS),
        ]
        args.extend(
            flag for flag in TRANSFORM_FLAGS
            if index in transform_positions[flag]
        )
        args.append(text)

        argument_key = tuple(args)
        if argument_key in used_args:
            continue

        output = capture_main(original_main, args)
        cases.append({"args": args, "out": output})
        used_args.add(argument_key)

    validate_cases(cases, count)
    return cases


def write_jsonl(path, cases):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as output_file:
        for case in cases:
            output_file.write(
                json.dumps(case, ensure_ascii=False, separators=(",", ":")) + "\n"
            )


def main_entry():
    random.seed(7)
    task_root = Path(__file__).resolve().parents[1]
    temporary_root, original_module = prepare_original_source(task_root)
    try:
        figlet_fonts = {
            name: original_module.FigletFont(name)
            for name in FONT_NAMES
        }
        original_figlet = original_module.Figlet

        class CachedFiglet(original_figlet):
            def setFont(self, **kwargs):
                if "font" in kwargs:
                    self.font = kwargs["font"]
                self.Font = figlet_fonts[self.font]

        original_module.Figlet = CachedFiglet
        data_root = task_root / "tests" / "data"
        used_args = set()
        visible_cases = make_cases(
            figlet_fonts, original_module.main, CASE_COUNT, used_args
        )
        hidden_cases = make_cases(
            figlet_fonts, original_module.main, CASE_COUNT, used_args
        )
        visible_args = {tuple(case["args"]) for case in visible_cases}
        hidden_args = {tuple(case["args"]) for case in hidden_cases}
        if visible_args & hidden_args:
            raise RuntimeError("Visible and hidden argument combinations overlap")

        write_jsonl(data_root / "visible.jsonl", visible_cases)
        write_jsonl(data_root / "hidden.jsonl", hidden_cases)
    finally:
        sys.path.remove(str(temporary_root))
        for module_name in tuple(sys.modules):
            if module_name == "pyfiglet" or module_name.startswith("pyfiglet."):
                del sys.modules[module_name]
        shutil.rmtree(temporary_root)


if __name__ == "__main__":
    main_entry()
