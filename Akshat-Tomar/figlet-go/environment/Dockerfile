# ---- Stage 1: fetch and prepare pyfiglet source ----
FROM alpine/git:2.45.2 AS source

RUN git clone https://github.com/pwaller/pyfiglet /src && \
    cd /src && \
    git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 && \
    rm -rf .git && \
    rm -rf pyfiglet/fonts-contrib && \
    rm -rf test-fonts && \
    mkdir -p pyfiglet/fonts && \
    cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---- Stage 2: the agent's working room ----
FROM golang:1.23-alpine3.20

# Install bash (Alpine has only sh by default; Harbor needs bash)
RUN apk add --no-cache bash=5.2.26-r0

# Create a normal user 'player' with uid 1000
RUN adduser -D -u 1000 player

# Copy the prepared pyfiglet repo into the workspace, owned by player
COPY --from=source --chown=player:player /src /workspace/repo

# Go build settings
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
