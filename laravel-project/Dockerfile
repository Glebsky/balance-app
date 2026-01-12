FROM php:8.3-cli

WORKDIR /var/www/html

RUN apt-get update && apt-get install -y \
    libzip-dev \
    libpng-dev \
    libonig-dev \
    default-mysql-client \
    unzip \
    && docker-php-ext-install pdo pdo_mysql sockets

COPY ./ /var/www/html

COPY --from=composer:2.8 /usr/bin/composer /usr/bin/composer

RUN composer install --no-interaction --optimize-autoloader

CMD ["php", "artisan"]
