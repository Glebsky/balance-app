# syntax=docker/dockerfile:1
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копируем модули сначала, чтобы кешировать слои
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь код
COPY . .

# Собираем бинарь
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o balance-consumer ./main.go

# Минимальный runtime образ
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/balance-consumer .
CMD ["./balance-consumer"]
