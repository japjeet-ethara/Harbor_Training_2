# default/figlet-go
# FIGlet-Go — Harbor Practice Task 2

## Overview
FIGlet-Go is a Go implementation of [PyFiglet](https://github.com/pwaller/pyfiglet). It converts normal text into large ASCII-art banners using FIGlet fonts.


## Evaluation Results

| Agent | Reward |
|---|---:|
| Oracle | 1.0 |
| NOP | 0.0 |
| Cheat | 0.167 |

## Screenshots
- [Oracle Result](C:\Users\PriyaKumari\Desktop\figlet-go\Screenshot\oracle.png)
- [NOP Result](C:\Users\PriyaKumari\Desktop\figlet-go\Screenshot\NOP.png)
- [Cheat Result](C:\Users\PriyaKumari\Desktop\figlet-go\Screenshot\Cheat.png)

## 2 Honest Lines
- **Rendering challenge:** The Shadow font added extra spaces between words because the wrong font files were being used. I fixed this by using only standard fonts.
- **Docker challenge:** Alpine did not have Bash installed by default. I installed Bash because Harbor needs it to run commands.

## Original Repository
- **Source:** [PyFiglet](https://github.com/pwaller/pyfiglet)
- **Pinned Commit:** `255827efbc09d662875dad50fa0edc1e1263bf45`
- **License:** MIT
