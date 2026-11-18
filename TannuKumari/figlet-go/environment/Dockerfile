# syntax=docker/dockerfile:1
#
# Two-stage room for the "port pyfiglet to Go" task.
#
# D1 two stages          : stage "source" fetches the repo, stage 2 is the agent's room
# D2 no Python at all    : final stage is golang:*-alpine, nothing installs python3
# D3 upstream's own setup: the Makefile copies fonts-standard/* into pyfiglet/fonts/
# D4 non-root user       : "player" (uid 1000) owns /workspace/repo
# D5 pinned base images  : tag + sha256 digest
# D6 bash present        : Harbor runs its commands with bash, Alpine ships none

# ---------------------------------------------------------------- stage 1: source
FROM alpine/git:latest@sha256:2306e2b05ee0ef99f5d25b0779f645b24ae9f91ba187107503592fcdfe78da3a AS source

ARG PYFIGLET_REPO=https://github.com/pwaller/pyfiglet
ARG PYFIGLET_SHA=255827efbc09d662875dad50fa0edc1e1263bf45

WORKDIR /
RUN set -eux; \
    git clone "${PYFIGLET_REPO}" /src; \
    cd /src; \
    git checkout "${PYFIGLET_SHA}"; \
    git rev-parse HEAD > /src/PINNED_COMMIT; \
    rm -rf /src/.git; \
    rm -rf /src/pyfiglet/fonts-contrib /src/test-fonts; \
    mkdir -p /src/pyfiglet/fonts; \
    cp /src/pyfiglet/fonts-standard/* /src/pyfiglet/fonts/; \
    test "$(ls -1 /src/pyfiglet/fonts | wc -l)" -gt 50

# ------------------------------------------------------- stage 2: the agent's room
FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

# D6: Harbor execs its commands with bash; Alpine ships only /bin/sh.
# coreutils gives a sane base64/timeout. Neither package pulls in python3.
RUN apk add --no-cache bash coreutils

# `bash -l` runs /etc/profile, which resets PATH and loses the Go toolchain the
# base image put there; keep `go` reachable in login shells too.
RUN printf 'export PATH="$PATH:/usr/local/go/bin"\n' > /etc/profile.d/go-path.sh \
 && chmod 0644 /etc/profile.d/go-path.sh

# D4: a normal user, uid 1000, owning the work tree
RUN addgroup -g 1000 player \
 && adduser -D -u 1000 -G player -s /bin/bash player \
 && mkdir -p /workspace \
 && chown player:player /workspace

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOFLAGS=-mod=mod \
    HOME=/home/player \
    GOCACHE=/home/player/.cache/go-build \
    GOMODCACHE=/home/player/go/pkg/mod

# D2: fail the build (not the grading run) if anything dragged Python in
RUN set -eux; \
    if command -v python3 || command -v python; then \
      echo "FATAL: python found in final image" >&2; exit 1; \
    fi; \
    command -v bash

USER player
WORKDIR /workspace/repo
