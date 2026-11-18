# Hand-in checklist (Step 8)

## Still to do on your own machine (I could not do these in my sandbox: no Docker, no Harbor runtime)
1. `docker build` + the six-check `docker run` line (README.md). Expect: `player`, `/workspace/repo`, 72 fonts, `NO_PYTHON`, a path to bash, `CAN_WRITE`.
2. `harbor run -p ./figlet-go -a oracle`, `-a nop`, and the cheat run. Take **real terminal screenshots** of the three rewards.
3. Optional: pin both `FROM` lines by digest (comment in the Dockerfile shows how).
4. Push the `figlet-go` folder to GitHub, in your own folder, with the three screenshots.

## Results I measured (local simulation, `dev/local_sim.sh`, NOT Harbor/Docker)
| Run | build | no_python | behaviour | reward | secret |
|---|---|---|---|---|---|
| oracle | 1 | 1 | 2000/2000 | **1.0** | yes ("nice port, human" drawn in slant) |
| nop | 0 | 0 | 0 | **0.0** | no |
| cheat | 1 | 0 | 0/2000 | **0.1667** | no ("No secret for you. Yet.") |

Extra checks: the Go port also matched the original on 2,989 fresh random cases over all 67 standard fonts (0 differences), and `harbor lint` shows only the expected `shared-verifier` warning.

## Page secret (Step 8)
`/tmp/figlet -f slant "$(echo Z28gdG91Y2ggZ3Jhc3M= | base64 -d)"` with the oracle's Go code draws clean letters (the text is `go touch grass`).

## Two lines for the hand-in (edit after your own run)
- Smushing rule that broke the agent most: none broke the oracle in my run, because I ported `FigletBuilder` line by line; the quirks to watch are the empty-buffer first character losing its leading blanks and right-to-left swapping the left/right roles (NOTES.md, rare cases 1 and 2).
- Docker trap hit: I avoided all three by following the hints (Alpine instead of bookworm, `apk add bash`, stage 1 fetches pyfiglet from GitHub); I could not build the image in my sandbox, so confirm D1-D6 with the `docker run` line.
