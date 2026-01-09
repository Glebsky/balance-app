<?php

namespace Database\Seeders;

use App\Models\Balance;
use App\Models\User;
use Illuminate\Database\Console\Seeds\WithoutModelEvents;
use Illuminate\Database\Seeder;
use Illuminate\Support\Facades\DB;

class UserBalanceSeeder extends Seeder
{
    use WithoutModelEvents;

    /**
     * Run the database seeds.
     */
    public function run(): void
    {
        $total = 1000;
        $batch = 100;
        $this->command?->info("Generating {$total} users with balances...");

        DB::transaction(function () use ($total, $batch) {
            for ($offset = 0; $offset < $total; $offset += $batch) {
                $size = min($batch, $total - $offset);
                $users = User::factory()->count($size)->create();
                $now = now();

                $balances = $users->map(function (User $user) use ($now) {
                    return [
                        'user_id' => $user->id,
                        'amount' => fake()->randomFloat(2, 0, 10_000),
                        'version' => 0,
                        'created_at' => $now,
                        'updated_at' => $now,
                    ];
                });

                Balance::query()->insert($balances->toArray());
            }
        });

        $this->command?->info('Seeded 1000 users with balances.');
    }
}
