# syntax=docker/dockerfile:1

FROM alpine:3.20 AS source
WORKDIR /src
RUN apk add --no-cache git
RUN git clone https://github.com/pwaller/pyfiglet.git /src/pyfiglet && \
    cd /src/pyfiglet && \
    git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 && \
    rm -rf .git pyfiglet/fonts-contrib test-fonts && \
    mkdir -p pyfiglet/fonts && \
    cp pyfiglet/fonts-standard/* pyfiglet/fonts/

FROM golang:1.23-alpine
RUN apk add --no-cache bash
RUN adduser -D -u 1000 player
COPY --from=source /src /workspace/repo
RUN chown -R player:player /workspace/repo
WORKDIR /workspace/repo
ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local
USER player
