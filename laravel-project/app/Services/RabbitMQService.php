<?php

namespace App\Services;

use Exception;
use Illuminate\Support\Facades\Log;
use PhpAmqpLib\Channel\AMQPChannel;
use PhpAmqpLib\Connection\AMQPStreamConnection;
use PhpAmqpLib\Message\AMQPMessage;

class RabbitMQService
{
    private ?AMQPStreamConnection $connection = NULL;
    private ?AMQPChannel          $channel    = NULL;
    private int                   $maxRetries = 3;
    private int                   $retryDelay = 1000; // milliseconds

    public function __construct(
        private readonly string $host,
        private readonly int $port,
        private readonly string $user,
        private readonly string $password,
        private readonly string $vhost = '/'
    )
    {
    }

    /**
     * Declare exchange and queue with persistent settings.
     */
    public function declareQueue(string $queueName, string $exchangeName = 'balance_exchange'): void
    {
        $channel = $this->getChannel();

        // Declare exchange (durable for persistence)
        $channel->exchange_declare(
            $exchangeName,
            'direct',
            false, // passive
            true, // durable
            false // auto_delete
        );

        // Declare queue (durable for persistence)
        $channel->queue_declare(
            $queueName,
            false, // passive
            true, // durable
            false, // exclusive
            false // auto_delete
        );

        // Bind queue to exchange
        $channel->queue_bind($queueName, $exchangeName, $queueName);
    }

    /**
     * Publish message to RabbitMQ.
     * @throws Exception
     */
    public function publish(string $queueName, array $data, string $exchangeName = 'balance_exchange'): void
    {
        $attempt       = 0;
        $lastException = NULL;

        while ($attempt < $this->maxRetries) {
            try {
                $this->declareQueue($queueName, $exchangeName);
                $channel = $this->getChannel();

                $messageBody = json_encode($data, JSON_THROW_ON_ERROR);
                $message     = new AMQPMessage(
                    $messageBody,
                    [
                        'delivery_mode' => AMQPMessage::DELIVERY_MODE_PERSISTENT, // Make message persistent
                        'content_type'  => 'application/json',
                        'timestamp'     => time(),
                    ]
                );

                // Enable publisher confirms
//                $channel->confirm_select();
                $channel->basic_publish(
                    $message,
                    $exchangeName,
                    $queueName
                );

                // Wait for confirmation
                $channel->wait_for_pending_acks(5);

                Log::debug('Message published to RabbitMQ', [
                    'queue'    => $queueName,
                    'exchange' => $exchangeName,
                ]);

                return;
            } catch (Exception $e) {
                $attempt++;
                $lastException = $e;

                // Reset connection on error
                $this->resetConnection();

                Log::error('Failed to publish message to RabbitMQ', [
                    'attempt'     => $attempt,
                    'max_retries' => $this->maxRetries,
                    'error'       => $e->getMessage(),
                ]);

                if ($attempt < $this->maxRetries) {
                    usleep($this->retryDelay * 1000 * $attempt);
                }
            }
        }

        throw new Exception(
            "Failed to publish message to RabbitMQ after {$this->maxRetries} attempts: " .
            ($lastException ? $lastException->getMessage() : 'Unknown error')
        );
    }

    /**
     * Close connections gracefully.
     */
    public function __destruct()
    {
        $this->resetConnection();
    }

    /**
     * Get or create RabbitMQ connection.
     */
    private function getConnection(): AMQPStreamConnection
    {
        if ($this->connection === NULL || !$this->connection->isConnected()) {
            $this->connect();
        }

        return $this->connection;
    }

    /**
     * Get or create RabbitMQ channel.
     */
    private function getChannel(): AMQPChannel
    {
        if ($this->channel === NULL || !$this->channel->is_open()) {
            $this->channel = $this->getConnection()->channel();
        }

        return $this->channel;
    }

    /**
     * Connect to RabbitMQ with retry logic.
     * @throws Exception
     */
    private function connect(): void
    {
        $attempt       = 0;
        $lastException = NULL;

        while ($attempt < $this->maxRetries) {
            try {
                $this->connection = new AMQPStreamConnection(
                    $this->host,
                    $this->port,
                    $this->user,
                    $this->password,
                    $this->vhost,
                    false, // insist
                    'AMQPLAIN', // login_method
                    NULL, // login_response
                    'en_US', // locale
                    3.0, // connection_timeout
                    3.0, // read_write_timeout
                    NULL, // context
                    false, // keepalive
                    0 // heartbeat
                );

                Log::info('Successfully connected to RabbitMQ', [
                    'host' => $this->host,
                    'port' => $this->port,
                ]);

                return;
            } catch (Exception $e) {
                $attempt++;
                $lastException = $e;

                Log::warning('Failed to connect to RabbitMQ', [
                    'attempt'     => $attempt,
                    'max_retries' => $this->maxRetries,
                    'error'       => $e->getMessage(),
                ]);

                if ($attempt < $this->maxRetries) {
                    usleep($this->retryDelay * 1000 * $attempt); // Exponential backoff
                }
            }
        }

        throw new Exception(
            "Failed to connect to RabbitMQ after {$this->maxRetries} attempts: " .
            ($lastException ? $lastException->getMessage() : 'Unknown error')
        );
    }

    /**
     * Reset connection and channel.
     */
    private function resetConnection(): void
    {
        try {
            if ($this->channel !== NULL && $this->channel->is_open()) {
                $this->channel->close();
            }
        } catch (Exception $e) {
            Log::warning('Error closing channel', ['error' => $e->getMessage()]);
        }

        try {
            if ($this->connection !== NULL && $this->connection->isConnected()) {
                $this->connection->close();
            }
        } catch (Exception $e) {
            Log::warning('Error closing connection', ['error' => $e->getMessage()]);
        }

        $this->channel    = NULL;
        $this->connection = NULL;
    }
}

