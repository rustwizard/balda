FROM golang:1.17-buster as build

WORKDIR /balda

COPY . ./

RUN make build

FROM debian:buster-slim

COPY --from=build /balda/bin/balda /usr/local/bin/balda
ENTRYPOINT ["balda"]
