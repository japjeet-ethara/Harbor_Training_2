# Figlet-Go Validation & Evidence Record

> **Note:** All measurements and evaluation records herein are verified from user-provided terminal evidence.

## Hardened Task & Isolation Architecture
- **Verifier Execution:** Shared verifier runs as root; agent execution remains as `player`.
- **Grader Protection:** Trusted grader protects `/tests` and verifier logs.
- **Privilege Boundary:** Solution builds, dependency checks, behavior verification, and banner commands execute using UID/GID 1000 with supplementary groups cleared.
- **Isolation Verification:** Focused isolation test measured UID 1000, GID 1000, groups 1000, and verified `Permission denied` when attempting to open the protected hidden dataset.
- **Scope:** Establishes the tested security boundary under verified conditions; does not claim universal exploit resistance.

## Historical Harbor Results (Original Hidden Dataset)
- **Oracle (`figlet-isolation-oracle-01` / `figlet-go__f3H3JCV`):**
  - All components: 1
  - Hidden matches: 2000/2000
  - Authored banner: unlocked
- **NOP (`figlet-isolation-nop-01` / `figlet-go__798cRJG`):**
  - All components: 0
- **Cheat (`figlet-isolation-cheat-01` / `figlet-go__w3UHb2e`):**
  - Build: 1
  - No Python: 0
  - Behaviour: 0
  - Reward: 1/6
  - Secret unlock: none
- **Oracle Restoration Integrity:**
  - Independently hash-verified: `63fb1050569e8b33c3f08a87b844848a1791e674da22bd97269d706fdd429cec`

## Current Dataset & Version Record

### Dataset Hashes
- **Visible hash (unchanged):** `f38c234706244f4b08e6ea0cde272d0976dd47932130c3c72200f1c0d937790c`
- **Original hidden hash:** `23147c81a7bf0987454e2dcd96bac02f9177741e02651ee03b25b753d1700bf5`
- **Current hidden hash:** `43f27eab79441fe3539220de9c5f627ab56dd2103effa7315e87b14988b0d30e`

### Discovery & Equivalence Filtering
- **Original normalized-lookup audit:** Found 13 answerable hidden records.
- **Original predicted reward:** Approximately 0.337667.
- **Revised predicted reward:** 1/6.
- **Offline prediction note:** BOTH normalized-lookup rewards are offline predictions, not Harbor runs.
- **Filtering methodology:** General normalization-based filtering was added to the generator, not a blacklist of the observed 13 cases.
- **Evaluation criteria:** Scoring weights and gate were preserved.

### Verified Current-Data Evidence
- **Dataset volume & determinism:** 2000 records per set; byte-identical across two generator runs.
- **Visible dataset stability:** Visible data unchanged.
- **Candidate exclusions:** Generator excluded 13 visible-equivalent and 9 hidden-equivalent candidates.
- **Independent audit:** 1993 visible equivalence keys, 2000 hidden keys, zero shared keys, and zero answerable hidden records.
- **Format & spot checks:** Independent format/uniqueness validation and 3 visible reference spot checks passed.

## Current Harbor Results (Current Dataset)
- **Oracle (`figlet-equivalence-oracle-01` / `figlet-go__LASstJC`):**
  - Build: 1
  - No Python: 1
  - Behaviour: 1
  - Reward: 1
  - Matches: 2000/2000
  - Authored banner: unlocked
- **NOP (`figlet-equivalence-nop-01` / `figlet-go__gzCSSoA`):**
  - All components zero (Build: 0, No Python: 0, Behaviour: 0, Reward: 0)
- **Cheat (`figlet-equivalence-cheat-01` / `figlet-go__PkD9GZQ`):**
  - Build: 1
  - No Python: 0
  - Behaviour: 0
  - Reward: 1/6
  - Secret unlock: none
  - Oracle restored and independently hash-verified

## Visible-Only Mutation Results
- **Baseline Concordance:** Every baseline matched 2000/2000 (dataset unchanged).
- **Execution Stability:** Each mutant had zero timeouts and zero nonzero child exits.
- **Mutation Sensitivity:**
  - Disabled equality: 851 differences (42.55%)
  - Disabled hierarchy: 701 differences (35.05%)
  - Disabled underscore: 431 differences (21.55%)
  - Disabled universal smushing: 408 differences (20.40%)
  - Disabled opposite pairs: 23 differences (1.15%)
- **Integrity Audits:** Evaluated via temporary copies only; post-test integrity audits completed without errors.
- **Coverage Note:** Counts overlap across rules and must not be summed as unique coverage.

## Limitations & Pending Items
- **Hidden Set Sensitivity:** Mutation sensitivity on the hidden dataset has not been measured.
- **Font Feature Coverage:** Big-X and hardblank smushing are disabled by all eight dataset fonts.
- **Directionality:** All eight dataset fonts have automatic left-to-right direction.
- **Untested Features:** Kerning, wrapping, and transformations have not been mutation-tested.
- **Generalization:** The first port passed the frozen visible and hidden sets; this does not establish difficulty for independent agents or universal correctness.
- **Security Scope:** Establishes tested isolation and privilege boundaries under verified conditions; does not claim universal exploit resistance or universal security.
- **Cheat / Normalized Measurement Scope:** Only the specified exact-argument lookup cheat has measured Harbor runs; there is no measured normalized-cheat Harbor run (both normalized-lookup rewards are offline predictions only).
- **Linter Warning:** Shared-verifier lint warning is deliberately accepted.
- **Screenshots:** Oracle, nop, and cheat reward screenshots are complete.
- **Assignment second secret:** Successfully rendered through the oracle Go executable inside figlet-room; exit code 0.
- **Pending Deliverables:** GitHub submission / final hand-in remains pending.
