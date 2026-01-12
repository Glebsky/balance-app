# Balance App

Система для управління балансами користувачів з мікросервісною архітектурою.

## Архітектура

Проект складається з двох частин:

1. **Laravel додаток** - генерує користувачів, періодично оновлює баланси та відправляє події в RabbitMQ
2. **Golang мікросервіс** - отримує події з RabbitMQ, зберігає їх в MySql та підтримує кеш

## Швидкий старт

Детальні інструкції дивіться в [SETUP.md](SETUP.md)

```bash
# 1. Налаштувати інфраструктуру
cp ./go-project/.env.example ./go-project/.env
cp ./laravel-project/.env.example ./laravel-project/.env

# 2. Налаштувати Laravel
docker compose run --rm laravel-worker bash -c "php artisan key:generate && php artisan migrate:fresh --seed"

# 3. Запуск докер
docker compose up -d
```

## Особливості реалізації

### Laravel
- ✅ Оптимізовані bulk updates
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
- ✅ Graceful shutdown

## Структура проекту

```
balance-app/
├── app/
│   ├── Console/Commands/UpdateBalancesCommand.php
│   ├── Events/BalanceUpdatedEvent.php
│   ├── Listeners/SendBalanceUpdateToRabbitMQListiner.php
│   ├── Models/Balance.php
│   ├── Models/User.php
│   └── Services/BalanceUpdaterService.php.php
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
├── Dockerfile
└── SETUP.md
```
