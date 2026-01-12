<?php

namespace App\Console\Commands;

use App\Services\BalanceUpdaterService;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\Log;

class UpdateBalancesCommand extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'balances:update {--users-count=50 : Number of random users to select (default: 50)}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Update random user balances in bulk with optimistic versioning';

    public function __construct(private readonly BalanceUpdaterService $updater)
    {
        parent::__construct();
    }

    /**
     * Execute the console command.
     */
    public function handle(): int
    {
        $usersCount = max(1, (int) $this->option('users-count'));
        $this->info("Updating random balances in batches of {$usersCount}...");

        try {
            $updated = $this->updater->updateRandomGroup($usersCount);
            $this->info("Updated {$updated} balances");

            return Command::SUCCESS;
        } catch (\Throwable $e) {
            Log::error('balances:update failed', [
                'error' => $e->getMessage(),
            ]);

            $this->error($e->getMessage());

            return Command::FAILURE;
        }
    }
}
