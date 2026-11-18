# Verification records

The final job directories are `jobs/oracle-final`, `jobs/nop-final`, and `jobs/cheat-final`. Each holds the original Harbor job, trial records, verifier stdout and reward JSON. `terminal/*-final.txt` are raw terminal transcripts. The PNGs in `screenshots/` were captured from the xterm windows that ran the commands, through X11 `xwd` on Ubuntu WSL. The PNG conversion preserved the captured pixels.

From the directory containing the task and its sibling pyfiglet checkout, the final Harbor commands were:

```bash
uvx --from harbor==0.25.0 harbor run -p komal-figlet-go -a oracle -o komal-figlet-go/evidence/jobs --job-name oracle-final
uvx --from harbor==0.25.0 harbor run -p komal-figlet-go -a nop -o komal-figlet-go/evidence/jobs --job-name nop-final
bash komal-figlet-go/validation/prepare-cheat.sh komal-cheat-final-check
uvx --from harbor==0.25.0 harbor run -p komal-cheat-final-check -a oracle -o komal-figlet-go/evidence/jobs --job-name cheat-final
```

The Docker build used `docker build --progress=plain -t komal-figlet-room work/komal-figlet-go/environment` from the Windows workspace. See [docker-build.txt](docker-build.txt) and [docker-checks.txt](docker-checks.txt). The room also ran `validation/container-details.sh komal-figlet-room tests` and `validation/container-details.sh komal-figlet-room page-unlock`. The pinned generator commands and hashes are in [reproducibility.txt](reproducibility.txt).

Before the diagnostic rerun, all three jobs were started together on a cold setup. The raw records are grouped under `initial-attempt/`. The earlier oracle scored 0.9983333333333334. It was followed by standalone differential checks and a sequential Harbor rerun.
