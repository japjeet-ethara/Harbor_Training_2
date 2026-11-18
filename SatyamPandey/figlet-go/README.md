# figlet-go

Screenshots: [oracle](screenshots/1-oracle.png), [nop](screenshots/2-nop.png), [cheat](screenshots/3-cheat.png)

- **Smushing rule:** equal-character smushing carries the exam most: switched off, 687 of 2000 visible cases fail, with hierarchy smushing next at 605.
- **Docker trap:** Alpine has no `bash`, so Harbor stops with `exec: "bash": executable file not found in $PATH` until the Dockerfile adds it. The bookworm Go image was no alternative, because it ships Python 3.
