# syntax=docker/dockerfile:1

# ---- Stage 1: fetch the pinned pyfiglet snapshot -------------------------
FROM alpine/git:v2.49.1@sha256:c0280cf9572316299b08544065d3bf35db65043d5e3963982ec50647d2746e26 AS source

RUN git clone https://github.com/pwaller/pyfiglet /src \
    && git -C /src checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf /src/.git /src/pyfiglet/fonts-contrib /src/test-fonts \
    && mkdir /src/pyfiglet/fonts \
    && cp /src/pyfiglet/fonts-standard/* /src/pyfiglet/fonts/

# ---- Stage 2: the agent's room (Go toolchain, no Python) -----------------
FROM golang:1.23.12-alpine3.22@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

RUN apk add --no-cache bash \
    && adduser -D -u 1000 -s /bin/bash player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
