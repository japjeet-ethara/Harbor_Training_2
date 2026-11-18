# Acceptance record

Task author metadata is Komal. The test source is pyfiglet at commit `255827efbc09d662875dad50fa0edc1e1263bf45`. The Docker and Harbor checks used Docker Desktop's Linux engine and Harbor 0.25.0 in Ubuntu WSL. [Runtime versions](evidence/runtime-versions.txt)

| Requirement | Implementation and observed evidence |
| --- | --- |
| Upstream attribution, setup, rules and CLI | [NOTES.md](NOTES.md) gives the SHA, date, MIT attribution, setup, verified rules, widths and option precedence. [Upstream check](evidence/upstream-checks.txt) records the paired rule calls and the original CLI's slant render. |
| D1 and D5, two stages and immutable image references | [Dockerfile](environment/Dockerfile) pins both stages by tag and digest. The source stage fetches the pinned commit with only `environment/` as context. [Docker build log](evidence/docker-build.txt) records the actual build. |
| D2, D3, D4 and D6, prepared fonts and runnable non-root environment | The [built image check](evidence/docker-checks.txt) reports `player`, UID/GID 1000, directory ownership 1000:1000, `/workspace/repo`, 72 prepared font-directory entries, `NO_PYTHON`, `/bin/bash`, and `CAN_WRITE`. It also checked removal of the git metadata, contributed fonts, and test fonts. [Image config](evidence/docker-image-config.json) confirms the user, directory and build environment. |
| Harbor settings | [task.toml](task.toml) sets the requested timeouts, public network, `player`, and the default shared verifier. [Lint](evidence/harbor-lint.json) produced one finding, the expected `shared-verifier`. |
| Candidate contract | [instruction.md](instruction.md) specifies the normal build entry point, standard-library CLI, eight options, byte-exact output, font retention and lookup from any directory, `go build ./...`, and Python removal. |
| Reproducible standard-font reference corpus | [gen.py](tests/gen.py) invokes the pinned CLI with seed 7. [Reproducibility evidence](evidence/reproducibility.txt) records two byte-identical regenerations and hashes. [Independent audit](evidence/differential.txt) checks 4,000 distinct argument vectors, zero whitespace-only answers, and three separate original CLI invocations. |
| Balanced adversarial coverage | [Coverage](tests/data/coverage.json) records 2,000 cases per split, each flag 500 times per split and 223 visible plus 222 hidden narrow-width RTL cases. Geometry proposals use the actual font edges. The visible split screened 103 at-or-over-width proposals and the hidden split screened 108. |
| Go grader, exact formula, secret | [test.sh](tests/test.sh) runs `go run grade.go` in `/tests`. [grade.go](tests/grade.go) streams exact stdout comparison, limits candidate execution to five seconds, and derives all scores from case outcomes. The Python point requires a build, a clean workspace, and a hidden hit. The verifier output and reward JSON for all jobs are under [evidence/jobs](evidence/jobs). |
| Oracle, real Go port | [solve.sh](solution/solve.sh) applies the column-oriented implementation, embeds the retained standard fonts, deletes Python files and builds in the agent repository. Direct comparisons matched 2,000 visible, 2,000 hidden and 657 independently generated cases across 67 fonts. [Differential run](evidence/differential.txt) |
| Parser, collision, CLI, font lookup and grading tests | [Go test log](evidence/go-tests.txt) records 7 named Go tests and 18 named subtests across the oracle and grader. It covers all six smushing rules, kerning, Unicode cells, transform order, external working directory, scoring, byte checks and timeout handling. |
| Baselines and terminal captures | See the results and screenshots below. Replay was installed in a separate task copy with [prepare-cheat.sh](validation/prepare-cheat.sh). |
| Assignment page unlock | The [command wrapper](validation/container-details.sh) ran the page's command with the compiled Go solution in the shipped room. [Raw output](evidence/page-unlock.txt) |
| Hand-in instructions and AI disclosure | [README.md](README.md) includes WSL setup, regeneration, validation and upload. See [AI_ASSISTANCE.md](AI_ASSISTANCE.md) and `SHA256SUMS`. GitHub upload is pending a destination and authorization. |

## Harbor baselines

| Baseline | Reward | Build | No Python | Behaviour | Terminal screenshot |
| --- | ---: | ---: | ---: | ---: | --- |
| Oracle | 1.0 | 1 | 1 | 1.0 | [Oracle](evidence/screenshots/oracle.png) |
| Nop | 0.0 | 0 | 0 | 0.0 | [Nop](evidence/screenshots/nop.png) |
| Visible argument replay | 0.16666666666666666 | 1 | 0 | 0.0 | [Replay](evidence/screenshots/cheat.png) |

The final oracle verifier counted 2,000 matches and printed the banner. Nop and replay printed `No secret for you. Yet.` The earlier oracle gave 1,995 matches while three jobs were running. A direct run reproduced zero hidden output differences. The grader gained failure diagnostics, a larger stdout drain allowance and a smaller worker pool before the sequential rerun. The first run had no per-case logs, so its five misses have no confirmed cause. The raw initial oracle evidence is preserved under [initial-attempt](evidence/initial-attempt).

## Two hand-in lines

- Smushing rule with the most observed trouble — none. The direct differential runs reproduced zero rendering differences.
- Docker trap encountered — the first development bind mount from Windows resolved to the wrong directory. Moving that temporary compilation to WSL's Linux filesystem resolved it. The submitted Dockerfile passed the required runtime checks.
