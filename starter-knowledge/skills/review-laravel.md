# review-laravel

Loaded at review when a repo in this track depends on `laravel/framework`. Run the tools first; review what they cannot see.

Tools: the repo's test suite (`php artisan test`, Pest or PHPUnit), `vendor/bin/phpstan` (Larastan) if configured, `vendor/bin/pint --test`, `composer audit`.

- **Mass assignment.** Models use `$fillable` (or the repo's guarded convention); `$request->all()` never passed to `create`/`update`. Use `validated()`.
- **Validation.** Every write route validates through a FormRequest or `validate()`. Rules cover type, size and existence, not just `required`.
- **Authorization.** Each controller action or Livewire method calls a Policy/Gate (`authorize`, `can` middleware). Route model binding scoped to the owner where it matters.
- **N+1.** Relations used in loops or views are eager-loaded (`with`, `load`). Consider `Model::preventLazyLoading()` in non-production.
- **Queries.** No raw `DB::raw`/`whereRaw` with interpolated input. Large tables use `chunk`/`lazy`, not `get()`.
- **Queues.** Jobs idempotent (retries happen), serializable, with `$tries`/`$timeout` set. Side effects after a transaction use `afterCommit`.
- **Migrations.** See review-database; `down()` works or is deliberately absent.
- **Config.** `env()` called only inside `config/` files (it returns null once config is cached).

Sources: [Laravel docs](https://laravel.com/docs), [Laravel Boost guidelines](https://github.com/laravel/boost), [Larastan](https://github.com/larastan/larastan).
