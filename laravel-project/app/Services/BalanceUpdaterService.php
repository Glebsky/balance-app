<?php

namespace App\Services;

use App\Events\BalanceUpdatedEvent;
use App\Models\Balance;
use Illuminate\Contracts\Events\Dispatcher;
use Illuminate\Database\QueryException;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Log;
use Throwable;

class BalanceUpdaterService
{
    private readonly int $maxRetries;

    public function __construct(private readonly Dispatcher $events)
    {
        $this->maxRetries = (int)config('balance.max_deadlock_retries', 5);
    }

    /**
     * Update a random group of balances with pessimistic locking and batching.
     */
    public function updateRandomGroup(int $batchSize = 50): int
    {
        $attempt = 0;

        do {
            try {
                $updates = DB::transaction(function () use ($batchSize) {
                    $now     = now();
                    $updates = [];

                    $balances = Balance::query()
                        ->select(['id', 'user_id', 'amount', 'version', 'created_at'])
                        ->inRandomOrder()
                        ->limit($batchSize)
                        ->lock('FOR UPDATE SKIP LOCKED')
                        ->cursor();

                    if ($balances->isEmpty()) {
                        return [];
                    }

                    $updates = $balances->map(function ($balance) use ($now) {
                        return [
                            'id'         => $balance->id,
                            'user_id'    => $balance->user_id,
                            'amount'     => max(0, round((float)$balance->amount + $this->randomDelta(), 2)),
                            'version'    => $balance->version + 1,
                            'created_at' => $balance->created_at ?? $now,
                            'updated_at' => $now,
                        ];
                    })->toArray();

                    Balance::query()->upsert(
                        $updates,
                        ['id'],
                        ['amount', 'version', 'updated_at']
                    );

                    return $updates;
                });

                $this->dispatchUpdates($updates);

                return count($updates);
            } catch (Throwable $e) {
                $attempt++;

                if (!$this->shouldRetry($e, $attempt)) {
                    throw $e;
                }

                usleep($this->backoffMicros($attempt));
            }
        } while ($attempt < $this->maxRetries);

        return 0;
    }

    private function dispatchUpdates(array $updates): void
    {
        foreach ($updates as $update) {
            Log::debug('Dispatching event', ['user_id' => $update['user_id']]);
            $this->events->dispatch(new BalanceUpdatedEvent(
                $update['user_id'],
                $update['amount'],
                (int)$update['version'],
                now()
            ));
        }
    }

    private function randomDelta(): float
    {
        // Simulate +/- 100 currency units to exercise versioning logic
        return random_int(-5000, 5000) / 100;
    }

    private function backoffMicros(int $attempt): int
    {
        return min(2_000_000, 500_000 * $attempt);
    }

    private function shouldRetry(Throwable $e, int $attempt): bool
    {
        $sqlState   = $e instanceof QueryException ? ($e->getCode() ?? '') : (string)$e->getCode();
        $isDeadlock = str_starts_with($sqlState, '40P01') || str_starts_with($sqlState, '40001');

        if ($isDeadlock && $attempt < $this->maxRetries) {
            Log::warning('Deadlock detected while updating balances, retrying', [
                'attempt'     => $attempt,
                'max_retries' => $this->maxRetries,
            ]);

            return true;
        }

        return false;
    }
}
