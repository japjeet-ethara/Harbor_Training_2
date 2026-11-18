# ---------------------------------------------------------------------------
# Stage 1 "source": fetch pyfiglet at a pinned commit and finish its own setup.
# git and friends live ONLY in this stage; they never reach the agent's room (D1).
# ---------------------------------------------------------------------------
FROM alpine:3.20 AS source

RUN apk add --no-cache git

WORKDIR /src
RUN git clone https://github.com/pwaller/pyfiglet . \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git \
 && rm -rf pyfiglet/fonts-contrib test-fonts \
 # D3: the Makefile target "minimal" copies fonts-standard into a new fonts folder
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------------------------------------------------------------------------
# Stage 2: the agent's room. Alpine Go image (golang:*-bookworm ships Python 3).
# Optional: pin by digest, e.g. golang:1.23-alpine@sha256:...  (see README steps)
# ---------------------------------------------------------------------------
FROM golang:1.23-alpine

# D6: Harbor runs its commands with bash, Alpine has none by default.
RUN apk add --no-cache bash

# D4: normal user "player" (uid 1000) who owns /workspace/repo.
RUN adduser -D -u 1000 -s /bin/bash player \
 && mkdir -p /workspace/repo \
 && chown player:player /workspace /workspace/repo

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

# D2: nothing here installs Python; "player" cannot apk add it either.
USER player
WORKDIR /workspace/repo
