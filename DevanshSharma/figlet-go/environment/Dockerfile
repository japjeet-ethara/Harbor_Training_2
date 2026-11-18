# ── Stage 1: fetch and prepare pyfiglet source ─────────────────────────────
FROM alpine:3.20.3 AS source

RUN apk add --no-cache git

RUN git clone https://github.com/pwaller/pyfiglet /src && \
    cd /src && \
    git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 && \
    # Remove heavy / irrelevant directories
    rm -rf .git pyfiglet/fonts-contrib test-fonts && \
    # Font setup (mirrors the Makefile "minimal" target)
    mkdir -p pyfiglet/fonts && \
    cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ── Stage 2: the agent's working room ───────────────────────────────────────
FROM golang:1.23-alpine3.20

# Install bash (Harbor runs commands with bash; Alpine has none by default)
RUN apk add --no-cache bash

# Create a non-root user "player" (uid 1000) with a home directory
RUN adduser -D -u 1000 -h /home/player player

# Copy the prepared source into the workspace, owned by player
COPY --from=source --chown=player:player /src /workspace/repo

# Go build settings
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

# Switch to the non-root user
USER player

WORKDIR /workspace/repo
