# Stage 1: fetch source, pin commit, prune and prepare fonts
FROM alpine:3.21 AS source

RUN apk add --no-cache git ca-certificates make python3 py3-build

WORKDIR /src

RUN git clone https://github.com/pwaller/pyfiglet.git . && \
    git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 && \
    rm -rf .git pyfiglet/fonts-contrib test-fonts && \
    make minimal

# Stage 2: agent room without Python installed
FROM golang:1.23-alpine

RUN apk add --no-cache bash

RUN adduser -D -u 1000 player && \
    mkdir -p /workspace/repo && \
    chown -R player:player /workspace

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
