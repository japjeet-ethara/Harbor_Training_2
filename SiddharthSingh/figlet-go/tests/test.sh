#!/bin/bash
set -euo pipefail

cd /workspace/repo

echo "Repository: $(pwd)"
echo "Running Go grader..."

go run /tests/grade.go