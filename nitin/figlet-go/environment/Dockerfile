# ---------------------------------------------------------------------------
# Stage 1 ("source"): fetch pyfiglet at a pinned commit and do the project's own
# setup step. git and everything else in this stage never reaches the agent's room.
# ---------------------------------------------------------------------------
FROM alpine:3.20 AS source

RUN apk add --no-cache git \
 && git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 # The Makefile's "minimal" target: the code loads fonts from pyfiglet/fonts,
 # but the repo only ships pyfiglet/fonts-standard.
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------------------------------------------------------------------------
# Stage 2: the agent's room. Go on Alpine = no Python anywhere (D2).
# Alpine has no bash, and Harbor runs its commands with bash, so we add it (D6).
# To pin by digest (D5, optional): docker inspect --format '{{index .RepoDigests 0}}' golang:1.23-alpine
# and write FROM golang:1.23-alpine@sha256:... instead.
# ---------------------------------------------------------------------------
FROM golang:1.23-alpine

RUN apk add --no-cache bash \
 && adduser -D -u 1000 player \
 && mkdir -p /workspace/repo \
 && chown -R player:player /workspace

COPY --from=source --chown=player:player /src/ /workspace/repo/

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
