#!/usr/bin/env bash
set -euo pipefail

image=kushal-figlet-room
docker image inspect --format 'Image={{.Id}} User={{.Config.User}} Workdir={{.Config.WorkingDir}} Env={{json .Config.Env}}' "$image"
test "$(grep -Ec '^FROM ' environment/Dockerfile)" = 2
grep -E '^FROM ' environment/Dockerfile
docker run --rm "$image" bash -ec '
    whoami
    test "$(whoami)" = player
    id
    test "$(id -u)" = 1000
    pwd
    test "$PWD" = /workspace/repo
    test "$(stat -c %u .)" = 1000
    ls -ld .
    count=$(find pyfiglet/fonts -name "*.flf" | wc -l)
    echo "Prepared FLF fonts: $count"
    test "$count" -gt 50
    test -f pyfiglet/__init__.py
    test ! -e .git && test ! -e test-fonts && test ! -e pyfiglet/fonts-contrib
    test "$CGO_ENABLED" = 0 && test "$GOTOOLCHAIN" = local
    ! command -v git
    ! command -v python
    ! command -v python3
    test -z "$(apk info | grep -Ei "^(python|py3-)" || true)"
    echo NO_PYTHON
    command -v bash
    if apk add --no-network python3; then
        echo Unexpected package installation >&2
        exit 1
    fi
    touch .write-check
    rm .write-check
    echo CAN_WRITE
    go version
    bash --version | head -1
'
