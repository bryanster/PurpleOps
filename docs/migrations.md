# Database migrations

The schema lives in `internal/store/migrate/sql/` as ordered `.sql` files. They are compiled into
the binary, so there is no migration tool to install and no step to forget: the server applies any
it has not run yet at startup, before it accepts a request.

Bookkeeping is a single table, `schema_migrations`, in the database's default schema:

| Column | Meaning |
|---|---|
| `version` | The file's number. Primary key, so a migration cannot be applied twice |
| `name` | The descriptive part of the filename |
| `checksum` | SHA-256 of the file as it was when it ran |
| `applied_at` | UTC |

It is not in `app` or `content` because migration `0001` is what creates those schemas.

## Seeing where a database is

`blctl` reports the same bookkeeping without starting a server ([`docs/cli.md`](cli.md)):

```sh
blctl migrate status          # every migration, applied or pending, with timestamps
blctl migrate up              # apply the pending ones and stop
blctl db info                 # schema version, file size, row counts
```

The server still migrates at startup, so this is for the deployment where you want the schema
change to happen before the new binary serves anything — and for finding out what a release is
about to do. It needs the server stopped: one process holds the database, which is the same rule as
[Two servers, one database](#two-servers-one-database) below.

## Adding a migration

1. Create `internal/store/migrate/sql/NNNN_lower_snake_case.sql`, where `NNNN` is the next version,
   zero-padded to four digits. Versions run from `0001` upwards with no gaps and no duplicates.
2. Write portable ANSI SQL. DuckDB-specific syntax belongs behind the store, not in the schema —
   the escape hatch in `PLAN.md` §1 depends on it.
3. Do **not** write `BEGIN`, `COMMIT` or `ROLLBACK`. The migrator wraps each file in one transaction
   of its own; a file that opens a second one fails.
4. Nothing else goes in that directory. A stray `README.md` or `0004_wip.sql.bak` is a startup
   error, not a file that is quietly skipped.

Tests catch a malformed set before it reaches CI. Run `go test ./internal/store/migrate/...`.

### Two DuckDB limitations to design around

**Do not give a table a foreign key pointing at rows you will need to update.** DuckDB runs an
`UPDATE` on an indexed table as a delete followed by an insert, and the delete half runs the
referential check — so a *referenced* row cannot be updated at all while anything points at it, not
even a column the key has nothing to do with. `0002_identity` learned this the expensive way: with
`app.session.user_id REFERENCES app."user"(id)`, recording a login was a constraint error for every
account that had ever signed in. `0003_user_updatable` removed those keys and moved the rule into
`requireUser` in the repositories. Verified against DuckDB v1.5.5; check whether it still holds
before reaching for a foreign key.

**There is no `ALTER TABLE ... DROP CONSTRAINT`** — "No support for that ALTER TABLE option yet!".
Removing a constraint means creating the table again beside the old one, copying the rows across,
dropping the old one and renaming, with every index recreated afterwards. `0003_user_updatable.sql`
is the worked example.

## Migrations are append-only

Once a migration is merged, it is never edited, renamed or deleted. Every deployment that already
ran it would otherwise hold a schema that no file in the tree describes.

This is enforced. The server refuses to start, and says which file, if:

| It finds | What happened | What to do |
|---|---|---|
| A recorded migration whose file has changed | Someone edited a merged migration | `git revert` the edit, and add a new migration with the change |
| A recorded migration under a different name | Someone renamed one | Restore the old filename |
| A recorded migration this binary does not contain | An older build was deployed over a newer database | Deploy a build that includes it, or restore a backup taken before it ran |

Line endings are not part of the checksum, so a checkout with `core.autocrlf` set does not trip this.

## There are no down migrations

Recovery from a bad migration is **restore from backup**, not a reverse script.

A down migration is written before anyone needs it, run once under pressure, and is the only SQL in
the tree that has never been tested against real data. When it matters it either fails or silently
destroys the rows it was supposed to save.

So take a backup before deploying a release that migrates:

```sh
# The database is a single file; stop the server before copying it.
systemctl stop blacklight
cp /var/lib/blacklight/blacklight.duckdb /var/backups/blacklight-$(date -u +%Y%m%dT%H%M%SZ).duckdb
systemctl start blacklight
```

To recover, stop the server, put the backup back, and start the previous release.

## When a migration fails

Each file is one transaction, so a failure leaves that file's changes entirely undone and its row
absent from `schema_migrations`. Migrations after it do not run. The error names the file and
repeats what the database said about it. Fix the SQL and start the server again — everything before
the failure stays applied and is not re-run.

## Two servers, one database

DuckDB allows one read-write process per file. A second server started against the same database
fails to open it at all, with `IO Error: Could not set lock on file …`, before it can migrate
anything. That is the intended outcome, not a bug to work around: during a deployment, the old
process must exit before the new one starts.
