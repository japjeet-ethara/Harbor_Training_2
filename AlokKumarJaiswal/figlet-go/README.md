# figlet-go: Harbor task guide

This is a Harbor task that grades an evaluated agent's Go port of the pinned [pyfiglet](https://github.com/pwaller/pyfiglet) CLI by comparing its stdout byte-for-byte against 2000 hidden reference outputs. The task author runs verification independently. Coding agents write the implementation; ChatGPT mentors and reviews.

For the full build/process log, decisions, and workspace-level context, see the [root README](../README.md).

## Current result

| Run | build | no_python | behaviour | reward | Job |
| --- | --- | --- | --- | --- | --- |
| Oracle (real port) | 1 | 1 | 1 | 1.0 | `figlet-equivalence-oracle-01` |
| Nop (empty agent) | 0 | 0 | 0 | 0.0 | `figlet-equivalence-nop-01` |
| Cheat (visible lookup) | 1 | 0 | 0 | 1/6 | `figlet-equivalence-cheat-01` |

Screenshots: [oracle.png](screenshots/oracle.png), [nop.png](screenshots/nop.png), [cheat.png](screenshots/cheat.png).

## Task directory

| Path | Purpose |
| --- | --- |
| `instruction.md` | Agent contract: what to build, options, hard rules, grading scope. This is the evaluated agent's specification; it is not modified by this README. |
| `task.toml` | Harbor task configuration |
| `environment/Dockerfile` | Two-stage container build (Alpine source → Go Alpine final) |
| `solution/solve.sh` | Real oracle deployment script (deterministic tar.gz/base64 of the Go port and fonts) |
| `cheat/solve.sh` | Exact-visible-argument lookup cheat; standard library only, no rendering |
| `tests/test.sh` | Verifier entry point; enforces root execution, protects `/tests`, creates isolated caches |
| `tests/grade.go` | Go grader: build, dependency, no_python, behaviour checks, secret branch, reward |
| `tests/grade_deps_test.go` | Focused Go tests for the dependency checker |
| `tests/grade_isolation_test.go` | Focused Go tests for the UID/GID isolation boundary |
| `tests/gen.py` | Dataset generator; author-only, requires the reference clone |
| `tests/data/visible.jsonl` | 2000 visible test cases. Author-side development data used for comparison, mutation testing, and the deliberate cheat experiment. The starting container image does not contain visible or hidden answer files; Harbor supplies `/tests` for verification after the agent phase, and the current launcher protects `/tests` before any submitted program executes. The cheat (`cheat/solve.sh`) embeds this file specifically for the author-controlled cheating experiment — an evaluated agent does not normally receive visible answers. |
| `tests/data/hidden.jsonl` | 2000 hidden test cases. Protected by root ownership and removed group/other permissions at grading time. |
| `tests/secret.b64` | Encoded secret; purpose and gate described below, content not reproduced |
| `screenshots/` | Terminal screenshots of oracle, nop, and cheat Harbor runs |
| `jobs/` | Local historical job output directories; not runtime input |
| `NOTES.md` | Detailed source analysis of the pinned pyfiglet (smushing, CLI, wrapping, direction). Copied from the original pinned upstream checkout; source paths and line references refer to that checkout. Initial "not executed" statements describe the source-study phase; later runtime checks are recorded in its runtime addendum and VALIDATION.md. Retains its verified original bytes |
| `HANDIN.md` | Submission summary with rewards, screenshot links, reflections |
| `VALIDATION.md` | Detailed current and historical evidence record |
| `.gitignore` | Standard exclusions |

## Agent contract (summary)

The full contract is [instruction.md](instruction.md). Key requirements:

- `main` package and `go.mod` at `/workspace/repo`
- `go build -o /tmp/figlet .` and `go build ./...` must succeed
- Go standard library only
- No `.py` files remaining under `/workspace/repo`
- Exact stdout match including spaces and final newlines
- Options: `-f`, `-w`, `-j`, `-D`, `-r`, `-F`, `-s`, `-n`
- Fonts remain in the repository; executable works from any cwd
- Options `-c`, `-L`, `-l`, `-i` are outside scope

## Container and task configuration

**Dockerfile stages:**

1. `alpine:3.22.2@sha256:4b7ce07...` — clones pyfiglet at commit `255827ef`, asserts HEAD, removes `.git`/`fonts-contrib`/`test-fonts`, copies standard fonts into `pyfiglet/fonts/`.
2. `golang:1.23-alpine@sha256:383395b...` — installs bash, creates player (UID/GID 1000), copies prepared source into `/workspace/repo`, sets `CGO_ENABLED=0` and `GOTOOLCHAIN=local`.

**task.toml:**

| Setting | Value |
| --- | --- |
| Agent user | `player` |
| Agent timeout | 3600 s |
| Verifier mode | `shared` |
| Verifier user | `root` |
| Verifier timeout | 900 s |
| Network mode | `public` |
| Build timeout | 900 s |
| Artifacts | `[]` |

The container has 72 font-directory entries under `pyfiglet/fonts/` (67 `.flf` files plus `.flc` files and `__init__.py`). The oracle embeds only the 67 `.flf` files. `command -v python3` returns nothing (Python is absent from PATH in Alpine; this is not an exhaustive interpreter search).

## Dataset generation

Generator: `tests/gen.py`, seed 7. The pinned original CLI is the sole answer oracle. All reference outputs come from subprocess raw-byte capture to preserve CLI transformations (stdout detach, additional newline).

Coverage across each set:

- Fonts: standard, slant, small, big, mini, banner, shadow, script
- Widths: 10, 20, 40, 60, 80, 120
- Justification: auto, left, center, right
- Direction: auto, left-to-right, right-to-left
- Boolean flags: `-r`, `-F`, `-s`, `-n` (independently sampled, 25% each)
- Text categories: single words, punctuation/smushing combos, Latin-1, missing glyphs, wrapping sentences, double spaces, hyphen-prefixed strings, generated phrases
- `--` separates options from text in every record

Safety filters: five-second reference timeout, glyph-width prefilter (rejects glyph widths ≥ selected width), duplicate rejection.

### Equivalence-overlap design

A normalized equivalence key captures: font, width, resolved direction (auto → left-to-right for all eight fonts), resolved justification (auto follows direction), reverse flag, flip flag, newline mode (`-s` takes precedence over `-n`), and exact text. During hidden generation, candidates matching any visible or previously accepted hidden equivalence key are rejected.

Current generation: 4039 candidates, 4000 accepted. Rejections: duplicate 7, unsafe_width 10, equivalent_visible 13, equivalent_hidden 9; timeouts/nonzero/stderr all zero. Independent audit: 1993 unique visible keys, 2000 unique hidden keys, zero shared keys, zero answerable hidden records.

| Dataset | SHA-256 |
| --- | --- |
| visible.jsonl | `f38c2347...937790c` |
| hidden.jsonl (current) | `43f27eab...88b0d30e` |

RTL narrow cases: visible 261, hidden 264.

## Grader checks and formula

The grader (`tests/grade.go`) runs as root. It is a Go program using only the standard library; no Reward Kit or Python dependency at grading time.

```
reward = (build + no_python + 4 × behaviour) / 6
```

**build** requires:

1. `go build -o /tmp/figlet .` succeeds (120 s timeout, as player)
2. `go build ./...` succeeds (120 s timeout, as player)
3. `go list -deps -json ./...` reports only standard-library packages or packages belonging to the solution's own main module within `/workspace/repo`, with no resolved dependency supplied through a module replacement (as player)

**no_python** requires all of: build passes, no `.py` files under `/workspace/repo`, and at least one hidden case matches.

**behaviour**: fraction of 2000 exact-output matches. Each case runs `/tmp/figlet` with the record's args, cwd `/tmp`, five-second timeout, as player.

**Secret gate**: only upon 2000/2000 hidden matches. Reads encoded content from `tests/secret.b64`, decodes, renders a FIGlet banner using the solution binary. The secret file is absent from the starting image.

Reward is written only after the secret branch completes. Dataset errors are verifier errors, not silent empty grading.

## Security boundary

`test.sh` runs as root and:

1. Rejects symlinks in `/tests`
2. Makes `/tests` root-owned, removes group/other permissions recursively
3. Creates root-owned `/logs/verifier` with mode 0700
4. Creates separate Go caches: root-owned for the trusted grader, player-owned for solution commands

`grade.go` launches all solution commands (`go build`, `go list`, `/tmp/figlet`, banner) as UID 1000, GID 1000, with supplementary groups cleared. The trusted grader retains root to read grading data.

Focused isolation test measured: uid 1000, gid 1000, groups 1000; root could open hidden.jsonl; player probe got permission denied. This is a measured access boundary, not a complete sandbox security proof.

**Warning:** do not run `test.sh` as root against writable host bind mounts. It recursively changes `/tests` ownership and permissions. Use Harbor's copied tests or disposable container copies.

## Oracle and cheat distinction

**Oracle** (`solution/solve.sh`): a faithful Go port of pyfiglet with 67 embedded `.flf` fonts via `go:embed`. Implements the full FIGlet rendering pipeline: font parsing, old-layout conversion, horizontal smushing (six rules + universal), kerning, word wrapping, justification, direction, reverse, flip, `-s`/`-n` normalization. Generated by `build_solve.py`, which verifies font and licence bytes match the reference clone. No interpreter or network required to deploy. Matched 2000/2000 visible and 2000/2000 hidden cases.

**Cheat** (`cheat/solve.sh`): a standard-library Go program that embeds `visible.jsonl` and performs exact argument-list lookup. No rendering algorithm. Produces empty stdout for unrecognized arguments. Deletes `.py` files. Expected and measured result: build=1, no_python=0 (zero hidden matches gates this), behaviour=0, reward=1/6, no secret unlock. Generated by `build_cheat.py`.

## Validation jobs and screenshots

| Run | Job | Trial ID | Screenshot |
| --- | --- | --- | --- |
| Oracle | `figlet-equivalence-oracle-01` | `figlet-go__LASstJC` | [oracle.png](screenshots/oracle.png) |
| Nop | `figlet-equivalence-nop-01` | `figlet-go__gzCSSoA` | [nop.png](screenshots/nop.png) |
| Cheat | `figlet-equivalence-cheat-01` | `figlet-go__PkD9GZQ` | [cheat.png](screenshots/cheat.png) |

Historical pre-equivalence results (original hidden dataset) are recorded in [VALIDATION.md](VALIDATION.md). Current results supersede them. All three current screenshots (`screenshots/oracle.png`, `screenshots/nop.png`, `screenshots/cheat.png`) were saved and visually reviewed, confirming actual job names, trial paths, and reward outputs.

## Safe reproduction commands

All commands use native Windows PowerShell. Go compilation and testing happen inside Docker. Commands are labeled **historical** (actually ran during development) or **reproduction** (for a reviewer to follow).

### Verified Python interpreter setup

For all Python helper scripts below, set the verified interpreter variable from the workspace root (parent directory of `figlet-go/`):

```powershell
# Workspace root setup block (native Windows PowerShell)
$pythonExe = 'C:\Users\AlokJaiswal\Desktop\6-Nov-first-harbor\.venv\Scripts\python.exe'
# Readers on another machine should specify their own verified Python 3.12 interpreter path.
```

### Build the Docker image (reproduction)

```powershell
# Working directory: figlet-go/
docker build -f environment/Dockerfile -t figlet-room environment
```

### Inspect the container (reproduction)

```powershell
# Working directory: figlet-go/
docker run --rm figlet-room sh -c "whoami; pwd; ls pyfiglet/fonts/ | wc -l; command -v python3 || echo NO_PYTHON; command -v bash"
```

### Regenerate datasets (reproduction, writes files)

Requires the pinned reference clone with prepared fonts at `pyfiglet/`. See [Section 1 of the root README](../README.md#1-source-study) for the reference clone and font preparation commands. (This reference setup is required only for author-side dataset generation and spot checks; a packaged Harbor oracle evaluation does not require local Python or the reference clone.)

```powershell
# Working directory: workspace root (parent of figlet-go/)
& $pythonExe -B '.\figlet-go\tests\gen.py'
```

This writes `figlet-go/tests/data/visible.jsonl` and `figlet-go/tests/data/hidden.jsonl`. The generator asserts visible hash stability and zero equivalence overlap. Output is deterministic across runs.

### Regenerate oracle solution (reproduction, writes file)

```powershell
# Working directory: workspace root
& $pythonExe -B '.\build_solve.py'
```

Requires `oracle_dev/` with Go sources and fonts matching `pyfiglet/pyfiglet/fonts-standard/*.flf`.

### Run Harbor oracle (reproduction, requires fresh job name)

```powershell
# Working directory: figlet-go/
$jobName = "figlet-oracle-fresh-01"
if (Test-Path "jobs/$jobName") { Write-Error "Job $jobName already exists"; return }
harbor run -p . -a oracle --job-name $jobName
```

### Run Harbor nop (reproduction, requires fresh job name)

```powershell
# Working directory: figlet-go/
$jobName = "figlet-nop-fresh-01"
if (Test-Path "jobs/$jobName") { Write-Error "Job $jobName already exists"; return }
harbor run -p . -a nop --job-name $jobName
```

### Run cheat evaluation (reproduction, requires fresh names)

The included `figlet-equivalence-cheat-01` job directory exists in `jobs/`, while the historical `solution_backup/` directory is intentionally excluded from this submission. `run_cheat.py` will refuse to rerun unchanged because the job name already exists. A new experiment requires fresh job and backup names; the script creates and verifies its backup.

To run a new cheat experiment:

```powershell
# Working directory: workspace root
# 1. Ensure cheat/solve.sh exists (build_cheat.py refuses overwrite; already generated)
# 2. Modify run_cheat.py job and backup names to fresh values, then:
& $pythonExe -B '.\run_cheat.py'
```

### Independent data validation (reproduction, read-only except spot checks run CLI)

```powershell
# Working directory: workspace root
& $pythonExe -B '.\verify_data.py'
```

Requires the pinned reference clone with prepared fonts. Checks format, type, count, uniqueness, hashes, and three visible spot checks.

### Run visible-only mutation check (reproduction, writes temp files only)

```powershell
# Working directory: workspace root
& $pythonExe -B '.\mutation_check.py'
```

Requires Docker image `figlet-room`. Currently tests only universal smushing. Operates on a temporary copy; original `oracle_dev/` is verified unchanged.

## Lint status

```powershell
# Working directory: figlet-go/
harbor lint . --select ALL
```

Only the shared-verifier warning, exit 1. This warning is deliberately accepted because the assignment requires shared verifier mode. The root verifier user is not the same as root agent.

## Limitations

- No paid real-model Harbor evaluation was performed. Task difficulty for independent agents is unknown.
- Hidden-dataset mutation sensitivity has not been measured.
- Big-X and hardblank smushing are disabled by all eight dataset fonts; all eight have automatic left-to-right direction.
- Kerning, wrapping, and transformation interactions have not been mutation-tested.
- The first port passed the frozen visible and hidden sets; this does not establish universal correctness.
- Security boundary is tested under verified conditions; universal exploit resistance is not claimed.
- Only the specified exact-argument lookup cheat has measured Harbor runs; normalized-lookup rewards are offline predictions only.
- The assignment's second secret was successfully rendered through the Go oracle; exit code 0. Evidence: [gotouchgrass.png](screenshots/gotouchgrass.png).

## Documents

| Document | Location |
| --- | --- |
| Agent contract | [instruction.md](instruction.md) |
| Source analysis | [NOTES.md](NOTES.md) |
| Submission summary | [HANDIN.md](HANDIN.md) |
| Evidence record | [VALIDATION.md](VALIDATION.md) |
| Root build log | [../README.md](../README.md) |

## Submission status

Submission copy assembled and verified: all 142 copied files matched their originals by SHA-256 before `.gitattributes` was added. Target: Harbor_Training_2/AlokKumarJaiswal. Files have been staged; commit and push are not yet complete.
