# Komal's FIGlet Go task

This Harbor task asks an agent to reproduce the pinned pyfiglet CLI in Go. Start with the [candidate contract](instruction.md), [upstream notes](NOTES.md), or [acceptance record](ACCEPTANCE.md).

The oracle embeds all 67 standard `.flf` files already present in the repository. Glyphs and the active line use columns of rune cells. Per-row extents identify candidate overlaps, and an ordered resolver checks actual cell pairs and rule flags. The most recent space stores an immutable canvas checkpoint so the renderer can replay a word after wrapping.

Visible and hidden corpora contain 2,000 cases each. Each split covers every required font, width, justification, and direction combination at least three times. Each transform flag appears in exactly 500 cases. The generator shuffles the option order and invokes upstream `main()` for exact UTF-8 output. Glyphs at or over the requested width are screened because upstream can reject them or retry at the exact boundary. Whitespace-only outputs are excluded so a replay miss cannot pass by emitting nothing. See the [coverage manifest](tests/data/coverage.json).

The verifier awards `(build + no_python + 4 × behaviour) / 6`. A failed build gives 0. A build with no matches gives 1/6 because removing Python only earns its point after a match. A clean build that matches every hidden answer gives 1. Harbor's shared verifier copies in grading files after agent execution. The assignment expects the `shared-verifier` lint finding.

## Windows and WSL setup

Start Docker Desktop in Linux-container mode and enable WSL integration for Ubuntu. Open an Ubuntu terminal for the commands below. Check that `docker version`, `docker compose version`, `git --version`, and `uv --version` work. [uv installation](https://docs.astral.sh/uv/getting-started/installation/)

Run from the directory where you want to keep the extracted task and its sibling upstream checkout. If you have the ZIP, extract it into a new folder named `komal-figlet-go`. The ZIP has `task.toml` directly at its root.

```bash
unzip komal-figlet-go.zip -d komal-figlet-go
cd komal-figlet-go
sha256sum -c SHA256SUMS
cd ..
git -c core.autocrlf=false clone https://github.com/pwaller/pyfiglet
git -C pyfiglet checkout --detach 255827efbc09d662875dad50fa0edc1e1263bf45
mkdir -p pyfiglet/pyfiglet/fonts
cp pyfiglet/pyfiglet/fonts-standard/* pyfiglet/pyfiglet/fonts/
uv python install 3.12.12
uvx --from harbor==0.25.0 harbor --version
```

The generator uses CPython 3.12.12 and its standard library. The sibling checkout and Git are its other inputs.

## Regenerate the corpora

```bash
rerun="$(mktemp -d "$PWD/komal-rerun.XXXXXX")"
uv run --no-project --python 3.12.12 python komal-figlet-go/tests/gen.py --output "$rerun/data-a"
uv run --no-project --python 3.12.12 python komal-figlet-go/tests/gen.py --output "$rerun/data-b"
diff -qr "$rerun/data-a" "$rerun/data-b"
diff -qr "$rerun/data-a" komal-figlet-go/tests/data
wc -l "$rerun"/data-a/*.jsonl
```

## Build and inspect the room

```bash
docker build --progress=plain -t komal-figlet-room komal-figlet-go/environment
docker image inspect komal-figlet-room --format '{{.Config.User}} {{.Config.WorkingDir}}'
docker run --rm komal-figlet-room bash -c '
  set -eu
  whoami
  pwd
  ls pyfiglet/fonts | wc -l
  test "$(id -u)" = 1000
  test "$(stat -c %u:%g .)" = 1000:1000
  if command -v python3 || command -v python || apk info -e python3; then exit 1; fi
  echo NO_PYTHON
  command -v bash
  test ! -e .git && test ! -e pyfiglet/fonts-contrib && test ! -e test-fonts
  touch x && echo CAN_WRITE
'
uvx --from harbor==0.25.0 harbor lint --json komal-figlet-go
```

## Validate the baselines

Use the same `rerun` variable from the generation step, or create a fresh directory using its first command. The replay preparation makes a separate task copy with the visible lookup solution.

```bash
uvx --from harbor==0.25.0 harbor run -p komal-figlet-go -a oracle -o "$rerun/jobs" --job-name oracle
uvx --from harbor==0.25.0 harbor run -p komal-figlet-go -a nop -o "$rerun/jobs" --job-name nop
bash komal-figlet-go/validation/prepare-cheat.sh "$rerun/cheat-task"
uvx --from harbor==0.25.0 harbor run -p "$rerun/cheat-task" -a oracle -o "$rerun/jobs" --job-name cheat
find "$rerun/jobs" -name reward.json -print -exec cat {} \;
bash komal-figlet-go/validation/container-details.sh komal-figlet-room tests | tee "$rerun/go-tests.txt"
bash komal-figlet-go/validation/container-details.sh komal-figlet-room page-unlock | tee "$rerun/page-unlock.txt"
```

For fresh screenshots, keep the completed Harbor command and its reward JSON visible in the terminal. Use the Windows Snipping Tool's window capture and save each PNG to the rerun directory.

To run the standalone differential check, compile and copy the WSL-compatible binary out of a disposable container.

```bash
container="$(docker run --detach --rm komal-figlet-room sleep 600)"
docker cp komal-figlet-go/solution "$container:/tmp/solution"
docker exec "$container" bash /tmp/solution/solve.sh
docker exec "$container" go build -o /tmp/figlet .
docker cp "$container:/tmp/figlet" "$rerun/figlet"
docker rm -f "$container"
chmod +x "$rerun/figlet"
uv run --no-project --python 3.12.12 python komal-figlet-go/validation/differential.py "$rerun/figlet"
```

## Upload to a destination you control

With an authorized GitHub repository, copy the task folder into its local clone and make a commit. Use your existing Git identity. Run the following from that clone.

```bash
git add -- komal-figlet-go
git diff --cached --stat
git commit -m "Add Komal FIGlet Harbor task"
git push
```
