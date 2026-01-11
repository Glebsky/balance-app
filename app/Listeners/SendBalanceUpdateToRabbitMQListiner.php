<?php

namespace App\Listeners;

use App\Events\BalanceUpdatedEvent;
use App\Services\RabbitMQService;
use Exception;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\Log;

class SendBalanceUpdateToRabbitMQListiner implements ShouldQueue
{
    use InteractsWithQueue;

    /**
     * The number of times the job may be attempted.
     */
    public int $tries = 3;

    /**
     * The number of seconds to wait before retrying the job.
     */
    public int $backoff = 5;

    /**
     * Create the event listener.
     */
    public function __construct(
        private readonly RabbitMQService $rabbitMQService
    )
    {
    }

    /**
     * Handle the event.
     */
    public function handle(BalanceUpdatedEvent $event): void
    {
        // Create a unique lock key to prevent duplicate processing
        $lockKey = 'balance_update_lock_' . $event->userId . '_' . $event->version;

        // Try to acquire a lock (expires in 60 seconds)
        $lock = Cache::lock($lockKey, 60);

        if (!$lock->get()) {
            Log::warning('Duplicate balance update detected, skipping', [
                'user_id' => $event->userId,
                'version' => $event->version,
            ]);
            return;
        }

        try {
            $this->rabbitMQService->publish(
                'balance_updates',
                $event->toArray()
            );

            Log::debug('Balance update sent to RabbitMQ', [
                'user_id' => $event->userId,
                'version' => $event->version,
            ]);
        } catch (Exception $e) {
            Log::error('Failed to send balance update to RabbitMQ', [
                'user_id' => $event->userId,
                'error'   => $e->getMessage(),
                'trace'   => $e->getTraceAsString(),
            ]);

            // Re-throw to allow queue retry mechanism
            throw $e;
        } finally {
            // Release the lock
            $lock->release();
        }
    }
}
