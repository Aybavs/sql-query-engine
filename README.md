# sql-query-engine

A small SQL query engine in Go. It lexes and parses a `SELECT` subset, plans it,
and executes it with a volcano (pull-based) operator model including a real hash
join. Tables are CSV files described by a small schema; you query them from a
REPL.

**What it is:** a readable, tested implementation of how a query gets from text
to rows — parsing, planning, and execution — with its results checked against
SQLite on tens of thousands of generated queries.

**What it is not:** a database. There is no storage engine, no persistence, no
transactions, no indexes, and no query optimizer. See [Non-goals](#non-goals).

## Features

- **Volcano execution model** — every operator pulls rows from its child through
  `Next()`, so `LIMIT` stops the scan early without any operator knowing that
  `LIMIT` exists.
- **Hash join** — `INNER JOIN` runs build/probe over a hash table rather than a
  nested loop, and handles duplicate and NULL keys correctly.
- **Three-valued NULL logic** — comparisons involving NULL are unknown, and
  `WHERE` keeps only rows that are exactly true.
- **Aggregates** — `COUNT`, `SUM`, `AVG`, `MIN`, `MAX` with `GROUP BY` and
  `HAVING`.
- **Plan-time type checking** — unknown columns, ambiguous references, and type
  mismatches are rejected before a single row is read.
- **Differential testing against SQLite** — a seeded generator produces queries
  and both engines must agree.

## Quick start

```bash
go run ./cmd/minisql -data examples
```

The bundled `examples/` directory holds this schema:

```
users(id INT, name TEXT, age INT, city TEXT)
orders(id INT, user_id INT, total INT)
```

backed by `users.csv` and `orders.csv`:

```csv
1,alice,30,berlin
2,bob,15,paris
3,carol,40,berlin
4,dan,,london
```

A session (output is verbatim):

```
minisql — one SQL statement per line (Ctrl-D to exit)
SELECT name, age FROM users WHERE age >= 18 ORDER BY age DESC
name  | age
------+----
carol | 40
alice | 30
(2 rows)

SELECT city, COUNT(id), AVG(age) FROM users GROUP BY city HAVING COUNT(id) > 1
city   | COUNT(id) | AVG(age)
-------+-----------+---------
berlin | 2         | 35
(1 row)

SELECT users.name, orders.total FROM users JOIN orders ON users.id = orders.user_id ORDER BY orders.total DESC
name  | total
------+------
alice | 300
carol | 250
alice | 100
(3 rows)

SELECT name, age FROM users WHERE age IS NULL
name | age
-----+-----
dan  | NULL
(1 row)

SELECT bogus FROM users
error: unknown column "bogus"
```

Note what the join left out: order 13 has an empty `user_id`, and a NULL key
never matches, so it appears in no result row.

## Supported SQL

```
SELECT <* | expr [, expr ...]>
FROM <table>
[[INNER] JOIN <table> ON <column> = <column>]
[WHERE <condition>]
[GROUP BY <column> [HAVING <condition>]]
[ORDER BY <expr> [ASC | DESC] [, ...]]
[LIMIT <n>]
```

Expressions support column references (`name`, `users.name`), literals,
`= <> < <= > >=`, `+ - * /`, `AND OR NOT`, `IS [NOT] NULL`, and the aggregate
functions `COUNT`, `SUM`, `AVG`, `MIN`, `MAX` (including `COUNT(*)`).

A projection that is not a bare column is labelled with its source expression,
so `SELECT COUNT(id)` reports a column named `COUNT(id)`.

## Data format

A schema file declares one table per line as `name(col TYPE, ...)` with types
`INT`, `FLOAT`, `TEXT`, or `BOOL`. Each table reads `<name>.csv` from the same
directory. CSV files have **no header row**, and an **empty field is NULL** —
which is how the examples above get a NULL age and a NULL join key.

## Architecture

```
SQL text
   │  lexer            tokens
   ▼
 parser  ─────────────▶ AST
   │
   ▼  planner (resolve names, check types against the catalog)
operator tree
   │
   ▼  executor (volcano: each Next() pulls from its child)
 rows ─▶ REPL prints a table
```

The planner resolves every column reference to an index and type before
execution, so operators work with positions rather than names and cannot fail on
an unknown column mid-scan.

The reasoning behind the execution model, the hash join, the NULL semantics, and
oracle-based testing is written up in
[docs/design-notes.md](docs/design-notes.md).

```
cmd/minisql/        entrypoint
internal/lexer/     tokenizer
internal/parser/    recursive-descent parser
internal/ast/       syntax tree and its rendering
internal/catalog/   table schemas
internal/csv/       schema-aware CSV reader
internal/plan/      AST → operator tree, name and type resolution
internal/exec/      operators (scan, filter, project, hash join, sort, limit, aggregate)
internal/value/     typed values and three-valued logic
internal/repl/      read-eval-print loop
internal/difftest/  test-only: SQLite oracle, query generator, differential runner
```

## Testing

```bash
go test -race ./...
```

Three layers:

- **Golden tests** per package — a query and fixture in, exact rows out.
- **A NULL semantics suite** (`internal/plan/null_test.go`) stating three-valued
  behaviour explicitly: unknown comparisons excluded by `WHERE`, `NOT unknown`
  still excluded, NULL join keys dropped, aggregates skipping NULLs, and
  `GROUP BY` collapsing NULL keys into one group.
- **Differential tests against SQLite** — a seeded generator emits queries from
  a bounded grammar, both engines run them over identical data, and the results
  must match as multisets.

Run a wider sweep or replay a specific run:

```bash
go test ./internal/difftest/ -run Differential -seed 31337 -queries 5000
```

Every failure prints the exact query and seed. The engine has been checked
across seeds 1, 2, 3, 99, 777, 12345, and 31337 at 5,000 queries each — 35,000
comparisons with no disagreements. Roughly a quarter of the generated queries
are joins, so the hash join is genuinely covered rather than incidentally
touched.

Comparison ignores row order, because SQL leaves it unspecified without
`ORDER BY`, but duplicate counts still have to match. `ORDER BY` correctness is
covered by the deterministic golden tests instead.

`modernc.org/sqlite` is pure Go and is imported only by `internal/difftest`,
which nothing outside its own tests imports, so the engine binary carries no
SQLite driver:

```bash
go list -deps ./cmd/minisql | grep -c sqlite   # 0
```

## Known divergences from SQLite

These are deliberate design choices, not bugs, and the query generator avoids
them so they cannot mask a real disagreement:

| Area | SQLite | This engine |
|---|---|---|
| `/` on two integers | integer division (`5/2` = 2) | float division (2.5) |
| comparing a text column to a number | coerced dynamically | rejected at plan time |
| booleans | stored as integers 0/1 | a distinct `BOOL` type |

## Non-goals

No storage engine or persistence, no transactions, no indexes, no query
optimizer, no subqueries, no `OUTER`/`CROSS` joins, no `DISTINCT`, no window
functions, and no DDL or DML. The engine reads CSV files and answers `SELECT`
queries.

## Requirements

Go 1.25 or newer. The engine itself uses only the standard library; the SQLite
driver used by the differential tests is what sets the Go version floor.

## License

MIT
