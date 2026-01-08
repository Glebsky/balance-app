<?php

namespace App\Console\Commands;

use App\Events\BalanceUpdated;
use App\Models\Balance;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Log;

class UpdateBalancesCommand extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'balances:update 
                            {--batch-size=50 : Number of balances to update per batch}
                            {--users-count= : Number of random users to select (default: random 10-100)}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Update balances for random users using optimized bulk updates';

    /**
     * Execute the console command.
     */
    public function handle(): int
    {
        $batchSize = (int) $this->option('batch-size');
        $usersCount = $this->option('users-count') 
            ? (int) $this->option('users-count') 
            : rand(10, 100);

        $this->info("Updating balances for {$usersCount} random users...");

        try {
            // Select random users with their balances
            $balances = Balance::query()
                ->inRandomOrder()
                ->limit($usersCount)
                ->lockForUpdate() // Prevent concurrent updates
                ->get();

            if ($balances->isEmpty()) {
                $this->warn('No balances found to update.');
                return Command::SUCCESS;
            }

            $updatedBalances = [];
            $now = now();

            // Process in batches for better performance
            foreach ($balances->chunk($batchSize) as $chunk) {
                $updates = [];

                foreach ($chunk as $balance) {
                    $oldAmount = $balance->amount;
                    $change = rand(-500, 500) / 100;
                    $newAmount = max(0, $oldAmount + $change); // Prevent negative balances
                    $newVersion = $balance->version + 1;

                    $updates[] = [
                        'id' => $balance->id,
                        'old_amount' => $oldAmount,
                        'new_amount' => $newAmount,
                        'new_version' => $newVersion,
                        'user_id' => $balance->user_id,
                    ];
                }

                // Bulk update using raw SQL with CASE statements for maximum performance
                $ids = array_column($updates, 'id');
                $idsString = implode(',', $ids);
                
                // Build CASE statements for amount
                $amountCases = [];
                $versionCases = [];
                foreach ($updates as $update) {
                    $amountCases[] = "WHEN {$update['id']} THEN " . $update['new_amount'];
                    $versionCases[] = "WHEN {$update['id']} THEN {$update['new_version']}";
                }
                
                $amountCase = implode(' ', $amountCases);
                $versionCase = implode(' ', $versionCases);

                DB::statement("
                    UPDATE balances 
                    SET 
                        amount = CASE id {$amountCase} ELSE amount END,
                        version = CASE id {$versionCase} ELSE version END,
                        updated_at = ?
                    WHERE id IN ({$idsString})
                ", [$now]);

                // Collect updated balances for event dispatching
                $updatedBalances = array_merge($updatedBalances, $updates);
            }

            // Dispatch events for updated balances
            foreach ($updatedBalances as $update) {
                event(new BalanceUpdated(
                    $update['user_id'],
                    $update['old_amount'],
                    $update['new_amount'],
                    $update['new_version'],
                    $now
                ));
            }

            $this->info("Successfully updated " . count($updatedBalances) . " balances.");
            Log::info('Balances updated', [
                'count' => count($updatedBalances),
                'timestamp' => $now->toIso8601String(),
            ]);

            return Command::SUCCESS;
        } catch (\Exception $e) {
            $this->error("Error updating balances: " . $e->getMessage());
            Log::error('Failed to update balances', [
                'error' => $e->getMessage(),
                'trace' => $e->getTraceAsString(),
            ]);

            return Command::FAILURE;
        }
    }
}
