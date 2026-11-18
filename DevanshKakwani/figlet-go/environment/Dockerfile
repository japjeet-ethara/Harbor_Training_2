# Stage 1: fetch pyfiglet at a fixed commit and prepare the source tree.
FROM alpine/git:v2.47.2@sha256:062a01ad7a0eb17cff382bc5e26086b4d710e56dfdfdf001109a49b6d9bd378c AS source

RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git \
 && rm -rf pyfiglet/fonts-contrib test-fonts \
 && mkdir pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# Stage 2: the agent's environment (Go only, no Python).
FROM golang:1.23-alpine

RUN apk add --no-cache bash \
 && adduser -D -u 1000 player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
