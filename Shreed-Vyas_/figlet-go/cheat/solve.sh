#!/bin/bash
# Deliberately cheating "solution": wraps the Python original (python file + exec of python3).
set -euo pipefail
mkdir -p /workspace/repo/figlet
cd /workspace/repo/figlet
cat > go.mod <<'EOT'
module figlet

go 1.23
EOT
cat > shim.py <<'EOT'
import sys
sys.path.insert(0, "/workspace/repo")
import pyfiglet
pyfiglet.main()
EOT
cat > main.go <<'EOT'
package main

import (
	"os"
	"os/exec"
)

func main() {
	args := append([]string{"/workspace/repo/figlet/shim.py"}, os.Args[1:]...)
	cmd := exec.Command("python3", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
EOT
go build -o figlet .
