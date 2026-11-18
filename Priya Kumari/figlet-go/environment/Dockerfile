
# Stage 1: Download the original PyFiglet source
FROM alpine:3.21 AS source

RUN apk add --no-cache git

WORKDIR /src

RUN git clone https://github.com/pwaller/pyfiglet.git pyfiglet \
    && cd pyfiglet \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git fonts-contrib test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp -R pyfiglet/fonts-standard/. pyfiglet/fonts/

# Stage 2: Create the Go environment
FROM golang:1.23-alpine

RUN apk add --no-cache bash \
    && adduser -D -u 1000 player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

WORKDIR /workspace/repo

USER player
