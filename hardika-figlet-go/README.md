# Hardika's Harbor FIGlet task

Port the pinned pyfiglet CLI to Go. The task runs in a two-stage Alpine Go image under the `player` user. The oracle parses FIGlet glyph matrices, plans placements and line breaks, and serializes the plan. The Go verifier grades a held-out corpus using exact stdout comparisons.

[Candidate instructions](instruction.md) · [Upstream notes](NOTES.md) · [Acceptance and evidence](ACCEPTANCE.md) · [AI assistance](AI_ASSISTANCE.md)

## Set up Windows and WSL

Start Docker Desktop with Linux containers and WSL integration enabled for Ubuntu. In PowerShell, run `wsl -d Ubuntu`. Use the commands below in Ubuntu Bash. Install [uv](https://docs.astral.sh/uv/getting-started/installation/) in Ubuntu if needed. This task was verified with CPython 3.12.13, Harbor 0.25.0, and Go 1.23.12 inside the shipped image.

```bash
docker version
docker compose version
uv --version

mkdir -p ~/figlet-practice/hardika-figlet-go
cd ~/figlet-practice
# Replace YOUR_WINDOWS_USER with your Windows user directory.
unzip -q /mnt/c/Users/YOUR_WINDOWS_USER/Downloads/hardika-figlet-go.zip -d hardika-figlet-go
uv venv --python 3.12.13 .harbor-venv
uv pip install --python .harbor-venv/bin/python -r hardika-figlet-go/evidence/harbor-pip-freeze.txt
source .harbor-venv/bin/activate
harbor --version
```

## Regenerate the corpus

The generator needs CPython 3.12.13, Git, and the prepared sibling checkout. It uses the Python standard library. It verifies the reference commit and the selected font copies before use. Each output is captured from the reference CLI in memory.

```bash
git -c core.autocrlf=false clone https://github.com/pwaller/pyfiglet.git
git -C pyfiglet checkout --detach 255827efbc09d662875dad50fa0edc1e1263bf45
mkdir -p pyfiglet/pyfiglet/fonts
cp pyfiglet/pyfiglet/fonts-standard/* pyfiglet/pyfiglet/fonts/

run=$(mktemp -d "$PWD/recheck.XXXXXX")
python -B hardika-figlet-go/tests/gen.py \
  --out "$run/data" --report "$run/coverage.json" --rejections "$run/rejections.jsonl"
cmp hardika-figlet-go/tests/data/visible.jsonl "$run/data/visible.jsonl"
cmp hardika-figlet-go/tests/data/hidden.jsonl "$run/data/hidden.jsonl"
wc -l "$run"/data/*.jsonl
sha256sum "$run"/data/*.jsonl
```

The corpus covers all 576 combinations of font, width, justification, and direction in each split. Four flag schedules are shuffled independently, with exactly 500 activations per flag per split. [Coverage](evidence/coverage.json) includes intersections with text scenarios and narrow RTL. [Rejections](evidence/generation-rejections.jsonl) records candidates discarded during generation.

## Build and verify the image

```bash
docker build -t hardika-figlet-room hardika-figlet-go/environment
docker run --rm hardika-figlet-room sh -ec '
  whoami
  id
  pwd
  stat -c "%u:%g %n" /workspace/repo
  ls pyfiglet/fonts | wc -l
  ! command -v python3
  ! command -v python
  echo NO_PYTHON
  command -v bash
  touch write-probe && rm write-probe && echo CAN_WRITE
'
harbor lint --select ALL hardika-figlet-go
```

Harbor lint emits the assignment's `shared-verifier` warning and exits with status 1. The agent runs as `player`. Harbor uploads the test files for the verifier after the agent phase.

## Run the Harbor baselines

Run from the parent of the task folder. `mktemp` gives each rerun its own evidence directory. For each completed job, its trial's `verifier/reward.json` contains the four scores. The matching `test-stdout.txt` is the verifier output.

```bash
jobs=$(mktemp -d "$PWD/jobs-check.XXXXXX")
harbor run -p ./hardika-figlet-go -a oracle --job-name oracle -o "$jobs"
cat "$jobs"/oracle/*/verifier/reward.json
harbor run -p ./hardika-figlet-go -a nop --job-name nop -o "$jobs"
cat "$jobs"/nop/*/verifier/reward.json

replay=$(mktemp -d "$PWD/replay-check.XXXXXX")
mkdir "$replay/hardika-figlet-go"
cp -a hardika-figlet-go/{environment,task.toml,instruction.md,tests} "$replay/hardika-figlet-go/"
cp -a hardika-figlet-go/cheat "$replay/hardika-figlet-go/solution"
cp hardika-figlet-go/tests/data/visible.jsonl "$replay/hardika-figlet-go/solution/"
harbor run -p "$replay/hardika-figlet-go" -a oracle --job-name cheat -o "$jobs"
cat "$jobs"/cheat/*/verifier/reward.json
```

The replay solution looks up exact argument vectors in the visible corpus. In Windows Terminal, run the baseline and its reward command in the same tab, then use **Snipping Tool**, choose **Window**, and select the terminal to capture each result. The included captures were taken directly from the xterm windows that ran the commands.

The reward is `(build + no_python + 4 × behaviour) / 6`. A no-op yields 0. Build-only yields `1/6` because `no_python` requires a hidden match. A perfect run earns 1.

## Verify the archive and upload

From the extracted task directory, run `sha256sum -c SHA256SUMS`. The top-level archive entries include `task.toml`, `instruction.md`, `environment/`, `solution/`, and `tests/`.

GitHub upload is pending a destination. Run these commands from the extracted root in WSL with your approved repository URL and your configured Git identity.

```bash
git init -b main
git add .
git commit -m "Add Hardika FIGlet Harbor task"
git remote add origin YOUR_APPROVED_GITHUB_REPOSITORY_URL
git push -u origin main
```
