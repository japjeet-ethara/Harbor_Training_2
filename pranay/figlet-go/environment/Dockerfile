# ---------- Stage 1: fetch pyfiglet and finish its own setup (rules D1, D3) ----------
# Pinned tag. For a digest pin run: docker inspect --format '{{index .RepoDigests 0}}' alpine:3.20
FROM alpine:3.20 AS source
RUN apk add --no-cache git
RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------- Stage 2: the agent's room (rules D2, D4, D5, D6) ----------
# Alpine, NOT bookworm: the bookworm Go image contains /usr/bin/python3.
# Pinned tag. For a digest pin run: docker inspect --format '{{index .RepoDigests 0}}' golang:1.23-alpine3.20
FROM golang:1.23-alpine3.20

# Harbor runs its commands with bash; Alpine has none (D6).
RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player \
 && mkdir -p /workspace \
 && chown player:player /workspace

# Only the finished repo comes across; git and friends stay in stage 1 (D1).
COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
