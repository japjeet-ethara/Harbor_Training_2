
# Stage 1: Clone and prepare the Python pyfiglet source
FROM alpine/git:v2.54.1 AS source

WORKDIR /tmp

RUN git clone https://github.com/pwaller/pyfiglet.git /tmp/pyfiglet \
    && cd /tmp/pyfiglet \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git \
    && rm -rf pyfiglet/fonts-contrib test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp -a pyfiglet/fonts-standard/. pyfiglet/fonts/ \
    && mkdir -p /src \
    && cp -a /tmp/pyfiglet/. /src/

# Stage 2: Go environment for the agent
FROM golang:1.23-alpine

# Install bash
RUN apk add --no-cache bash

# Create non-root player user with UID 1000
RUN adduser -D -u 1000 player

# Copy prepared repository from Stage 1
COPY --from=source --chown=player:player /src /workspace/repo

# Configure Go
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

# Run as non-root user
USER player

WORKDIR /workspace/repo
