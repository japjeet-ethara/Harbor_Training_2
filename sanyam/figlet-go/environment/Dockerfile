# Stage 1: fetch pyfiglet at the pinned commit and finish its own setup (Makefile: copy
# fonts-standard into pyfiglet/fonts). Nothing from this stage but /src reaches the agent.
FROM alpine/git@sha256:062a01ad7a0eb17cff382bc5e26086b4d710e56dfdfdf001109a49b6d9bd378c AS source
RUN git clone https://github.com/pwaller/pyfiglet /src \
    && cd /src \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git pyfiglet/fonts-contrib test-fonts \
    && mkdir pyfiglet/fonts \
    && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# Stage 2: the agent's room. golang:1.23-alpine has no python; bash is for Harbor.
FROM golang@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f
RUN apk add --no-cache bash && adduser -D -u 1000 player
COPY --from=source --chown=player:player /src /workspace/repo
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
USER player
WORKDIR /workspace/repo
