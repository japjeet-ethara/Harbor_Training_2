#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=${REPO_ROOT:-/workspace/repo}
if [ ! -d "$repo_root" ]; then
    repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
fi
cd "$repo_root"
mkdir -p fonts

if [ -d pyfiglet ]; then
    find pyfiglet -type f \( -name '*.flf' -o -name '*.tlf' \) -exec mv -f {} fonts/ \;
fi

if ! find fonts -type f \( -name '*.flf' -o -name '*.tlf' \) -print -quit | grep -q .; then
    echo "No FIGlet fonts found under pyfiglet/" >&2
    exit 1
fi

cat > go.mod <<'EOF'
module figlet

go 1.20
EOF

cp "$script_dir/main.go.template" main.go
find . -type f -name '*.py' -delete

if find . -type f -name '*.py' -print -quit | grep -q .; then
    echo "Python source files remain in the repository" >&2
    exit 1
fi
