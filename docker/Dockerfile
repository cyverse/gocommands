# GoCommands Dockerfile
FROM golang:1.23-alpine AS builder

RUN apk add make git

WORKDIR /app

COPY go.mod go.sum ./

COPY . .
RUN make build

FROM alpine:3.20.1
LABEL maintainer="Illyoung Choi <iychoi@email.arizona.edu>"
LABEL description="GoCommands"

# Fix libc dependency issue in alpine
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

COPY --from=builder /app/bin/gocmd /usr/bin/

# Setup User
RUN addgroup gocmd && adduser -G gocmd -D gocmd

USER gocmd
WORKDIR /home/gocmd

ENTRYPOINT ["/usr/bin/gocmd"]