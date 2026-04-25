FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /tmp/hubproxy ./cmd/hubproxy

FROM alpine:latest

COPY --from=builder /tmp/hubproxy /usr/bin/hubproxy

EXPOSE 8080 8081
CMD ["/usr/bin/hubproxy"]
