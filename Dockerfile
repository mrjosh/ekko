FROM golang:1.16-alpine AS builder

LABEL maintainer="Alireza Josheghani <josheghani.dev@gmail.com>"

RUN apt update && apt -y upgrade

RUN apt install -y portaudio19-dev pkg-config libopus-dev libopusfile-dev

# For mac install these packages:
# brew install pkg-config opus opusfile portaudio

# Creating work directory
WORKDIR /build

# Adding project to work directory
ADD . /build

# build project
RUN go build -o casty .

FROM alpine:latest

COPY --from=builder /build/casty /usr/bin/casty

EXPOSE 3000
EXPOSE 62155

ENTRYPOINT ["/usr/bin/casty"]
