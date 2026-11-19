# default/figlet-go

## Reflection

- **Smushing / Layout Rule:** Getting right-to-left text (`-D right-to-left`) and right alignment (`-j right`) to line up properly when multiple character-overlapping (smushing) rules applied at once caused the most mismatches against `pyfiglet`.
- **Docker Trap:** Standard Go Docker images already have Python installed, which broke our "no Python" rule—switching to a minimal Alpine image solved that, though we had to manually install `bash` and `git` so Harbor could run our test scripts.