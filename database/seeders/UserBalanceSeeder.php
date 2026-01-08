<?php

namespace Database\Seeders;

use App\Models\Balance;
use App\Models\User;
use Illuminate\Database\Console\Seeds\WithoutModelEvents;
use Illuminate\Database\Seeder;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Hash;
use Illuminate\Support\Str;

class UserBalanceSeeder extends Seeder
{
    use WithoutModelEvents;

    /**
     * Run the database seeds.
     */
    public function run(): void
    {
        $usersCount = 1000;
        $batchSize = 100;
        
        $this->command->info("Generating {$usersCount} users with balances...");

        for ($i = 0; $i < $usersCount; $i += $batchSize) {
            $users = [];
            $balances = [];
            $now = now();

            for ($j = 0; $j < $batchSize && ($i + $j) < $usersCount; $j++) {
                $users[] = [
                    'name' => fake()->name(),
                    'email' => fake()->unique()->safeEmail(),
                    'email_verified_at' => $now,
                    'password' => Hash::make('password'),
                    'remember_token' => Str::random(10),
                    'created_at' => $now,
                    'updated_at' => $now,
                ];
            }

            // Bulk insert users
            DB::table('users')->insert($users);
            
            // Get inserted user IDs
            $insertedUsers = DB::table('users')
                ->whereIn('email', array_column($users, 'email'))
                ->pluck('id', 'email')
                ->toArray();

            // Prepare balances for bulk insert
            foreach ($users as $user) {
                $userId = $insertedUsers[$user['email']] ?? null;
                if ($userId) {
                    $balances[] = [
                        'user_id' => $userId,
                        'amount' => fake()->randomFloat(2, 0, 10000),
                        'version' => 1,
                        'created_at' => $now,
                        'updated_at' => $now,
                    ];
                }
            }

            // Bulk insert balances
            if (!empty($balances)) {
                DB::table('balances')->insert($balances);
            }

            $this->command->info("Processed " . min($i + $batchSize, $usersCount) . " / {$usersCount} users");
        }

        $this->command->info("Successfully generated {$usersCount} users with balances!");
    }
}
