FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY static/ static/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /image-hosting-server .

FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /data/images \
    && chown 65532:65532 /data/images

COPY --from=builder /image-hosting-server /usr/local/bin/image-hosting-server

# Run as an unprivileged user. Override with --user (or compose's user:) to
# match the owner of the mounted upload directory.
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["image-hosting-server"]
