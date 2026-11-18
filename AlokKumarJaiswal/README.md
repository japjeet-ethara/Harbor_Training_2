# figlet-go: build log for a Harbor task

Practice Task 2 ports the pinned Python [pyfiglet](https://github.com/pwaller/pyfiglet) CLI to Go and creates a Harbor task that grades exact byte-for-byte rendering behavior. This file records what was built, how the work progressed, and the purpose of every workspace file. The task itself is described in [figlet-go/README.md](figlet-go/README.md).

Labels: **verified** = ran and saw the result. **inferred** = reasoning, not tested. **historical** = ran during an earlier phase; the current state may differ.

## Result

| Run | Reward | Job |
| --- | --- | --- |
| Oracle | 1.0 | `figlet-equivalence-oracle-01` |
| Nop | 0.0 | `figlet-equivalence-nop-01` |
| Exact-argument visible-lookup cheat | 1/6 (~0.167, below 0.2) | `figlet-equivalence-cheat-01` |

Reward = (build + no_python + 4 × behaviour) / 6. All three current screenshots ([screenshots/](figlet-go/screenshots/): `oracle.png`, `nop.png`, `cheat.png`) were saved and visually reviewed, confirming actual job names, trial paths, and reward JSON.

## Workspace layout

This table describes the original development workspace and submission contents. The GitHub submission includes `figlet-go/`, `oracle_dev/`, the root helper files, and `.gitattributes`. The reference clone, dataset backups, and historical `solution_backup/` directory are intentionally excluded from this submission; reference-clone setup commands are provided below.

| Path | Purpose |
| --- | --- |
| `figlet-go/` | Runnable Harbor task: instruction, Dockerfile, grader, datasets, solution, cheat, screenshots, author docs |
| `oracle_dev/` | Editable Go implementation (four `.go` sources, `go.mod`, MIT `LICENSE`, 67 embedded `.flf` fonts) |
| `.gitattributes` | Submission packaging configuration (`* -text`); preserves exact file bytes without Git line-ending conversion (including frozen JSONL data, fonts, and deployment scripts). Submission packaging configuration, not an original development helper |
| `pyfiglet/` | Pinned reference clone at commit `255827ef`; requires standard-font preparation before use |
| `data-first-run/` | Preserved baseline of the original dataset generation |
| `data-equivalence-first-run/` | Preserved baseline of the revised (equivalence-filtered) dataset generation |
| `solution_backup/` | Guarded backup directory created by `run_cheat.py` (historical development artifact; intentionally excluded from this submission) |

### Root helper files

| File | Purpose | Notes |
| --- | --- | --- |
| `verify_smushing.py` | Two original `smushChars` function checks (underscore mode 130, Big-X mode 144) | Both passed |
| `verify_reference.py` | 32 CLI capture checks (8 fonts × 4 flag combinations) | Verified `-s` and `-n` precedence |
| `verify_data.py` | Independent dataset validation: format, type, count, uniqueness, three visible reference spot checks | Does not rerun hidden answers |
| `dev_compare.go` | Visible-only Go comparison runner; mounts into Docker container | Used for mutation baseline |
| `build_solve.py` | Verifies font/licence bytes, generates `solution/solve.sh` as deterministic tar.gz/base64 | Writes output; not read-only |
| `build_cheat.py` | Generates `cheat/solve.sh` with exact-visible-argument lookup; refuses overwrite if output exists | Writes output; not read-only |
| `run_cheat.py` | Guarded Harbor swap/run/restore cycle with backup verification | Uses fixed job `figlet-equivalence-cheat-01` and backup name. The included job directory exists, while historical `solution_backup/` is intentionally excluded; rerunning unchanged will refuse because the job name already exists. A new experiment requires fresh job and backup names; the script creates and verifies its backup |
| `probe_hidden_access.go` | Opens/closes hidden file path without reading content; used to test isolation boundary | Measured permission denied for UID 1000 |
| `mutation_check.py` | Temporary-copy mutation experiment against visible dataset in Docker | Currently implements only the universal-smushing mutation; earlier rules tested by modifying the single target between runs |
| `audit_equivalent_args.py` | Independent aggregate normalization audit across visible/hidden datasets | Expects current dataset hashes |
| `verify_grader_build_only.sh` | Historical pre-isolation build-only fixture; assumes player-run verifier | Not compatible with the current root-required `test.sh` without modification |

Python helpers generally locate workspace inputs relative to `__file__`. Go and shell tools use their documented container paths: `dev_compare.go` expects `/cases/visible.jsonl` and `/tmp/figlet`; `probe_hidden_access.go` checks `/tests/data/hidden.jsonl`. The workspace's sibling directory layout (`pyfiglet/`, `figlet-go/`, `oracle_dev/`) is required by the author workflows.

### Author workflow commands

To run these helpers from the workspace root, set the verified interpreter variable first:

```powershell
# Workspace root setup block (native Windows PowerShell)
$pythonExe = 'C:\Users\AlokJaiswal\Desktop\6-Nov-first-harbor\.venv\Scripts\python.exe'
# Readers on another machine should specify their own verified Python 3.12 interpreter path.

# Checks that preserve the original task files:
& $pythonExe -B '.\verify_smushing.py'
& $pythonExe -B '.\verify_reference.py'
& $pythonExe -B '.\verify_data.py'
& $pythonExe -B '.\audit_equivalent_args.py'
& $pythonExe -B '.\mutation_check.py'        # Writes temporary files during Docker test; restores and verifies original state

# Output-generating helpers (author workflows that write or modify task files):
& $pythonExe -B '.\figlet-go\tests\gen.py'   # Writes tests/data/visible.jsonl and hidden.jsonl
& $pythonExe -B '.\build_solve.py'          # Writes figlet-go/solution/solve.sh
& $pythonExe -B '.\build_cheat.py'          # Writes figlet-go/cheat/solve.sh (refuses overwrite if target exists)
& $pythonExe -B '.\run_cheat.py'            # Guarded swap/run/restore (refuses if job or backup exists)
```

The historical helper `verify_grader_build_only.sh` was written for an earlier pre-isolation environment where the verifier ran as `player`; it is not directly compatible with the current root-required `test.sh`.

## 0. Environment reused

Native Windows PowerShell, Docker Desktop with Linux containers, amd64. Existing tools were reused rather than reinstalled:

- Harbor 0.25.0
- Python 3.12.16
- Git 2.56.0.windows.4
- `uv` available; exact version not recorded
- Go runs inside Docker only, not installed locally (container: go1.23.12 linux/amd64)

Existing interpreter: `C:\Users\AlokJaiswal\Desktop\6-Nov-first-harbor\.venv\Scripts\python.exe`

Environment variables already set from Task 1: `PYTHONUTF8=1`, `PYTHONDONTWRITEBYTECODE=1`, `git core.autocrlf=input`.

Work began in Cursor. Switched to Antigravity after reaching Cursor's available-model limit.

## 1. Source study

The following PowerShell reproduction block demonstrates the reference checkout and font preparation:

```powershell
# Prerequisites from workspace root (C:\Users\AlokJaiswal\Desktop\16-Nov-second-harbor)
$pythonExe = 'C:\Users\AlokJaiswal\Desktop\6-Nov-first-harbor\.venv\Scripts\python.exe'
# Readers on another machine should specify their own verified Python 3.12 interpreter path.

# This reproduction block is strictly for a fresh checkout.
# If .\pyfiglet already exists, it must be checked for pinned commit 255827efbc09d662875dad50fa0edc1e1263bf45 and prepared fonts before use; directory existence alone does not prove readiness.
if (Test-Path ".\pyfiglet") {
    Write-Host "Directory .\pyfiglet already exists. An existing directory must be checked for pinned commit 255827efbc09d662875dad50fa0edc1e1263bf45 and prepared fonts before use; directory existence alone does not prove readiness. Do not rerun this setup block."
    return
}

# 1. Clone into .\pyfiglet and verify exit code
git clone https://github.com/pwaller/pyfiglet .\pyfiglet
if ($LASTEXITCODE -ne 0) {
    Write-Error "git clone failed with exit code $LASTEXITCODE"
    return
}

# 2. Inside the clone, select the pinned commit and inspect metadata
Set-Location .\pyfiglet
git checkout 255827efbc09d662875dad50fa0edc1e1263bf45
git rev-parse HEAD
git log -1 --format="%cI"

# 3. Prepare standard fonts inside the clone:
# The loader expects pyfiglet\fonts, but the repository stores standard fonts in pyfiglet\fonts-standard.
# Note the difference between path perspectives:
#   Workspace-root paths:
#     Source:      .\pyfiglet\pyfiglet\fonts-standard\*
#     Destination: .\pyfiglet\pyfiglet\fonts\
#   Paths used inside the clone (.\pyfiglet):
#     Source:      .\pyfiglet\fonts-standard\*
#     Destination: .\pyfiglet\fonts\
New-Item -ItemType Directory -Force -Path ".\pyfiglet\fonts"
Copy-Item -Path ".\pyfiglet\fonts-standard\*" -Destination ".\pyfiglet\fonts\" -Recurse

# 4. Run the original CLI while still inside the pinned clone
& $pythonExe -B -m pyfiglet -f slant hello

# 5. Return to the workspace root
Set-Location ..
```

Repository: `pwaller/pyfiglet`, commit `255827efbc09d662875dad50fa0edc1e1263bf45`, commit date `2026-08-02T22:51:18+01:00`, licence MIT.

Standard fonts initially live in `pyfiglet/fonts-standard/` while font loading expects `pyfiglet/fonts/`. Performed the standard-font copy operation above, including `__init__.py`. Did not need packaging or contributed fonts. The original CLI rendered slanted "hello" and exited 0. (verified)

This reference setup is required for author-side operations (regenerating data with `tests/gen.py`, running `verify_reference.py`, or executing spot checks with `verify_data.py`). It is not required for a packaged Harbor oracle evaluation, which runs entirely inside the self-contained container.

`figlet-go/NOTES.md` records detailed source analysis: six horizontal smushing rules, their precedence, Big-X discrepancy, CLI flags, wrapping, direction, justification, and `-s`/`-n` behavior. It was copied from the original pinned upstream checkout (`255827efbc09d662875dad50fa0edc1e1263bf45`); its source paths and line references refer to that checkout. Initial "not executed" statements describe the source-study phase; later runtime checks are recorded in its runtime addendum and in [VALIDATION.md](figlet-go/VALIDATION.md). (NOTES.md itself is not modified and retains its verified original bytes.) The task copy and reference clone were hash-verified identical.

Smushing checks: underscore mode 130 (`'_' + '/' → '/'`) and Big-X mode 144 (`'/' + '\\' → '|'`) both passed. The Big-X source comment differs from the implementation; the implementation is authoritative. This discrepancy did not cause the Go port to fail. (verified)

## 2. Task initialization and Docker environment

```powershell
# From the workspace root
harbor task init figlet-go --org alokinethara --verifier-mode shared --no-pytest
```

Explicit `--verifier-mode shared` avoided relying on the template default. `--no-pytest` omitted the generated pytest template; this task uses a Go grader rather than reusing Task 1's Reward Kit verifier.

The Dockerfile uses two stages:

- **source:** `alpine:3.22.2` with Git and CA certificates; clones the exact commit, asserts HEAD, removes `.git`/`fonts-contrib`/`test-fonts`, prepares standard fonts.
- **final:** `golang:1.23-alpine`; installs bash, creates player UID/GID 1000, sets `CGO_ENABLED=0` and `GOTOOLCHAIN=local`.

Both images are digest-pinned. Build succeeded before and after pinning. No Docker failure was encountered. The Debian-Go image's Python presence and Alpine's missing bash were known traps that were avoided, not failures experienced. (verified)

Measured container runtime: user `player`, cwd `/workspace/repo`, 72 font-directory entries, `command -v python3` returned nothing (absence from PATH; not an exhaustive interpreter search), `/bin/bash` present, `/workspace/repo` writable. The 72 directory entries include non-`.flf` files (`__init__.py`, `.flc` files, etc.) and differ from the 67 `.flf` fonts embedded by the oracle. (verified)

## 3. Reference capture and data

The original CLI's `main()` detaches stdout. Used subprocess raw-byte capture to preserve CLI transformations and the additional newline.

`verify_reference.py` passed 32 captures: eight fonts × (none, `-s`, `-n`, both `-s`/`-n`). Both flags together matched `-s` alone for every tested font. (verified)

Generator (`tests/gen.py`): seed 7, 2000 visible + 2000 hidden JSONL records. Each record is `{"args":[...],"out":"..."}`. Expected outputs come only from the pinned original CLI. Eight fonts: standard, slant, small, big, mini, banner, shadow, script. Widths: 10, 20, 40, 60, 80, 120. All justification/direction choices and independently sampled boolean flags. Texts include punctuation, multiple spaces, wrapping phrases, Latin-1, and missing glyphs. `--` separates options from text. Five-second reference timeout with finite attempt/timeout limits. A glyph-width prefilter rejects candidates where any glyph width ≥ selected width; this is a precaution, not proof of a universally established reference infinite-loop bug.

### Equivalence-overlap fix

An audit found that while the visible and hidden argument lists had zero exact overlap, 13 hidden records were answerable by normalizing options (resolving defaults, direction, justification, `-s` precedence). Predicted cheat reward with this weakness: approximately 0.338 (offline prediction, not a Harbor measurement).

Fix: during hidden generation, reject equivalence keys found in visible or previously accepted hidden cases. Normalize defaults, resolved direction/justification, and `-s` precedence; preserve exact text and separate transformation flags. Keep assignment scoring weights and gate. No blacklist of particular observed inputs.

Current generation: 4039 candidates, 4000 accepted. Rejections: duplicate 7, unsafe_width 10, equivalent_visible 13, equivalent_hidden 9; timeouts/nonzero/stderr all zero. Independent audit confirmed 1993 unique visible equivalence keys, 2000 unique hidden keys, zero shared keys, zero answerable hidden records. Revised files reproduced byte-for-byte across two runs (`fc.exe /b`). Revised predicted cheat reward: 1/6 (offline prediction). (verified)

Current dataset hashes:

| File | SHA-256 |
| --- | --- |
| visible.jsonl | `f38c234706244f4b08e6ea0cde272d0976dd47932130c3c72200f1c0d937790c` |
| hidden.jsonl (current) | `43f27eab79441fe3539220de9c5f627ab56dd2103effa7315e87b14988b0d30e` |
| hidden.jsonl (original, historical) | `23147c81a7bf0987454e2dcd96bac02f9177741e02651ee03b25b753d1700bf5` |

## 4. Grader

Go standard-library grader (`tests/grade.go`), no Reward Kit or Python dependency at grading time. The grader runs as root inside the shared-verifier container and launches all solution commands as UID/GID 1000 with cleared supplementary groups.

Reward formula: `(build + no_python + 4 × behaviour) / 6`

- **build** (0 or 1): requires both `go build -o /tmp/figlet .` and `go build ./...` to succeed, plus a dependency metadata check (`go list -deps -json ./...`) that accepts standard packages and the solution's own in-repository main-module packages while rejecting external/replaced dependencies.
- **no_python** (0 or 1): requires build success, no `.py` files under `/workspace/repo`, AND at least one hidden-case match.
- **behaviour** (0.0–1.0): fraction of 2000 exact-output matches, subprocess cwd `/tmp`, five-second timeout per case.

Dataset errors are verifier errors, not silent empty grading. Stale `/tmp/figlet` is removed. Reward is written only after the secret branch completes. The secret branch renders an authored FIGlet banner using the solution binary only upon 2000/2000 hidden matches. The secret file is absent from the starting image. Base64 is encoding, not encryption.

## 5. Oracle development

`oracle_dev/` contains four Go sources (`main.go`, `font.go`, `builder.go`, `string.go`), `go.mod`, MIT `LICENSE`, and 67 unchanged standard `.flf` files embedded with `go:embed`.

Development approach: a NUL-byte stub first produced 0 visible matches and 2000 differences with no execution failures, validating `dev_compare.go` before the real port. The first port then matched 2000/2000 visible cases. (verified)

`build_solve.py` verifies all font/licence bytes match `pyfiglet/pyfiglet/fonts-standard/` and packages existing source into deterministic tar.gz/base64 inside `solution/solve.sh`. No interpreter or network required to deploy it. Its build-success message alone does not prove behavioral correctness.

| Property | Value |
| --- | --- |
| Solution size | 236164 bytes |
| Solution SHA-256 | `63fb1050569e8b33c3f08a87b844848a1791e674da22bd97269d706fdd429cec` |
| Cheat size | 260567 bytes |
| Cheat SHA-256 | `c9fc5e62259913708a7b750da2724d971d22b0dc4baa53f5000a5be7b6c4ff98` |

## 6. Security hardening

Initial shared verifier and submitted program both ran as player. An open-without-reading Go probe (`probe_hidden_access.go`) running as UID 1000 could open the hidden file in a Docker reproduction. A full answer-stealing exploit was not run inside Harbor.

Changes: kept shared mode but set verifier user to root. `test.sh` makes copied `/tests` root-owned and removes group/other access, protects logs, and creates separate trusted/solution Go caches. `grade.go` launches all solution builds, dependency inspection, behavior, and banner commands as UID/GID 1000 with cleared supplementary groups.

Focused test measured: `uid 1000, gid 1000, groups 1000`; root could open the dataset; child probe got permission denied. Hardened oracle, nop, and cheat all passed their expected checks. (verified)

This is a measured access boundary, not a complete sandbox security proof. Do not run `test.sh` as root against writable host bind mounts: it recursively changes `/tests` ownership/permissions. Use Harbor's copied tests or disposable container copies.

## 7. Mutation evidence

Visible-only, independent temporary mutants of the proven Go port. Every baseline: 2000 matches. Each mutant: zero timeouts, zero nonzero exits.

| Mutation | Differences | % of 2000 |
| --- | --- | --- |
| Disabled equality | 851 | 42.55 |
| Disabled hierarchy | 701 | 35.05 |
| Disabled underscore | 431 | 21.55 |
| Disabled universal smushing | 408 | 20.40 |
| Disabled opposite pairs | 23 | 1.15 |

Counts overlap across rules; do not sum as unique coverage. Original-file integrity checks passed.

`mutation_check.py` currently implements only the universal-smushing mutation. Earlier rules were tested by changing the script's single guarded target between runs. Historical results are linked in [figlet-go/VALIDATION.md](figlet-go/VALIDATION.md).

## 8. Cheat validation and restoration

`build_cheat.py` packages a standard-library Go lookup of exact visible argument lists; no rendering algorithm. Passes 2000/2000 visible control cases. Misses produce no stdout. Deletes Python files to exercise the strengthened no_python gate. Refuses to overwrite an existing output.

`run_cheat.py` checks hashes, refuses an existing job/backup, creates an exclusive verified backup, performs a guarded swap, checks Harbor's return code, restores the real oracle, and verifies the restored hash. Power loss or forced termination is not guaranteed recoverable.

Current state: The included `figlet-equivalence-cheat-01` job directory exists in `figlet-go/jobs/`. The historical `solution_backup/` directory is intentionally excluded from this submission. Because `run_cheat.py` uses fixed job name `figlet-equivalence-cheat-01`, rerunning it unchanged will refuse because the job name already exists. A new experiment requires choosing fresh job and backup names; the script creates and verifies its backup.

## 9. Current Harbor results

These use the root/player boundary and revised hidden dataset:

| Run | Job / Trial | build | no_python | behaviour | reward | Secret |
| --- | --- | --- | --- | --- | --- | --- |
| Oracle | `figlet-equivalence-oracle-01` / `figlet-go__LASstJC` | 1 | 1 | 1 | 1 | Unlocked |
| Nop | `figlet-equivalence-nop-01` / `figlet-go__gzCSSoA` | 0 | 0 | 0 | 0 | — |
| Cheat | `figlet-equivalence-cheat-01` / `figlet-go__PkD9GZQ` | 1 | 0 | 0 | 1/6 | — |

All three completed with zero exceptions and successful command exit. All three terminal screenshots were saved and visually reviewed. Real oracle restored and independently hash-verified after cheat run.

Final `harbor lint . --select ALL`: only the shared-verifier warning, exit 1. Warning deliberately accepted because assignment requires shared mode. Root verifier must not be confused with root agent.

## 10. Lessons learned

- **Windows LF/BOM handling:** Python `open()` defaults and Git autocrlf can silently change line endings. Explicit `encoding="utf-8"`, `newline="\n"`, and `autocrlf=input` are necessary.
- **Exact capture:** pyfiglet detaches stdout; subprocess raw-byte capture preserves the CLI's additional newline.
- **Fresh job names:** Harbor refuses reused job names. Scripts that automate Harbor runs must check for existing jobs.
- **Protected backups:** `run_cheat.py` uses exclusive file creation and hash verification, but cannot recover from SIGKILL during the swap window.
- **Independent checks:** Verification scripts separate reference execution from frozen verification. `verify_smushing.py` directly imports the pinned reference module and calls `smushChars` in-process. In contrast, CLI reference-capture tools (`verify_reference.py`, `tests/gen.py`, and the spot checks in `verify_data.py`) launch child processes from the reference clone to capture exact stdout and avoid testing any system-installed library, verifying source identity (Git HEAD) and import paths where implemented. Meanwhile, `audit_equivalent_args.py` verifies frozen hashes and computes normalized equivalence overlap without executing the reference, and `mutation_check.py` evaluates code variations against frozen visible data inside a Docker container.

## Not proven / known limitations

- **Difficulty.** No model other than the oracle's author has attempted this. The first-draft port scored 100%, which suggests the task may be easy. No fresh-agent run was performed.
- **Real-agent conditions.** Whether the 3600 s timeout and public network suit a real model run was never tested.
- **Hidden mutation sensitivity.** Mutation experiments used only visible data.
- **Font feature coverage.** Big-X smushing and hardblank smushing are disabled by all eight dataset fonts. All eight fonts have automatic left-to-right direction.
- **Untested features.** Kerning, wrapping, and transformation interactions have not been mutation-tested.
- **Security scope.** Establishes the tested isolation boundary; does not claim universal exploit resistance.
- **`mutation_check.py` limitation.** Currently universal-only; no CLI selector.
- **`verify_grader_build_only.sh` limitation.** Historical fixture assumes player-run verifier; not compatible with current root-required `test.sh`.
- **Assignment's second secret.** Successfully rendered through the Go oracle inside figlet-room; exit code 0. Screenshot: [gotouchgrass.png](figlet-go/screenshots/gotouchgrass.png).

## Submission status

Submission repository: https://github.com/japjeet-ethara/Harbor_Training_2

Submission folder: `AlokKumarJaiswal/`, containing the `figlet-go/` task.

The submission copy was assembled and all 142 copied files matched their originals by SHA-256 before `.gitattributes` was added. The `.gitattributes` file (`* -text`) preserves exact bytes without Git line-ending conversion. The three final evaluation jobs and all four screenshots are included. Files have been staged, but no submission commit or push has occurred.