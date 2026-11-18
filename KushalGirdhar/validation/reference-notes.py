from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "pyfiglet"))
import pyfiglet

for bit, left, right in (
    (1, "|", "|"), (2, "_", "/"), (4, "|", "]"), (8, "}", "{"),
    (16, "/", "\\"), (16, "\\", "/"), (16, ">", "<"), (16, "<", ">"),
    (32, "$", "$"), (64, " ", "|"), (64, "|", "|"),
):
    font = pyfiglet.FigletFont()
    font.smushMode = bit | 128 if bit < 64 else bit
    builder = pyfiglet.FigletBuilder("", font, "left-to-right", 80, "left")
    builder.prevCharWidth = builder.curCharWidth = 3
    print(f"{bit:2d} {left!r} + {right!r} -> {builder.smushChars(left, right)!r}")
