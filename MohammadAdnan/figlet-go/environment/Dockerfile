# Stage 1 ("source"): fetch pyfiglet at a pinned commit and finish its own setup.
# git and the rest of this stage never reach the agent's room.
FROM alpine/git:2.47.2@sha256:062a01ad7a0eb17cff382bc5e26086b4d710e56dfdfdf001109a49b6d9bd378c AS source
RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# Stage 2: the agent's room. Go on Alpine (no Python), bash for Harbor, non-root user.
FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f
RUN apk add --no-cache bash \
 && adduser -D -u 1000 player
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local
COPY --from=source --chown=player:player /src /workspace/repo
USER player
WORKDIR /workspace/repo
