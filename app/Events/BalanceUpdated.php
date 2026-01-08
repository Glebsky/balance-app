<?php

namespace App\Events;

use Illuminate\Foundation\Events\Dispatchable;
use Illuminate\Queue\SerializesModels;

class BalanceUpdated
{
    use Dispatchable, SerializesModels;

    /**
     * Create a new event instance.
     */
    public function __construct(
        public readonly int $userId,
        public readonly float $oldAmount,
        public readonly float $newAmount,
        public readonly int $version,
        public readonly \DateTimeInterface $timestamp
    ) {
    }

    /**
     * Convert the event to an array for RabbitMQ.
     */
    public function toArray(): array
    {
        return [
            'user_id' => $this->userId,
            'old_amount' => $this->oldAmount,
            'new_amount' => $this->newAmount,
            'version' => $this->version,
            'timestamp' => $this->timestamp->format('c'), // ISO 8601 format
            'event_id' => uniqid('balance_', true), // Unique event ID for idempotency
        ];
    }
}
