<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Run the migrations.
     */
    public function up(): void
    {
        Schema::create('balances', function (Blueprint $table) {
            $table->id();
            $table->foreignId('user_id')->constrained('users')->onDelete('cascade');
            $table->decimal('amount', 15, 2)->default(0);
            $table->unsignedInteger('version')->default(1);
            $table->timestamp('updated_at')->useCurrent()->useCurrentOnUpdate();
            
            // Індекси для оптимізації запитів
            $table->index('user_id');
            $table->index('updated_at');
            $table->unique('user_id'); // Один баланс на користувача
        });
    }

    /**
     * Reverse the migrations.
     */
    public function down(): void
    {
        Schema::dropIfExists('balances');
    }
};
