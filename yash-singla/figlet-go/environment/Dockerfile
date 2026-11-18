# syntax=docker/dockerfile:1

# Stage 1: fetch and prepare pinned pyfiglet source (fonts only for runtime)
FROM alpine/git:2.47.2 AS source
WORKDIR /src
RUN git clone https://github.com/pwaller/pyfiglet.git . \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git pyfiglet/fonts-contrib test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# Stage 2: agent room (Go only, no Python)
FROM golang:1.23.6-alpine3.21

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

RUN apk add --no-cache bash \
    && adduser -D -u 1000 player \
    && mkdir -p /workspace/repo

COPY --from=source --chown=player:player /src /workspace/repo
RUN chown -R player:player /workspace/repo

USER player
WORKDIR /workspace/repo
