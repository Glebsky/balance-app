<?php

return [
    'max_deadlock_retries' => env('BALANCE_DEADLOCK_RETRIES', 5),
    'batch_size' => env('BALANCE_BATCH_SIZE', 50),
    'user_count' => env('BALANCE_USER_COUNT', 50),
];
