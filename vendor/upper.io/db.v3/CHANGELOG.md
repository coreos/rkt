## Changelog

Dec 15th, 2016: On `db.v2`, upper-db produced queries that mutated themselves:

```
q := sess.SelectFrom("users")

q.Where(...) // This method modified q's internal state.
```

Starting on `db.v3` this is no longer valid, if you want to use values to
represent queries you'll have to reassign them, like this:

```
q := sess.SelectFrom("users")

q = q.Where(...)

q.And(...) // Nothing happens, the Where() method does not affect q.
```

This applies to all query builder methods, `db.Result`, `db.And` and `db.Or`.

If you want to check your code for statatements that might rely on the old
behaviour and could cause you trouble use `dbcheck`:

```
go get -u github.com/upper/cmd/dbcheck

dbcheck github.com/my/package/...
```
