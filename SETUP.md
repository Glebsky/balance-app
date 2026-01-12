# Інструкція з запуску проекту Balance App

## Опис проекту

Проект складається з двох частин:
1. **Laravel додаток** - генерує користувачів, періодично оновлює баланси та відправляє події в RabbitMQ
2. **Golang мікросервіс** - отримує події з RabbitMQ, зберігає їх в MySQl та підтримує кеш

## Вимоги

- Docker та Docker Compose


```bash
# 1. Налаштувати інфраструктуру
cp ./go-project/.env.example ./go-project/.env
cp ./laravel-project/.env.example ./laravel-project/.env

# 2. Налаштувати Laravel
docker compose run --rm laravel-worker bash -c "php artisan key:generate && php artisan migrate:fresh --seed"

# 3. Запуск докер
docker compose up -d
```


Сервіси будуть доступні:
- MySQL: `localhost:3306`
- RabbitMQ Management UI: `http://localhost:15672` (guest/guest)

## Laravel Scheduler - balance-laravel-scheduler
Laravel scheduler автоматично оновлює баланси кожні 10 секунд.

##  Queue Worker (для обробки подій) - balance-laravel-worker
Laravel scheduler автоматично оброблює запити до RabbitMQ.

## Golang мікросервіс - balance-go-worker

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
- Логін: `balance`
- Пароль: `balance`
- Перевірити чергу `balance_updates` на наявність повідомлень

### 3. Перевірка Go сервісу

```bash
# Перевірити логи
docker-compose logs go-service
```

## Архітектура та особливості

### Laravel частина

1. **Оптимізовані bulk updates**
2. **Event/Listener pattern** - події для decoupling логіки
3. **RabbitMQ Service** - з автоматичним reconnect та retry логікою
4. **Scheduled tasks** - автоматичне оновлення балансів кожні 10 секунд

### Golang мікросервіс
1. **Idempotency** - перевірка timestamp для уникнення дублікатів
2. **Thread-safe кеш** - використання sync.Map для безпечної роботи з кешем
3. **Періодична синхронізація** - автоматична синхронізація кешу з БД
4. **Dead-letter queue** - обробка помилкових повідомлень

## Налаштування

### Зміна інтервалу оновлення балансів

У файлі `routes/console.php`:
```php
Schedule::command(UpdateBalancesCommand::class)
    ->everyTenSeconds() // Змінити на ->everyFifteenSeconds() для 15 секунд
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
docker-compose exec mysql mysql -u laravel -laravel laravel_db
docker-compose exec mysql-go mysql -u go -go laravel_db
```

### Проблеми з Go сервісом

```bash
# Перевірити логи
docker-compose logs -f balance-go-worker

# Перезапустити
docker-compose restart balance-go-worker
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
4. Налаштуйте monitoring та alerting
5. Використовуйте HTTPS для RabbitMQ Management UI
6. Налаштування backup для баз даних

