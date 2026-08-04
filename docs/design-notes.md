# Design notes: how a query becomes rows

Most of what makes a query engine interesting is invisible from the outside. The
SQL surface here is small on purpose — the point was never to support a lot of
syntax, it was to build the machinery that turns text into rows and to be able
to defend every part of it. These are the decisions worth writing down, and the
ones I would want to be asked about.

## Operators pull, they do not push

Execution is a tree of operators, and the obvious way to run one is bottom-up:
scan the table into a slice, filter the slice, sort it, take the first five.
That works and it is easy to reason about. It also reads a million rows to
answer `LIMIT 5`.

Instead every operator implements one method:

```go
type Operator interface {
	Schema() Schema
	Next() (value.Row, bool)
}
```

`Next()` returns one row and pulls whatever it needs from its child. `Limit`
counts to five and then returns `false`; `Filter` above the scan stops being
asked for rows; the scan stops reading. No operator knows that `LIMIT` exists,
and nobody had to write the optimization — it falls out of the shape. This is
the volcano model, and the property that makes it worth the indirection is
exactly this: **the consumer controls how much work the producer does.**

The cost is that some operators cannot be lazy. `Sort` has to see every row
before it can emit the first one, so it materializes. So does the build side of
a join. Being explicit about which operators block and which stream is most of
what it means to understand an execution plan.

## Hash join, and why the obvious join is the wrong one

A nested-loop join is four lines: for each row on the left, walk the right side
looking for matches. It is also O(n·m), and the constant is a full re-scan of
one input per row of the other.

The hash join splits the work into two phases. Drain the right input once,
hashing each row by its join key into a table. Then stream the left input,
hashing each row's key and probing. One pass over each input, O(n+m). The price
is memory: the entire build side is resident while probing. That is the real
trade, and it is why the build side should be the smaller input — a choice a
query optimizer would make and this engine does not, since it always builds from
the right.

Two details that are easy to get wrong and that the tests pin down:

**Duplicate keys.** A key maps to a *slice* of rows, not one row. If two orders
belong to the same user, probing that user must emit both. Getting this wrong
produces a join that silently drops rows, which is the kind of bug that survives
casual testing because the result still looks like a plausible table.

**NULL keys never match.** A row whose join key is NULL is dropped during build
and skipped during probe. This is not an optimization, it is the semantics:
`NULL = NULL` is unknown, not true, so a NULL key matches nothing — including
another NULL. The hash table would happily group them together if you let it,
which is exactly why it is worth being deliberate.

## Three-valued logic, and the one place SQL breaks its own rule

NULL is not a value, it is the absence of one, so any comparison with it answers
"unknown" rather than true or false. That gives three truth values, and the
connectives follow from it: `false AND unknown` is `false` (nothing can rescue
it), while `true AND unknown` is `unknown`. `NOT unknown` is still `unknown` —
negating ignorance does not produce knowledge.

The consequence that surprises people is what `WHERE` does with it. A predicate
that evaluates to unknown does not pass:

```go
if !v.IsNull() && v.Type == value.TBool && v.B {
	return row, true
}
```

So `WHERE age > 10` and `WHERE NOT (age > 10)` both exclude a row with a NULL
age. Between them they do not cover the table, which looks like a bug until you
remember that neither statement is true when the age is unknown.

Then `GROUP BY` does the opposite. Grouping puts every NULL city in a single
group, even though `NULL = NULL` is not true. SQL treats NULL as a value here
and as an unknown everywhere else, and the reason is pragmatic rather than
principled: a `GROUP BY` that scattered each NULL into its own group would be
useless. The engine implements both behaviours because both are correct — they
are just correct about different things. `internal/plan/null_test.go` states all
of it as executable specification, because this is the part of SQL where "I
think it works" is not good enough.

## Resolving names before reading rows

The planner walks the AST and turns every column reference into an index into
the row, checking types as it goes. An unknown column, an ambiguous bare name
across two joined tables, or a comparison between text and a number all fail
here — before the first row is read.

This buys two things. Errors are reported against the query rather than
appearing halfway through a scan, which is the difference between "unknown
column `bogus`" and a partial result followed by a failure. And operators become
simpler: they index into a row instead of looking up names, so the hot path has
no map lookups and no error handling for something the planner already proved
cannot happen.

## Testing against something that already knows the answer

Golden tests only find the bugs you thought of. You write the query, you work
out the expected rows, and you assert them — so the test encodes the same
understanding that produced the code, including its mistakes.

An oracle breaks that loop. The same data is loaded into SQLite, a generator
emits queries from a bounded grammar, both engines run them, and the results
have to match. SQLite has been correct for two decades; when it disagrees with
this engine, this engine is almost certainly wrong. Around 35,000 generated
queries have gone through this without a disagreement.

Two things make it honest rather than decorative.

The generator has to reach the hard parts. My first version only queried a
single table, which meant the hash join — the most intricate operator here —
never appeared in a single comparison. "Verified against SQLite" would have been
technically true and substantially misleading. The runner now asserts that joins
are a real share of what gets checked, so that particular self-deception cannot
come back quietly.

And when a disagreement does turn up, there are exactly two honest responses:
the engine is wrong and gets fixed, or the divergence is deliberate and gets
written down in the README. Widening the numeric tolerance until the failure
disappears is a third option that is always available and always wrong. The
comparison does round to six significant digits, which absorbs the last-bit
noise between two different ways of computing an average — but the cells carry
type tags, so the text `35` never compares equal to the number 35, and NULL uses
a sentinel no generated literal can produce. Where the two engines genuinely
disagree by design — integer division, cross-type comparison, booleans — the
generator does not emit the construct at all, so a real bug cannot hide behind a
known one.
