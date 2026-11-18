# ---------------------------------------------------------------------------
# Stage 1 ("source"): fetch pyfiglet at a pinned commit and finish its setup.
# git and everything else in this stage never reaches the agent's room.
# ---------------------------------------------------------------------------
FROM alpine/git:v2.54.0@sha256:6a1fdb962814ca11508ca8d1bab94375b5b7ac4e4a9a40981c29fc32c02f095d AS source

ARG PYFIGLET_REPO=https://github.com/pwaller/pyfiglet
ARG PYFIGLET_SHA=255827efbc09d662875dad50fa0edc1e1263bf45

WORKDIR /src
RUN git clone "${PYFIGLET_REPO}" . \
 && git checkout "${PYFIGLET_SHA}" \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 # D3: the Makefile's font step - the code loads fonts from pyfiglet/fonts/,
 # but the repository ships them in pyfiglet/fonts-standard/.
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------------------------------------------------------------------------
# Stage 2: the agent's room. Go only - no Python, no git. Non-root user.
# ---------------------------------------------------------------------------
FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

# D6: Harbor runs its commands with bash, which Alpine does not ship.
RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
