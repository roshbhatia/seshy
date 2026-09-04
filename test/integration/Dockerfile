FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /usr/local/bin/sy ./cmd/sy

# ── runtime image ────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache git bash

COPY --from=builder /usr/local/bin/sy /usr/local/bin/sy
COPY test/integration/run_tests.sh /run_tests.sh
RUN chmod +x /run_tests.sh

ENTRYPOINT ["/run_tests.sh"]
