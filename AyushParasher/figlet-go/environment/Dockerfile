# ---------- Stage 1: "source" — fetch pyfiglet and finish its own setup ----------
# alpine/git v2.47.2, pinned by digest (D5)
FROM alpine/git@sha256:062a01ad7a0eb17cff382bc5e26086b4d710e56dfdfdf001109a49b6d9bd378c AS source

# Clone at the pinned commit, drop git history and non-standard fonts, then do
# D3: the Makefile's "clean" + "minimal" font step (without python3 -m build).
RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------- Stage 2: the agent's room ----------
# golang:1.23-alpine (Go 1.23.12, Alpine 3.22), pinned by digest (D5). No Python in it (D2).
FROM golang@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

# D6: Harbor runs its commands with bash, which Alpine does not ship.
# D4: normal user "player", uid 1000.
RUN apk add --no-cache bash \
 && adduser -D -u 1000 player

# D1: only the prepared source crosses over from stage 1 (no git, no .git)
COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo
