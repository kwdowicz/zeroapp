# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/zeroapp .

FROM scratch
COPY --from=build /out/zeroapp /zeroapp
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

USER 65532:65532
EXPOSE 8080
ENV ADDR=:8080

ENTRYPOINT ["/zeroapp"]
