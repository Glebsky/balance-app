<?php

use App\Console\Commands\UpdateBalancesCommand;
use Illuminate\Foundation\Inspiring;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\Schedule;

Artisan::command('inspire', function () {
    $this->comment(Inspiring::quote());
})->purpose('Display an inspiring quote');

// Schedule balance updates every 10 seconds
Schedule::command(UpdateBalancesCommand::class)
    ->everyTenSeconds()
    ->runInBackground();
