FROM golang:alpine

RUN apk add --no-cache make git

WORKDIR /build
ADD . /build
RUN make update-depend
RUN make
RUN mv stc /go/bin/stc

WORKDIR /stc
VOLUME ["/stc"]
CMD ["/go/bin/stc"]
