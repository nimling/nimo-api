FROM golang:1.25-alpine AS builder

ARG GOOS=linux
ARG GOARCH=amd64
ARG VERSION=dev

RUN apk add --no-cache git make

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go version
RUN CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build -a -installsuffix cgo -ldflags "-X github.com/nimling/nimo-api/internal.Version=${VERSION}" -o nimo$(if [ "$GOOS" = "windows" ]; then echo ".exe"; fi) ./cmd

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/
COPY --from=builder /app/nimo* .

CMD ["./nimo"]
