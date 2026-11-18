# default/figlet-go

# Harbor Practice Task 2 – FIGlet in Go

## Project Overview

This project recreates the functionality of **pyfiglet** in Go. FIGlet converts ordinary text into large ASCII-art letters using font files.

The goal is to build a Go-based command-line application that produces output matching the original pyfiglet implementation exactly, including spacing, alignment, character rendering, and smushing behaviour.

The project also includes a Docker environment, test datasets, an automated grader, and agent evaluations.

## Objectives

- Understand the original pyfiglet implementation.
- Build a two-stage Docker environment.
- Create a clear task instruction for an AI coding agent.
- Generate visible and hidden test cases.
- Implement an automated grading system in Go.
- Develop a Go implementation that matches pyfiglet output.
- Evaluate the solution using Oracle, Nop, and Cheat agents.

## Project Structure

```text
figlet-go/
├── cheat/
│   └── solve.sh
├── environment/
│   └── Dockerfile
├── jobs/
├── screenshot/
│   ├── oracle.png
│   ├── nop.png
│   └── cheat.png
├── solution/
│   └── solve.sh
├── tests/
│   ├── data/
│   │   ├── visible.jsonl
│   │   └── hidden.jsonl
│   ├── gen.py
│   ├── grade.go
│   ├── secret.b64
│   └── test.sh
├── NOTES.md
├── instruction.md
└── task.toml
```

*The structure above describes the intended project layout; filenames may vary if your actual project uses different names.*

## Technology Stack

- **Go:** Main implementation and automated grader.
- **Docker:** Isolated execution environment.
- **Alpine Linux:** Lightweight final container image.
- **Harbor:** Task execution and agent evaluation.
- **Python:** Used on the host to generate test cases from the original pyfiglet implementation.
- **JSONL:** Storage format for visible and hidden test datasets.

## Docker Environment

The Docker environment uses a multi-stage build:

1. **Source stage:** Retrieves pyfiglet from its pinned GitHub commit and prepares the standard fonts.
2. **Final stage:** Uses a pinned Go Alpine image, installs Bash, creates a non-root user named `player`, and copies the prepared source into `/workspace/repo`.

The final environment is designed to:
- Exclude Python.
- Run as a non-root user.
- Allow the player user to write to the repository.
- Support Harbor execution through Bash.
- Keep the required fonts available.

## Implementation Requirements

The Go application must:

- Reproduce pyfiglet output byte for byte.
- Use the Go standard library only.
- Keep the required `.flf` font files inside the repository or embed them.
- Work independently of the current working directory.
- Support the required command-line options:
  - `-f` – Font selection
  - `-w` – Output width
  - `-j` – Justification
  - `-D` – Text direction
  - `-r` – Random font
  - `-F` – Full-width mode
  - `-s` – Smushing mode
  - `-n` – Disable smushing

The implementation must not contain Python files.

## Testing and Dataset Generation

The test generator creates two datasets:

| Dataset | Number of cases | Purpose |
|---|---:|---|
| `visible.jsonl` | 2,000 | Visible test cases |
| `hidden.jsonl` | 2,000 | Hidden evaluation cases |

The generator uses a fixed random seed to make the generated datasets reproducible.

Test cases cover different fonts, widths, justification modes, text directions, rendering options, special characters, and difficult smushing combinations.

Expected output is captured from the original pyfiglet implementation.

## Automated Grading

The grader is written in Go and evaluates the implementation using three components.

| Component | Weight | Requirement |
|---|---:|---|
| Build | 1 | Go application builds successfully |
| No Python | 1 | No `.py` files remain and at least one hidden case passes after a successful build |
| Behaviour | 4 | Exact output matches across hidden test cases |

The final reward is calculated as:

`Reward = (Build + No_Python + 4 × Behaviour) / 6`

Each component is scored between 0 and 1.

## Agent Evaluation

Three agents are used to validate the grading system:

| Agent | Expected reward | Purpose |
|---|---:|---|
| Oracle | 1.0 | Demonstrates a fully correct implementation |
| Nop | 0.0 | Confirms that doing no implementation work earns no reward |
| Cheat | Below 0.2 | Confirms that memorising visible examples does not pass hidden tests |

The Cheat evaluation should also produce the grader's unsuccessful-secret message.

## Running the Task

Run the following commands from the parent directory containing `figlet-go`:

### Run Oracle

```powershell
harbor run -p ./figlet-go -a oracle
```

### Run Nop

```powershell
harbor run -p ./figlet-go -a nop
```

### Build the Docker image

```powershell
docker build -t figlet-room ./figlet-go/environment/
```

## Submission Checklist

- [ ] `NOTES.md` contains the project research and smushing-rule explanations.
- [ ] Dockerfile uses the required two-stage environment.
- [ ] Final image runs as the non-root `player` user.
- [ ] Python is absent from the final image.
- [ ] `instruction.md` clearly describes implementation requirements.
- [ ] `tests/gen.py` generates reproducible datasets.
- [ ] Visible and hidden datasets contain 2,000 cases each.
- [ ] `tests/grade.go` implements the required grading logic.
- [ ] `tests/secret.b64` is present.
- [ ] Oracle reward is 1.0.
- [ ] Nop reward is 0.0.
- [ ] Cheat reward is below 0.2 with no secret unlocked.
- [ ] Genuine Oracle, Nop, and Cheat terminal screenshots are included.
- [ ] Two honest observations about smushing and Docker challenges are documented.
- [ ] The task folder is pushed to GitHub.

## Conclusion

This project demonstrates how to reproduce an existing command-line application in Go, build a controlled Docker environment, generate reproducible tests, and evaluate AI-generated implementations using hidden test cases and weighted rewards.

The final objective is a reliable Go implementation that matches the original pyfiglet output exactly and successfully passes the automated evaluation.

## Two Honest Lines
1. Smushing rule: The biggest challenge was getting the smushing behaviour right, especially handling character collisions and spacing without changing the expected output.

2. Docker trap: The Docker trap I encountered was the difference between the host project path and the container’s /workspace/repo path, which caused path-related issues during execution.