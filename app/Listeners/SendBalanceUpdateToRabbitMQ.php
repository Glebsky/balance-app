<?php

namespace App\Listeners;

use App\Events\BalanceUpdated;
use App\Services\RabbitMQService;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Support\Facades\Log;

class SendBalanceUpdateToRabbitMQ implements ShouldQueue
{
    use InteractsWithQueue;

    /**
     * Create the event listener.
     */
    public function __construct(
        private readonly RabbitMQService $rabbitMQService
    ) {
    }

    /**
     * Handle the event.
     */
    public function handle(BalanceUpdated $event): void
    {
        try {
            $this->rabbitMQService->publish(
                'balance_updates',
                $event->toArray()
            );

            Log::debug('Balance update sent to RabbitMQ', [
                'user_id' => $event->userId,
                'version' => $event->version,
            ]);
        } catch (\Exception $e) {
            Log::error('Failed to send balance update to RabbitMQ', [
                'user_id' => $event->userId,
                'error' => $e->getMessage(),
                'trace' => $e->getTraceAsString(),
            ]);

            // Re-throw to allow queue retry mechanism
            throw $e;
        }
    }
}
