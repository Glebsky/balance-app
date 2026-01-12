<?php

namespace Database\Factories;

use App\Models\Balance;
use Illuminate\Database\Eloquent\Factories\Factory;

class BalanceFactory extends Factory
{
    protected $model = Balance::class;

    public function definition(): array
    {
        return [
            'amount'     => fake()->randomFloat(2, 0, 10000),
            'version'    => 0,
            'created_at' => now(),
            'updated_at' => now(),
        ];
    }
}
