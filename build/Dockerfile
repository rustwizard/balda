FROM golang:1.21-bookworm as build

WORKDIR /balda

COPY . ./

RUN make build

FROM debian:bookworm-slim

COPY --from=build /balda/bin/balda /usr/local/bin/balda
ENTRYPOINT ["balda"]
