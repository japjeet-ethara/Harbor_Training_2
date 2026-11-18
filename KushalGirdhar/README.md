# Kushal FIGlet Go

A Harbor exercise in porting the pinned pyfiglet CLI to Go. `instruction.md` is the candidate contract. `solution/` is the oracle, `tests/` contains the exam, and `cheat/` supplies the visible-answer replay for baseline validation.

## Verification

| Harbor agent | Build | Python removal | Hidden matches | Reward |
| --- | ---: | ---: | ---: | ---: |
| Oracle | 1 | 1 | 2,000/2,000 | 1.0 |
| Nop | 0 | 0 | 0/2,000 | 0.0 |
| Visible replay | 1 | 0 | 0/2,000 | 0.16666666666666666 |

The numerator is build + Python removal + four times the exact-match fraction. It is divided by six. A nop scores zero because every component is zero. A build with no matching output scores 1/6 because both output-dependent components are zero. A perfect, Python-free build scores (1 + 1 + 4)/6. The oracle's original verifier stdout contains the unlock line and artwork. The nop and visible replay print `No secret for you. Yet.`

Each split has 2,000 cases and includes every value of all eight graded options. Each Boolean flag occurs 500 times per split. `evidence/coverage.json` counts each measured option-value pair and tracks the narrow explicit right-to-left cases by width. `evidence/rejected-generation.jsonl` records inputs screened for glyph widths at or above the wrap boundary, duplicate effective requests, or other reported reasons in the generation run. The generator captures binary stdout from the original main function and restores its temporary argv and stdout bindings on every call. All frozen outputs contain visible artwork, and the 4,000 argument vectors are distinct.

## Run on Windows with Ubuntu in WSL

Install Docker Desktop and enable its WSL integration for Ubuntu. Open Ubuntu from Windows Terminal, enter the parent directory containing the extracted `kushal-figlet-go/` folder, and confirm `docker version` shows both client and server. Install Git and uv in Ubuntu if they are missing. The recorded run used Harbor 0.25.0, CPython 3.12.12, and Go 1.23.12 inside the image. The host generator uses only the Python standard library. The command `uvx --from harbor==0.25.0 harbor --version` resolves Harbor and its host dependencies.

```bash
uv python install 3.12.12
uvx --from harbor==0.25.0 harbor --version
git clone https://github.com/pwaller/pyfiglet.git
git -C pyfiglet checkout --detach 255827efbc09d662875dad50fa0edc1e1263bf45
mkdir -p pyfiglet/pyfiglet/fonts
cp pyfiglet/pyfiglet/fonts-standard/* pyfiglet/pyfiglet/fonts/
cd kushal-figlet-go
```

In a Bash session, enable `pipefail` so logged failures propagate. Create a fresh rerun folder to preserve the results shipped in `evidence/`.

```bash
set -o pipefail
reruns=$(mktemp -d "$PWD/../figlet-rerun.XXXXXX")
bash validation/reproduce.sh "$reruns/data" |& tee "$reruns/data.log"
docker build -t kushal-figlet-room environment/ |& tee "$reruns/docker-build.log"
bash validation/docker-check.sh |& tee "$reruns/docker-check.log"
uvx --from harbor==0.25.0 harbor lint . |& tee "$reruns/lint.log"
bash validation/local-oracle.sh "$reruns/local" |& tee "$reruns/differential.log"
```

Harbor lint reports `shared-verifier` and exits 1 with the required shared environment. Review its output before continuing. The shared verifier uses the same environment as the agent. Harbor stages verifier files for grading after the agent phase.

Run these from separate terminal tabs or one after another. The replay command stages a temporary task copy and installs the visible lookup in its oracle directory. The real solution stays in the original task.

```bash
bash validation/harbor-baseline.sh oracle "$reruns/jobs"
bash validation/harbor-baseline.sh nop "$reruns/jobs"
bash validation/harbor-baseline.sh cheat "$reruns/jobs"
```

The baseline wrapper prints the raw reward and verifier stdout after each job. For new screenshots, keep each run visible in its own terminal until it prints the reward and capture the actual terminal with the Windows Snipping Tool. Save the captures in the new rerun folder.

To verify the shipped file manifest, run `sha256sum -c SHA256SUMS` from the extracted task root. For the assignment's final page-unlock check, mount `solution/` read-only into the built image, run `bash /solution/solve.sh` as the image's default user, then execute the page-unlock command from the assignment. The actual command and render are recorded in `evidence/page-unlock-final.log`.

## Upload

GitHub upload is pending a destination. Once the repository and submissions directory are authorized, edit the paths below and run these commands in WSL. Extract the ZIP onto the WSL Linux filesystem to preserve the Unix script modes.

```bash
repo_url='https://github.com/OWNER/REPO.git'
task_path='/absolute/path/to/extracted/kushal-figlet-go'
destination='path/to/submissions/kushal-figlet-go'
git clone "$repo_url" figlet-submission
cd figlet-submission
git switch -c kushal-figlet-go
test ! -e "$destination" || exit 1
mkdir -p "$(dirname "$destination")"
cp -a "$task_path" "$destination"
git add -- "$destination"
git status --short
git ls-files --stage -- "$destination"
```

After reviewing the staged files and confirming that shell scripts show mode `100755`, commit and push the branch.

```bash
git -c user.name='Kushal Girdhar' -c user.email='kushal.girdhar@ethara.ai' commit -m 'Add Kushal FIGlet Go task'
git push -u origin HEAD
```

## Hand-in observations

- Smushing rule that caused the most observed trouble. None. The measured differential runs found no oracle rendering mismatch. The large-X rule differed from the assignment's opening example, and the original Python behavior resolved it.
- Docker trap that occurred. None of the three traps in the assignment occurred. A carriage return in the first Windows-written validation shell script broke that check after its runtime assertions. The Linux scripts were converted to LF and rerun successfully.
