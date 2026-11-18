# Hand-in notes

**Rewards:** oracle 1.0, nop 0.0, cheat 0.167 (no secret for the cheat). Screenshots are in `Screenshots/`.

**Rule that broke the most:** Right-to-left. `smushAmount` and `smushRow` swap the left and right rows, and the `SM_BIGX` comment says `/` followed by `\` becomes `X` while the code returns `|`, so the port has to follow the code and not the comment.

**Docker trap:** The verifier ran in `separate` mode and could not see Go (`go: command not found`). Setting `environment_mode = "shared"` in `task.toml` fixed it.
