# Інструкція з запуску проекту Balance App

## Опис проекту

Проект складається з двох частин:
1. **Laravel додаток** - генерує користувачів, періодично оновлює баланси та відправляє події в RabbitMQ
2. **Golang мікросервіс** - отримує події з RabbitMQ, зберігає їх в PostgreSQL та підтримує кеш

## Вимоги

- Docker та Docker Compose
- PHP 8.2+
- Composer
- Go 1.21+ (якщо запускаєте Go сервіс локально)

## Крок 1: Запуск інфраструктури (Docker Compose)

```bash
# Запустити всі сервіси (MySQL, PostgreSQL, RabbitMQ)
docker-compose up -d

# Перевірити статус
docker-compose ps
```

Сервіси будуть доступні:
- MySQL: `localhost:3306`
- PostgreSQL: `localhost:5432`
- RabbitMQ Management UI: `http://localhost:15672` (guest/guest)

## Крок 2: Налаштування Laravel

```bash
# Встановити залежності
composer install

# Скопіювати .env файл
cp .env.example .env

# Згенерувати ключ додатку
php artisan key:generate

# Налаштувати .env файл (вже налаштовано для Docker)
# DB_CONNECTION=mysql
# DB_HOST=127.0.0.1
# DB_PORT=3306
# DB_DATABASE=balance_app
# DB_USERNAME=balance_user
# DB_PASSWORD=balance_pass

# Запустити міграції
php artisan migrate

# Запустити сідери для генерації 1000 користувачів
php artisan db:seed
```

## Крок 3: Запуск Laravel Scheduler

Laravel scheduler автоматично оновлює баланси кожні 10 секунд.

```bash
# Запустити scheduler (в окремому терміналі)
php artisan schedule:work
```

Або для production:
```bash
# Додати в crontab
* * * * * cd /path-to-project && php artisan schedule:run >> /dev/null 2>&1
```

## Крок 4: Запуск Queue Worker (для обробки подій)

```bash
# Запустити queue worker (в окремому терміналі)
php artisan queue:work
```

## Крок 5: Запуск Golang мікросервісу

### Варіант 1: Через Docker Compose (рекомендовано)

```bash
# Go сервіс автоматично запуститься з docker-compose
docker-compose up go-service

# Перевірити логи
docker-compose logs -f go-service
```

### Варіант 2: Локально

```bash
cd go-service

# Встановити залежності
go mod download

# Налаштувати змінні оточення
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=balance_db
export RABBITMQ_HOST=localhost
export RABBITMQ_PORT=5672
export RABBITMQ_USER=guest
export RABBITMQ_PASSWORD=guest

# Запустити
go run main.go
```

## Перевірка роботи системи

### 1. Перевірка Laravel

```bash
# Перевірити кількість користувачів
php artisan tinker
>>> User::count()
>>> Balance::count()

# Перевірити останні оновлення балансів
>>> Balance::latest('updated_at')->take(10)->get()
```

### 2. Перевірка RabbitMQ

Відкрити RabbitMQ Management UI: `http://localhost:15672`
- Логін: `guest`
- Пароль: `guest`
- Перевірити чергу `balance_updates` на наявність повідомлень

### 3. Перевірка Go сервісу

```bash
# Перевірити логи
docker-compose logs go-service

# Або підключитися до PostgreSQL
docker-compose exec postgres psql -U postgres -d balance_db

# Перевірити дані
SELECT COUNT(*) FROM balance_updates;
SELECT * FROM balance_updates ORDER BY created_at DESC LIMIT 10;
```

## Архітектура та особливості

### Laravel частина

1. **Оптимізовані bulk updates** - використовує raw SQL з CASE statements для масового оновлення балансів
2. **Event/Listener pattern** - події для decoupling логіки
3. **RabbitMQ Service** - з автоматичним reconnect та retry логікою
4. **Scheduled tasks** - автоматичне оновлення балансів кожні 10 секунд

### Golang мікросервіс

1. **Idempotency** - перевірка timestamp для уникнення дублікатів
2. **Thread-safe кеш** - використання sync.Map для безпечної роботи з кешем
3. **Періодична синхронізація** - автоматична синхронізація кешу з БД
4. **Dead-letter queue** - обробка помилкових повідомлень
5. **Connection pooling** - оптимізоване підключення до PostgreSQL

## Налаштування

### Зміна інтервалу оновлення балансів

У файлі `routes/console.php`:
```php
Schedule::command(UpdateBalancesCommand::class)
    ->everyTenSeconds() // Змінити на ->everyFifteenSeconds() для 15 секунд
```

### Налаштування синхронізації кешу (Go)

У `docker-compose.yml` або змінних оточення:
```yaml
SYNC_INTERVAL_SECONDS: 30  # Інтервал синхронізації
SYNC_BATCH_SIZE: 100        # Розмір батчу для синхронізації
```

## Troubleshooting

### Проблеми з підключенням до RabbitMQ

```bash
# Перевірити статус RabbitMQ
docker-compose ps rabbitmq

# Перевірити логи
docker-compose logs rabbitmq

# Перезапустити
docker-compose restart rabbitmq
```

### Проблеми з базою даних

```bash
# Перевірити підключення до MySQL
docker-compose exec mysql mysql -u balance_user -pbalance_pass balance_app

# Перевірити підключення до PostgreSQL
docker-compose exec postgres psql -U postgres -d balance_db
```

### Проблеми з Go сервісом

```bash
# Перевірити логи
docker-compose logs -f go-service

# Перезапустити
docker-compose restart go-service
```

## Зупинка сервісів

```bash
# Зупинити всі сервіси
docker-compose down

# Зупинити та видалити volumes (видалить дані)
docker-compose down -v
```

## Production рекомендації

1. Використовуйте Redis для кешування в Laravel
2. Налаштуйте supervisor для queue workers
3. Використовуйте PostgreSQL connection pooling (PgBouncer)
4. Налаштуйте monitoring та alerting
5. Використовуйте HTTPS для RabbitMQ Management UI
6. Налаштуйте backup для баз даних

