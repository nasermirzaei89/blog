FROM golang:1.24.5-alpine3.22 AS builder

WORKDIR /src

RUN apk update
RUN apk add make build-base

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN make build

###

FROM alpine

COPY --from=builder /src/bin/blog /

ENTRYPOINT ["/blog"]
