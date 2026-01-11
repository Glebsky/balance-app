# Balance App

Система для управління балансами користувачів з мікросервісною архітектурою.

## Архітектура

Проект складається з двох частин:

1. **Laravel додаток** - генерує користувачів, періодично оновлює баланси та відправляє події в RabbitMQ
2. **Golang мікросервіс** - отримує події з RabbitMQ, зберігає їх в PostgreSQL та підтримує кеш

## Швидкий старт

Детальні інструкції дивіться в [SETUP.md](SETUP.md)

```bash
# 1. Запустити інфраструктуру
cp .env.example .env
docker-compose run --rm --no-deps laravel-worker composer install
docker-compose up -d

# 2. Налаштувати Laravel
docker-compose exec laravel-worker php artisan key:generate
docker-compose exec laravel-worker php artisan migrate
docker-compose exec laravel-worker php artisan db:seed
```

## Особливості реалізації

### Laravel
- ✅ Оптимізовані bulk updates через raw SQL
- ✅ Event/Listener pattern для decoupling
- ✅ RabbitMQ Service з автоматичним reconnect
- ✅ Scheduled tasks для періодичного оновлення
- ✅ Dependency Injection
- ✅ Error handling та logging

### Golang
- ✅ Idempotency через timestamp перевірку
- ✅ Thread-safe кеш з sync.Map
- ✅ Періодична синхронізація кешу з БД
- ✅ Dead-letter queue для обробки помилок
- ✅ Connection pooling для PostgreSQL
- ✅ Graceful shutdown

## Структура проекту

```
balance-app/
├── app/
│   ├── Console/Commands/UpdateBalancesCommand.php
│   ├── Events/BalanceUpdated.php
│   ├── Listeners/SendBalanceUpdateToRabbitMQ.php
│   ├── Models/
│   └── Services/RabbitMQService.php
├── go-service/
│   ├── internal/
│   │   ├── config/
│   │   ├── consumer/
│   │   ├── database/
│   │   ├── logger/
│   │   └── sync/
│   └── main.go
├── docker-compose.yml
└── SETUP.md
```

## Ліцензія

MIT License
