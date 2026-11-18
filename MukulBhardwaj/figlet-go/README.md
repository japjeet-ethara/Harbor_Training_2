# 

# \# figlet-go: Go port of pyfiglet (Harbor task)

# 

# A Harbor task where an agent must rewrite pyfiglet (Python FIGlet) as a Go program that matches the original output byte for byte, with no Python left in the repository.

# 

# \- Source: https://github.com/pwaller/pyfiglet

# \- Pinned commit: 255827efbc09d662875dad50fa0edc1e1263bf45

# \- Licence: MIT

# 

# \## How it works

# \- \*\*Environment:\*\* two-stage Dockerfile. Stage 1 clones pyfiglet at the pinned commit, removes `.git`, `fonts-contrib` and `test-fonts`, and copies `fonts-standard` into `fonts`. Stage 2 is `golang:1.23-alpine` with bash, a non-root user `player` (uid 1000), and no Python.

# \- \*\*Tests:\*\* `tests/gen.py` runs the original pyfiglet to freeze 2000 visible and 2000 hidden cases. `tests/grade.go` scores `(build + no\_python + 4 x behaviour) / 6` on the hidden cases only.

# \- \*\*Secret:\*\* a perfect score unlocks a hidden message, drawn in slant letters by the solver's own program.

# \- \*\*Oracle:\*\* `solution/solve.sh` writes the Go port and removes all `.py` files.

# \- \*\*Cheat check:\*\* `cheat/solve.sh` only looks up the visible answers, so it cannot pass the hidden cases.

# 

# \## Results

# | Run | Reward |

# |-----|--------|

# | oracle | 1.000 (build 1, no\_python 1, behaviour 1) |

# | nop | 0.000 |

# | cheat | 0.167 (build point only, no secret) |

# 

# Screenshots are in the `screenshots` folder.

# 

# \## Two lines

# \- \*\*Smushing rule that broke the agent most:\*\* Right-to-left and `-j right` both add spaces on the left, and the agent kept mixing up where those spaces go.

# \- \*\*Docker trap I hit:\*\* Alpine has no bash, which Harbor needs, so bash had to be installed in stage 2. I also hit a related setup issue: `test.sh` ran the grader from `/tests`, so `go build` failed with "go.mod not found" until it was run from `/workspace/repo`.

# '@ | Set-Content MukulBhardwaj\\figlet-go\\README.md

