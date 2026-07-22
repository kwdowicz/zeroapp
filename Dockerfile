# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/zeroapp .

FROM scratch
COPY --from=build /out/zeroapp /zeroapp

USER 65532:65532
EXPOSE 8080
ENV ADDR=:8080

ENTRYPOINT ["/zeroapp"]
