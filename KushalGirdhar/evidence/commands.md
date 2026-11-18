# Recorded validation commands

Commands ran in WSL Ubuntu with Docker Desktop. The pinned upstream clone was a sibling of the task, with the Makefile-equivalent font copy applied. The task directory was `work/kushal-figlet-go` relative to the session workspace. Tool versions and upstream smoke commands are in `reference-runtime.log`.

From the task folder:

```bash
docker build -t kushal-figlet-room environment/
bash validation/docker-check.sh
uvx --from harbor==0.25.0 harbor lint .
bash validation/reproduce.sh ../final-data-reruns
bash validation/local-oracle.sh ../final-local-oracle
bash validation/harbor-baseline.sh oracle evidence/harbor
bash validation/harbor-baseline.sh nop evidence/harbor
bash validation/harbor-baseline.sh cheat evidence/harbor
```

The final three commands executed inside separate real xterm sessions, each on a dedicated Xvfb display. Each capture was taken after the session finished. X11 `xwd -silent -name "Harbor terminal <mode>"` captured the actual window. Netpbm `xwdtopnm` and `pnmtopng` converted that capture to PNG. The terminal transcripts preserve the expanded Harbor command lines and the raw jobs preserve their exact scores and verifier output. The page unlock's executed Docker command is in `page-unlock-final.log`.

The task keeps public Docker networking and the shared verifier. Harbor's host LiteLLM dependency reported a failed optional cost-map fetch and fell back to its bundled copy. The three baseline jobs ran without trial exceptions.
