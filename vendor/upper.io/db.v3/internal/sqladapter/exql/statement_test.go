package exql

import (
	"regexp"
	"strings"
	"testing"
)

var (
	reInvisible = regexp.MustCompile(`[\t\n\r]`)
	reSpace     = regexp.MustCompile(`\s+`)
)

func mustTrim(a string, err error) string {
	if err != nil {
		panic(err.Error())
	}
	a = reInvisible.ReplaceAllString(strings.TrimSpace(a), " ")
	a = reSpace.ReplaceAllString(strings.TrimSpace(a), " ")
	return a
}

func TestTruncateTable(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Truncate,
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `TRUNCATE TABLE "table_name"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestDropTable(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  DropTable,
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `DROP TABLE "table_name"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestDropDatabase(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:     DropDatabase,
		Database: &Database{Name: "table_name"},
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `DROP DATABASE "table_name"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestCount(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Count,
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT COUNT(1) AS _t FROM "table_name"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestCountRelation(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Count,
		Table: TableWithName("information_schema.tables"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT COUNT(1) AS _t FROM "information_schema"."tables"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestCountWhere(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Count,
		Table: TableWithName("table_name"),
		Where: WhereConditions(
			&ColumnValue{Column: &Column{Name: "a"}, Operator: "=", Value: NewValue(RawValue("7"))},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT COUNT(1) AS _t FROM "table_name" WHERE ("a" = 7)`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectStarFrom(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Select,
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT * FROM "table_name"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectStarFromAlias(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Select,
		Table: TableWithName("table.name AS foo"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT * FROM "table"."name" AS "foo"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectStarFromRawWhere(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Select,
		Table: TableWithName("table.name AS foo"),
		Where: WhereConditions(
			&Raw{Value: "foo.id = bar.foo_id"},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT * FROM "table"."name" AS "foo" WHERE (foo.id = bar.foo_id)`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	stmt = Statement{
		Type:  Select,
		Table: TableWithName("table.name AS foo"),
		Where: WhereConditions(
			&Raw{Value: "foo.id = bar.foo_id"},
			&Raw{Value: "baz.id = exp.baz_id"},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT * FROM "table"."name" AS "foo" WHERE (foo.id = bar.foo_id AND baz.id = exp.baz_id)`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectStarFromMany(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Select,
		Table: TableWithName("first.table AS foo, second.table as BAR, third.table aS baz"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT * FROM "first"."table" AS "foo", "second"."table" AS "BAR", "third"."table" AS "baz"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectTableStarFromMany(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo.name"},
			&Column{Name: "BAR.*"},
			&Column{Name: "baz.last_name"},
		),
		Table: TableWithName("first.table AS foo, second.table as BAR, third.table aS baz"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo"."name", "BAR".*, "baz"."last_name" FROM "first"."table" AS "foo", "second"."table" AS "BAR", "third"."table" AS "baz"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectArtistNameFrom(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Select,
		Table: TableWithName("artist"),
		Columns: JoinColumns(
			&Column{Name: "artist.name"},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "artist"."name" FROM "artist"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectJoin(t *testing.T) {
	var s, e string

	stmt := Statement{
		Type:  Select,
		Table: TableWithName("artist a"),
		Columns: JoinColumns(
			&Column{Name: "a.name"},
		),
		Joins: JoinConditions(&Join{
			Table: TableWithName("books b"),
			On: OnConditions(
				&ColumnValue{
					Column:   ColumnWithName("b.author_id"),
					Operator: `=`,
					Value:    NewValue(ColumnWithName("a.id")),
				},
			),
		}),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "a"."name" FROM "artist" AS "a" JOIN "books" AS "b" ON ("b"."author_id" = "a"."id")`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectJoinUsing(t *testing.T) {
	var s, e string

	stmt := Statement{
		Type:  Select,
		Table: TableWithName("artist a"),
		Columns: JoinColumns(
			&Column{Name: "a.name"},
		),
		Joins: JoinConditions(&Join{
			Table: TableWithName("books b"),
			Using: UsingColumns(
				ColumnWithName("artist_id"),
				ColumnWithName("country"),
			),
		}),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "a"."name" FROM "artist" AS "a" JOIN "books" AS "b" USING ("artist_id", "country")`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectUnfinishedJoin(t *testing.T) {
	stmt := Statement{
		Type:  Select,
		Table: TableWithName("artist a"),
		Columns: JoinColumns(
			&Column{Name: "a.name"},
		),
		Joins: JoinConditions(&Join{}),
	}

	s := mustTrim(stmt.Compile(defaultTemplate))
	e := `SELECT "a"."name" FROM "artist" AS "a"`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectNaturalJoin(t *testing.T) {
	var s, e string

	stmt := Statement{
		Type:  Select,
		Table: TableWithName("artist"),
		Joins: JoinConditions(&Join{
			Table: TableWithName("books"),
		}),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT * FROM "artist" NATURAL JOIN "books"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectRawFrom(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Select,
		Table: TableWithName(`artist`),
		Columns: JoinColumns(
			&Column{Name: `artist.name`},
			&Column{Name: Raw{Value: `CONCAT(artist.name, " ", artist.last_name)`}},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "artist"."name", CONCAT(artist.name, " ", artist.last_name) FROM "artist"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectFieldsFrom(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectFieldsFromWithLimitOffset(t *testing.T) {
	var s, e string
	var stmt Statement

	// LIMIT only.
	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Limit: 42,
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" LIMIT 42`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	// OFFSET only.
	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Offset: 17,
		Table:  TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" OFFSET 17`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	// LIMIT AND OFFSET.
	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Limit:  42,
		Offset: 17,
		Table:  TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" LIMIT 42 OFFSET 17`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestStatementGroupBy(t *testing.T) {
	var s, e string
	var stmt Statement

	// Simple GROUP BY
	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		GroupBy: GroupByColumns(
			&Column{Name: "foo"},
		),
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" GROUP BY "foo"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		GroupBy: GroupByColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
		),
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" GROUP BY "foo", "bar"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectFieldsFromWithOrderBy(t *testing.T) {
	var s, e string
	var stmt Statement

	// Simple ORDER BY
	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		OrderBy: JoinWithOrderBy(
			JoinSortColumns(
				&SortColumn{Column: &Column{Name: "foo"}},
			),
		),
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" ORDER BY "foo"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	// ORDER BY field ASC
	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		OrderBy: JoinWithOrderBy(
			JoinSortColumns(
				&SortColumn{Column: &Column{Name: "foo"}, Order: Ascendent},
			),
		),
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" ORDER BY "foo" ASC`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	// ORDER BY field DESC
	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		OrderBy: JoinWithOrderBy(
			JoinSortColumns(
				&SortColumn{Column: &Column{Name: "foo"}, Order: Descendent},
			),
		),
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" ORDER BY "foo" DESC`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	// ORDER BY many fields
	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		OrderBy: JoinWithOrderBy(
			JoinSortColumns(
				&SortColumn{Column: &Column{Name: "foo"}, Order: Descendent},
				&SortColumn{Column: &Column{Name: "bar"}, Order: Ascendent},
				&SortColumn{Column: &Column{Name: "baz"}, Order: Descendent},
			),
		),
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" ORDER BY "foo" DESC, "bar" ASC, "baz" DESC`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	// ORDER BY function
	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		OrderBy: JoinWithOrderBy(
			JoinSortColumns(
				&SortColumn{Column: &Column{Name: Raw{Value: "FOO()"}}, Order: Descendent},
				&SortColumn{Column: &Column{Name: Raw{Value: "BAR()"}}, Order: Ascendent},
			),
		),
		Table: TableWithName("table_name"),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" ORDER BY FOO() DESC, BAR() ASC`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectFieldsFromWhere(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Table: TableWithName("table_name"),
		Where: WhereConditions(
			&ColumnValue{Column: &Column{Name: "baz"}, Operator: "=", Value: NewValue(99)},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" WHERE ("baz" = '99')`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestSelectFieldsFromWhereLimitOffset(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type: Select,
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Table: TableWithName("table_name"),
		Where: WhereConditions(
			&ColumnValue{Column: &Column{Name: "baz"}, Operator: "=", Value: NewValue(99)},
		),
		Limit:  10,
		Offset: 23,
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `SELECT "foo", "bar", "baz" FROM "table_name" WHERE ("baz" = '99') LIMIT 10 OFFSET 23`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestDelete(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Delete,
		Table: TableWithName("table_name"),
		Where: WhereConditions(
			&ColumnValue{Column: &Column{Name: "baz"}, Operator: "=", Value: NewValue(99)},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `DELETE FROM "table_name" WHERE ("baz" = '99')`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestUpdate(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Update,
		Table: TableWithName("table_name"),
		ColumnValues: JoinColumnValues(
			&ColumnValue{Column: &Column{Name: "foo"}, Operator: "=", Value: NewValue(76)},
		),
		Where: WhereConditions(
			&ColumnValue{Column: &Column{Name: "baz"}, Operator: "=", Value: NewValue(99)},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `UPDATE "table_name" SET "foo" = '76' WHERE ("baz" = '99')`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	stmt = Statement{
		Type:  Update,
		Table: TableWithName("table_name"),
		ColumnValues: JoinColumnValues(
			&ColumnValue{Column: &Column{Name: "foo"}, Operator: "=", Value: NewValue(76)},
			&ColumnValue{Column: &Column{Name: "bar"}, Operator: "=", Value: NewValue(Raw{Value: "88"})},
		),
		Where: WhereConditions(
			&ColumnValue{Column: &Column{Name: "baz"}, Operator: "=", Value: NewValue(99)},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `UPDATE "table_name" SET "foo" = '76', "bar" = 88 WHERE ("baz" = '99')`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestInsert(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Insert,
		Table: TableWithName("table_name"),
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Values: NewValueGroup(
			&Value{V: "1"},
			&Value{V: 2},
			&Value{V: Raw{Value: "3"}},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `INSERT INTO "table_name" ("foo", "bar", "baz") VALUES ('1', '2', 3)`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestInsertMultiple(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Insert,
		Table: TableWithName("table_name"),
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Values: JoinValueGroups(
			NewValueGroup(
				NewValue("1"),
				NewValue("2"),
				NewValue(RawValue("3")),
			),
			NewValueGroup(
				NewValue(RawValue("4")),
				NewValue(RawValue("5")),
				NewValue(RawValue("6")),
			),
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `INSERT INTO "table_name" ("foo", "bar", "baz") VALUES ('1', '2', 3), (4, 5, 6)`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestInsertReturning(t *testing.T) {
	var s, e string
	var stmt Statement

	stmt = Statement{
		Type:  Insert,
		Table: TableWithName("table_name"),
		Returning: ReturningColumns(
			ColumnWithName("id"),
		),
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Values: NewValueGroup(
			&Value{V: "1"},
			&Value{V: 2},
			&Value{V: Raw{Value: "3"}},
		),
	}

	s = mustTrim(stmt.Compile(defaultTemplate))
	e = `INSERT INTO "table_name" ("foo", "bar", "baz") VALUES ('1', '2', 3) RETURNING "id"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestRawSQLStatement(t *testing.T) {
	stmt := RawSQL(`SELECT * FROM "foo" ORDER BY "bar"`)

	s := mustTrim(stmt.Compile(defaultTemplate))
	e := `SELECT * FROM "foo" ORDER BY "bar"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func BenchmarkStatementSimpleQuery(b *testing.B) {
	stmt := Statement{
		Type:  Count,
		Table: TableWithName("table_name"),
		Where: WhereConditions(
			&ColumnValue{Column: &Column{Name: "a"}, Operator: "=", Value: NewValue(Raw{Value: "7"})},
		),
	}

	for i := 0; i < b.N; i++ {
		_, _ = stmt.Compile(defaultTemplate)
	}
}

func BenchmarkStatementSimpleQueryHash(b *testing.B) {
	stmt := Statement{
		Type:  Count,
		Table: TableWithName("table_name"),
		Where: WhereConditions(
			&ColumnValue{Column: &Column{Name: "a"}, Operator: "=", Value: NewValue(Raw{Value: "7"})},
		),
	}

	for i := 0; i < b.N; i++ {
		_ = stmt.Hash()
	}
}

func BenchmarkStatementSimpleQueryNoCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		stmt := Statement{
			Type:  Count,
			Table: TableWithName("table_name"),
			Where: WhereConditions(
				&ColumnValue{Column: &Column{Name: "a"}, Operator: "=", Value: NewValue(Raw{Value: "7"})},
			),
		}
		_, _ = stmt.Compile(defaultTemplate)
	}
}

func BenchmarkStatementComplexQuery(b *testing.B) {
	stmt := Statement{
		Type:  Insert,
		Table: TableWithName("table_name"),
		Columns: JoinColumns(
			&Column{Name: "foo"},
			&Column{Name: "bar"},
			&Column{Name: "baz"},
		),
		Values: NewValueGroup(
			&Value{V: "1"},
			&Value{V: 2},
			&Value{V: Raw{Value: "3"}},
		),
	}

	for i := 0; i < b.N; i++ {
		_, _ = stmt.Compile(defaultTemplate)
	}
}

func BenchmarkStatementComplexQueryNoCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		stmt := Statement{
			Type:  Insert,
			Table: TableWithName("table_name"),
			Columns: JoinColumns(
				&Column{Name: "foo"},
				&Column{Name: "bar"},
				&Column{Name: "baz"},
			),
			Values: NewValueGroup(
				&Value{V: "1"},
				&Value{V: 2},
				&Value{V: Raw{Value: "3"}},
			),
		}
		_, _ = stmt.Compile(defaultTemplate)
	}
}
