# Stage 1: source
FROM alpine/git:2.47.1 AS source

WORKDIR /src
RUN git clone https://github.com/pwaller/pyfiglet . \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git pyfiglet/fonts-contrib test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# Stage 2: agent's room
FROM golang:1.23.4-alpine3.21

RUN apk add --no-cache bash \
    && adduser -D -u 1000 -s /bin/bash player \
    && mkdir -p /workspace/repo \
    && chown -R player:player /workspace

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
