# Stage 1: fetch and prepare pyfiglet source
FROM alpine:3.20 AS source

RUN apk add --no-cache git

WORKDIR /src
RUN git clone https://github.com/pwaller/pyfiglet . && \
    git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 && \
    rm -rf .git pyfiglet/fonts-contrib test-fonts && \
    mkdir -p pyfiglet/fonts && \
    cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# Stage 2: agent workspace room
FROM golang:1.23-alpine

RUN apk add --no-cache bash

RUN adduser -D -u 1000 player

COPY --from=source --chown=player:player /src /workspace/repo
RUN chown player:player /workspace/repo

WORKDIR /workspace/repo

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

USER player
