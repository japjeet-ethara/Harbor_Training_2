# Stage 1 — clone and prepare pyfiglet source (no runtime footprint)
FROM alpine/git:latest@sha256:2306e2b05ee0ef99f5d25b0779f645b24ae9f91ba187107503592fcdfe78da3a AS source

WORKDIR /src
RUN git clone https://github.com/pwaller/pyfiglet . \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git \
    && rm -rf pyfiglet/fonts-contrib test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# Stage 2 — agent room (Go + bash, no Python)
FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

RUN apk add --no-cache bash

RUN adduser -D -u 1000 player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
