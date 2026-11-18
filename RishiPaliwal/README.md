# Rishi Paliwal

## figlet-go

This folder contains a Harbor task that asks an agent to recreate
[pyfiglet](https://github.com/pwaller/pyfiglet) as a Go command-line program.
The implementation is expected to use only the Go standard library and match
pyfiglet's output for the supported font, width, justification, direction,
reverse, flip, and newline options.

### Results

| Run | Result | Reward |
| --- | --- | ---: |
| Oracle solution | Full behavior, build, and no-Python scores | 1.000 |
| No-op | No implementation | 0.000 |
| Cheat lookup | Visible-case lookup; does not generalize to hidden cases | 0.167 |

These are the results shown in the included Harbor run screenshots.

### Screenshots

#### Oracle

![Oracle run result](figlet-go/oracle%20result.png)

#### No-op

![No-op run result](figlet-go/nop%20results.png)

#### Cheat lookup

![Cheat lookup run result](figlet-go/cheat%20result.png)

### Contents

- `figlet-go/instruction.md` - task requirements.
- `figlet-go/environment/` - container setup.
- `figlet-go/solution/` - reference Go implementation.
- `figlet-go/cheat/` - visible-case lookup attempt.
- `figlet-go/NOTES.md` - notes on pyfiglet options and rendering rules.
- `figlet-go/tests/` - grader, test generator, and test fixtures.
- `figlet-go/*.png` - Harbor run result screenshots.
- `jobs/` - Harbor run logs and result files.

Generated test-case JSONL files are not included; `tests/gen.py` can create
them when the original pyfiglet source is available beside this task folder.
