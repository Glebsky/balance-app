<?php

namespace Database\Seeders;

use App\Models\User;
use Illuminate\Database\Seeder;
use Illuminate\Database\Console\Seeds\WithoutModelEvents;

class UserBalanceSeeder extends Seeder
{
    use WithoutModelEvents;

    public function run(): void
    {
        $total = 1000;
        $this->command?->info("Generating {$total} users with balances...");

        User::factory()
            ->count($total)
            ->hasBalance(1)
            ->create();

        $this->command?->info("Successfully seeded {$total} users and their balances.");
    }
}
