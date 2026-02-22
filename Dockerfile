# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/aretheyup ./main.go

FROM alpine:3.21

WORKDIR /app

RUN addgroup -S app && adduser -S -G app app && apk add --no-cache ca-certificates

COPY --from=builder /out/aretheyup ./aretheyup
COPY templates ./templates
COPY static ./static

USER app

EXPOSE 8080

ENTRYPOINT ["./aretheyup"]
