FROM golang:1.20-bullseye as build

WORKDIR /build

COPY . .

RUN ["make", "cedar"]

FROM debian:bullseye

WORKDIR /project

COPY --from=build /build/build/cedar .

RUN apt-get update && apt-get install -y ca-certificates

CMD /project/cedar --level=info service --rpcUserAuth --workers=1