# alias stc='docker run --rm -it -v /tmp:/tmp -v $HOME/.config/stc:/root/.config/stc xdrpp-stc'
FROM golang:bullseye AS build

RUN GOPROXY=direct go install github.com/xdrpp/stc/...@latest

FROM debian:bullseye-slim

COPY --from=build /go/bin/stc /usr/local/bin/stc

ENV EDITOR=vim
ENV TERM=xterm-256

RUN apt-get update && apt-get -y install vim \
    curl \ 
    jq && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["stc"]
