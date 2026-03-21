FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o purpleops .
RUN CGO_ENABLED=0 go build -o seed ./cmd/seed

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git netcat-openbsd
RUN addgroup -S purpleops && adduser -S purpleops -G purpleops
WORKDIR /usr/src/app
COPY --from=builder /build/purpleops .
COPY --from=builder /build/seed .
COPY templates/ templates/
COPY static/ static/
COPY custom/ custom/
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh && \
    chown -R purpleops:purpleops /usr/src/app
USER purpleops
HEALTHCHECK --interval=30s --timeout=5s CMD nc -z localhost 8888 || exit 1
ENTRYPOINT ["./entrypoint.sh"]
