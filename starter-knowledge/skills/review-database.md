# review-database

Loaded at review for every track; apply it when the diff touches migrations, schema, SQL or ORM queries. Skip it otherwise.

Tools if present: `squawk` on Postgres migrations, `sqlfluff lint`, `EXPLAIN` (or `EXPLAIN ANALYZE` on a copy) for new or changed queries on large tables.

- **Locks on live tables.** Adding a column with a volatile default, changing a type, adding `NOT NULL` or a foreign key without `NOT VALID`, or creating an index without `CONCURRENTLY` (Postgres) can lock a busy table. Split into safe steps.
- **Deploy order.** Code and schema compatible in both orders: add column, deploy code that writes it, backfill in batches, then constrain; drop only after no code reads it.
- **Indexes.** New `WHERE`, `JOIN` and `ORDER BY` columns on large tables are indexed; no duplicate or unused new index.
- **N+1 and volume.** No query per row in a loop. Unbounded `SELECT` without `LIMIT` or pagination on growing tables.
- **Transactions.** Multi-step writes that must succeed together are in one transaction; no network call held inside one. Isolation and `SELECT ... FOR UPDATE` where concurrent writers race.
- **Constraints.** Integrity enforced in the DB (unique, foreign key, check) where the app assumes it.
- **Types.** Money as `numeric`/integer cents, timestamps with time zone, text length limits deliberate.

Sources: [PostgreSQL explicit locking](https://www.postgresql.org/docs/current/explicit-locking.html), [strong_migrations](https://github.com/ankane/strong_migrations), [Squawk rules](https://squawkhq.com/docs/rules), [Use The Index, Luke](https://use-the-index-luke.com/).
