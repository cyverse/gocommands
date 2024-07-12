# GoCommands Dockerfile

FROM alpine:3.20.1
LABEL maintainer="Illyoung Choi <iychoi@email.arizona.edu>"
LABEL description="GoCommands"

ARG GOCOMMANDS_VER=1.0
LABEL version=${GOCOMMANDS_VER}
ENV GOCOMMANDS_VER=${GOCOMMANDS_VER}

# Setup Utility Packages
RUN arch=$(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/ | sed s/i386/386/ | sed s/i686/386/) && \
    echo https://github.com/cyverse/gocommands/releases/download/${GOCOMMANDS_VER}/gocmd-${GOCOMMANDS_VER}-linux-${arch}.tar.gz && \
    wget --quiet https://github.com/cyverse/gocommands/releases/download/${GOCOMMANDS_VER}/gocmd-${GOCOMMANDS_VER}-linux-${arch}.tar.gz -P /tmp && \
    mkdir /tmp/gocmd && \
    tar zxvf /tmp/gocmd-${GOCOMMANDS_VER}-linux-${arch}.tar.gz -C /tmp/gocmd && \
    install /tmp/gocmd/gocmd /usr/bin && \
    rm -rf /tmp/gocmd-${GOCOMMANDS_VER}-linux-${arch}.tar.gz /tmp/gocmd

# Fix libc dependency issue in alpine
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

WORKDIR /

ENTRYPOINT ["/usr/bin/gocmd"]