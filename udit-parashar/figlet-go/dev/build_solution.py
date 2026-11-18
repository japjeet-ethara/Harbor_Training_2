#!/usr/bin/env python3
"""Author-side helper: regenerate solution/solve.sh from the tested Go sources in dev/oracle_src/.
(solve.sh must be self-contained because Harbor only copies solution/ into the container.)"""
from pathlib import Path

HERE = Path(__file__).resolve().parent
TASK = HERE.parent
SRC = HERE / "oracle_src"
D = "__GO_SOURCE_EOF__"

parts = ['''#!/bin/bash
# Oracle solution: a Go port of pyfiglet's FIGlet rendering (FigletFont parsing, FigletBuilder
# smushing / wrapping / justification, FigletString reverse / flip / strip / normalize, and the
# option handling of main()). Standard library only.
set -euo pipefail
cd /workspace/repo

# 1. fonts: keep the .flf files inside the repository, in a folder that is embedded with go:embed
mkdir -p fonts
cp pyfiglet/fonts-standard/*.flf fonts/

# 2. the Go source
''']
for name in ("go.mod", "main.go", "font.go", "render.go"):
    body = (SRC / name).read_text()
    assert D not in body
    parts.append(f"cat > {name} <<'{D}'\n{body}{D}\n\n")
parts.append('''# 3. the Python is gone
find /workspace/repo -name '*.py' -delete

# 4. self-check, and make sure the files belong to whoever the verifier runs as
go build ./... && go build -o /tmp/figlet .
if [ "$(id -u)" = "0" ]; then chown -R player:player /workspace/repo 2>/dev/null || true; fi
chmod -R a+rX /workspace/repo
echo "oracle: Go port written and built"
''')
out = TASK / "solution" / "solve.sh"
out.write_text("".join(parts))
out.chmod(0o755)
print("wrote", out, out.stat().st_size, "bytes")
