FROM php:8.3-cli

WORKDIR /var/www/html

# Установим нужные расширения PHP
RUN apt-get update && apt-get install -y \
    libzip-dev \
    libpng-dev \
    libonig-dev \
    default-mysql-client \
    unzip \
    && docker-php-ext-install pdo pdo_mysql sockets

# Копируем код
COPY ./ /var/www/html

# Установим composer
COPY --from=composer:2.8 /usr/bin/composer /usr/bin/composer

# Установим зависимости Laravel
RUN composer install --no-interaction --optimize-autoloader

# Команда по умолчанию
CMD ["php", "artisan"]
