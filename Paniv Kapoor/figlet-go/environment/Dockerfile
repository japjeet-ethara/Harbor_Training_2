# Stage 1: fetch the pinned pyfiglet source, then drop git history and extra fonts.
FROM alpine/git:v2.54.0 AS source

WORKDIR /src

RUN git clone https://github.com/pwaller/pyfiglet.git . \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git \
    && rm -rf pyfiglet/fonts-contrib pyfiglet/test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp -a pyfiglet/fonts-standard/. pyfiglet/fonts/

# Stage 2: Go build environment. This stage has no Python.
FROM golang:1.23-alpine

RUN apk add --no-cache bash \
    && addgroup -g 1000 player \
    && adduser -D -u 1000 -G player -s /bin/bash player

COPY --from=source /src /workspace/repo
RUN chown -R player:player /workspace/repo

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

WORKDIR /workspace/repo
USER player
