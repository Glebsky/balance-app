# syntax=docker/dockerfile:1
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o balance-consumer ./main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/balance-consumer .
CMD ["./balance-consumer"]
