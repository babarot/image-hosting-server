FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY static/ static/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /image-hosting-server .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /image-hosting-server /usr/local/bin/image-hosting-server

EXPOSE 8080

ENTRYPOINT ["image-hosting-server"]
