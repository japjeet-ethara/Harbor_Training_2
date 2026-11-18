
# Stage 1: Prepare the pyfiglet source and fonts
FROM alpine:3.20 AS source

RUN apk add --no-cache git

WORKDIR /src

RUN git clone https://github.com/pwaller/pyfiglet.git . \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git pyfiglet/fonts-contrib test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# Stage 2: Create the final Go environment
FROM golang:1.23-alpine

RUN apk add --no-cache bash \
    && adduser -D -u 1000 player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
