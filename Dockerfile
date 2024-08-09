FROM golang:1.22.6-alpine AS builder

ARG GITHUB_TOKEN

# Setup
RUN mkdir -p /go/src/github.com/zekihan/sdash
WORKDIR /go/src/github.com/zekihan/sdash

RUN apk add --no-cache git ca-certificates make

COPY go.mod .
COPY go.sum .

ENV CGO_ENABLED=0 GO111MODULE=on GOOS=linux

RUN go mod download

COPY . .

RUN make build

# Copy into scratch container
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/src/github.com/zekihan/sdash/tmp/bin/sdash ./

ENTRYPOINT ["./sdash"]
