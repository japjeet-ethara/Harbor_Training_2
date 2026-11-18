# syntax=docker/dockerfile:1
#
# Two-stage room for the figlet-go task.
#  - stage "source": fetches pyfiglet at a pinned commit and finishes its setup
#    (the Makefile's `minimal` step: copy fonts-standard/ into pyfiglet/fonts/).
#  - final stage: Go on Alpine, bash for Harbor, non-root user "player", no Python.
#
# Network notes (works behind HTTPS-inspecting proxies / antivirus):
#  - pyfiglet is downloaded by Docker itself (ADD), which trusts the same
#    certificates as your machine, instead of by `git clone` inside a RUN step.
#  - apk uses plain-http mirrors. Safe: every Alpine package is signature-checked
#    by apk against the keys already in the image, whatever the transport.
#
# Pinned to a fixed patch tag. For a byte-identical room forever, replace the tag
# with its digest:  docker inspect --format '{{index .RepoDigests 0}}' golang:1.23.12-alpine3.22

ARG GO_IMAGE=golang:1.23.12-alpine3.22

############################ stage 1: source ############################
FROM ${GO_IMAGE} AS source

ARG PYFIGLET_SHA=255827efbc09d662875dad50fa0edc1e1263bf45

# GitHub's archive of exactly this commit (no .git folder inside).
ADD https://github.com/pwaller/pyfiglet/archive/${PYFIGLET_SHA}.tar.gz /tmp/pyfiglet.tar.gz

# Last three lines = the Makefile's `clean` + `minimal` font step:
# fonts live in fonts-standard/, but the code loads them from pyfiglet/fonts/.
RUN mkdir /tmp/x \
 && tar -xzf /tmp/pyfiglet.tar.gz -C /tmp/x \
 && mv /tmp/x/pyfiglet-* /src \
 && cd /src \
 && test -f pyfiglet/__init__.py \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && rm -rf pyfiglet/fonts \
 && mkdir pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

############################ stage 2: the agent's room ############################
FROM ${GO_IMAGE}

# bash is required by Harbor; nothing else is added (in particular no python).
RUN sed -i 's#^https://#http://#' /etc/apk/repositories \
 && apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player \
 && mkdir -p /workspace \
 && chown player:player /workspace

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

COPY --from=source --chown=player:player /src /workspace/repo

USER player
WORKDIR /workspace/repo
