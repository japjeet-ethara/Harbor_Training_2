# akshat/figlet-go

A Harbor task: port pyfiglet to Go, byte-for-byte identical output.

## Results

| Agent  | Reward |
|--------|--------|
| oracle | 1.0    |
| nop    | 0.0    |
| cheat  | 0.167  |

## Hardest smushing rule

The hierarchy rule (SM_HIERARCHY) caused the most failures. The agent initially got the class ordering of `/\` wrong relative to `[]`, `{}`, and `()`, which broke cases involving those characters in narrow-width right-to-left mode.

## Docker trap hit

Alpine has no `bash` by default. Harbor runs verifier commands with `bash`, so the first build attempt failed immediately with `exec: "bash": executable file not found` before any test code ran. The fix is one line in the Dockerfile: `RUN apk add --no-cache bash`.
