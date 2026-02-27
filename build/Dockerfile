FROM golang:1.26-trixie AS build

WORKDIR /balda

COPY . ./

RUN make build

FROM debian:trixie-slim

COPY --from=build /balda/bin/balda /usr/local/bin/balda
ENTRYPOINT ["balda"]
