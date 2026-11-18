# ============================================================
# Stage 1 — "source": fetch and prepare pyfiglet from GitHub
# ============================================================
FROM alpine:3.21.3 AS source

RUN apk add --no-cache git

# Clone pyfiglet at the pinned commit
RUN git clone https://github.com/pwaller/pyfiglet /src && \
    cd /src && \
    git checkout 255827efbc09d662875dad50fa0edc1e1263bf45

# Font setup (mirrors the Makefile "minimal" target):
# fonts/ is what pyfiglet's code loads from; fonts-standard/ is what ships on GitHub
RUN mkdir -p /src/pyfiglet/fonts && \
    cp /src/pyfiglet/fonts-standard/* /src/pyfiglet/fonts/

# Clean up things the agent should never see
RUN rm -rf /src/.git \
           /src/pyfiglet/fonts-contrib \
           /src/test-fonts

# ============================================================
# Stage 2 — the agent's room
# ============================================================
FROM golang:1.23-alpine

# Install bash (Alpine ships with sh only; Harbor needs bash)
RUN apk add --no-cache bash

# Create a non-root user "player" with uid 1000
RUN adduser -D -u 1000 player

# Copy the prepared pyfiglet source into the workspace, owned by player
COPY --from=source --chown=player:player /src /workspace/repo

# Go toolchain settings
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

# Switch to non-root user
USER player

WORKDIR /workspace/repo
