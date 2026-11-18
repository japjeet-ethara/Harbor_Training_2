FROM golang:1.23-alpine

RUN apk add --no-cache bash

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

WORKDIR /app

COPY . /tests/
