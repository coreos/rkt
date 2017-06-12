package sqlbuilder

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"upper.io/db.v3"
)

func TestSelect(t *testing.T) {

	b := &sqlBuilder{t: newTemplateWithUtils(&testTemplate)}
	assert := assert.New(t)

	assert.Equal(
		`SELECT DATE()`,
		b.Select(db.Func("DATE")).String(),
	)

	assert.Equal(
		`SELECT DATE() FOR UPDATE`,
		b.Select(db.Func("DATE")).Amend(func(query string) string {
			return query + " FOR UPDATE"
		}).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist"`,
		b.SelectFrom("artist").String(),
	)

	assert.Equal(
		`SELECT DISTINCT "bcolor" FROM "artist"`,
		b.Select().Distinct("bcolor").From("artist").String(),
	)

	assert.Equal(
		`SELECT DISTINCT * FROM "artist"`,
		b.Select().Distinct().From("artist").String(),
	)

	assert.Equal(
		`SELECT DISTINCT ON("col1"), "col2" FROM "artist"`,
		b.Select().Distinct(db.Raw(`ON("col1")`), "col2").From("artist").String(),
	)

	assert.Equal(
		`SELECT DISTINCT ON("col1") AS col1, "col2" FROM "artist"`,
		b.Select().Distinct(db.Raw(`ON("col1") AS col1`)).Distinct("col2").From("artist").String(),
	)

	assert.Equal(
		`SELECT DISTINCT ON("col1") AS col1, "col2", "col3", "col4", "col5" FROM "artist"`,
		b.Select().Distinct(db.Raw(`ON("col1") AS col1`)).Columns("col2", "col3").Distinct("col4", "col5").From("artist").String(),
	)

	assert.Equal(
		`SELECT DISTINCT ON(SELECT foo FROM bar) col1, "col2", "col3", "col4", "col5" FROM "artist"`,
		b.Select().Distinct(db.Raw(`ON(?) col1`, db.Raw(`SELECT foo FROM bar`))).Columns("col2", "col3").Distinct("col4", "col5").From("artist").String(),
	)

	assert.Equal(
		`SELECT DISTINCT ON (SELECT foo FROM bar, SELECT baz from qux) col1, "col2", "col3", "col4", "col5" FROM "artist"`,
		b.Select().Distinct(db.Raw(`ON ? col1`, []interface{}{db.Raw(`SELECT foo FROM bar`), db.Raw(`SELECT baz from qux`)})).Columns("col2", "col3").Distinct("col4", "col5").From("artist").String(),
	)

	{
		q := b.Select().Distinct(db.Raw(`ON (?) col1`, []db.RawValue{db.Raw(`SELECT foo FROM bar WHERE id = ?`, 1), db.Raw(`SELECT baz from qux WHERE id = 2`)})).Columns("col2", "col3").Distinct("col4", "col5").From("artist").
			Where("id", 3)
		assert.Equal(
			`SELECT DISTINCT ON ((SELECT foo FROM bar WHERE id = $1, SELECT baz from qux WHERE id = 2)) col1, "col2", "col3", "col4", "col5" FROM "artist" WHERE ("id" = $2)`,
			q.String(),
		)

		assert.Equal(
			[]interface{}{1, 3},
			q.Arguments(),
		)
	}

	{
		rawCase := db.Raw("CASE WHEN id IN ? THEN 0 ELSE 1 END", []int{1000, 2000})
		sel := b.SelectFrom("artist").OrderBy(rawCase)
		assert.Equal(
			`SELECT * FROM "artist" ORDER BY CASE WHEN id IN ($1, $2) THEN 0 ELSE 1 END`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1000, 2000},
			sel.Arguments(),
		)
	}

	{
		rawCase := db.Raw("CASE WHEN id IN ? THEN 0 ELSE 1 END", []int{1000})
		sel := b.SelectFrom("artist").OrderBy(rawCase)
		assert.Equal(
			`SELECT * FROM "artist" ORDER BY CASE WHEN id IN ($1) THEN 0 ELSE 1 END`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1000},
			sel.Arguments(),
		)
	}

	{
		rawCase := db.Raw("CASE WHEN id IN ? THEN 0 ELSE 1 END", []int{})
		sel := b.SelectFrom("artist").OrderBy(rawCase)
		assert.Equal(
			`SELECT * FROM "artist" ORDER BY CASE WHEN id IN (NULL) THEN 0 ELSE 1 END`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}(nil),
			sel.Arguments(),
		)
	}

	{
		rawCase := db.Raw("CASE WHEN id IN (NULL) THEN 0 ELSE 1 END")
		sel := b.SelectFrom("artist").OrderBy(rawCase)
		assert.Equal(
			`SELECT * FROM "artist" ORDER BY CASE WHEN id IN (NULL) THEN 0 ELSE 1 END`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}(nil),
			rawCase.Arguments(),
		)
	}

	{
		rawCase := db.Raw("CASE WHEN id IN (?, ?) THEN 0 ELSE 1 END", 1000, 2000)
		sel := b.SelectFrom("artist").OrderBy(rawCase)
		assert.Equal(
			`SELECT * FROM "artist" ORDER BY CASE WHEN id IN ($1, $2) THEN 0 ELSE 1 END`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1000, 2000},
			rawCase.Arguments(),
		)
	}

	{
		sel := b.Select(db.Func("DISTINCT", "name")).From("artist")
		assert.Equal(
			`SELECT DISTINCT($1) FROM "artist"`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{"name"},
			sel.Arguments(),
		)
	}

	assert.Equal(
		`SELECT * FROM "artist" WHERE (1 = $1)`,
		b.Select().From("artist").Where(db.Cond{1: 1}).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE (1 = ANY($1))`,
		b.Select().From("artist").Where(db.Cond{1: db.Func("ANY", "name")}).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE (1 = ANY(column))`,
		b.Select().From("artist").Where(db.Cond{1: db.Func("ANY", db.Raw("column"))}).String(),
	)

	{
		q := b.Select().From("artist").Where(db.Cond{"id NOT IN": []int{0, -1}})
		assert.Equal(
			`SELECT * FROM "artist" WHERE ("id" NOT IN ($1, $2))`,
			q.String(),
		)
		assert.Equal(
			[]interface{}{0, -1},
			q.Arguments(),
		)
	}

	{
		q := b.Select().From("artist").Where(db.Cond{"id NOT IN": []int{-1}})
		assert.Equal(
			`SELECT * FROM "artist" WHERE ("id" NOT IN ($1))`,
			q.String(),
		)
		assert.Equal(
			[]interface{}{-1},
			q.Arguments(),
		)
	}

	assert.Equal(
		`SELECT * FROM "artist" WHERE ("id" IN ($1, $2))`,
		b.Select().From("artist").Where(db.Cond{"id IN": []int{0, -1}}).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE (("id" = $1 OR "id" = $2 OR "id" = $3))`,
		b.Select().From("artist").Where(
			db.Or(
				db.Cond{"id": 1},
				db.Cond{"id": 2},
				db.Cond{"id": 3},
			),
		).String(),
	)

	{
		q := b.Select().From("artist").Where(
			db.Or(
				db.And(db.Cond{"a": 1}, db.Cond{"b": 2}, db.Cond{"c": 3}),
				db.And(db.Cond{"d": 1}, db.Cond{"e": 2}, db.Cond{"f": 3}),
			),
		)
		assert.Equal(
			`SELECT * FROM "artist" WHERE ((("a" = $1 AND "b" = $2 AND "c" = $3) OR ("d" = $4 AND "e" = $5 AND "f" = $6)))`,
			q.String(),
		)
		assert.Equal(
			[]interface{}{1, 2, 3, 1, 2, 3},
			q.Arguments(),
		)
	}

	{
		q := b.Select().From("artist").Where(
			db.Or(
				db.And(db.Cond{"a": 1, "b": 2, "c": 3}),
				db.And(db.Cond{"f": 6, "e": 5, "d": 4}),
			),
		)
		assert.Equal(
			`SELECT * FROM "artist" WHERE ((("a" = $1 AND "b" = $2 AND "c" = $3) OR ("d" = $4 AND "e" = $5 AND "f" = $6)))`,
			q.String(),
		)
		assert.Equal(
			[]interface{}{1, 2, 3, 4, 5, 6},
			q.Arguments(),
		)
	}

	assert.Equal(
		`SELECT * FROM "artist" WHERE ((("id" = $1 OR "id" = $2 OR "id" IS NULL) OR ("name" = $3 OR "name" = $4)))`,
		b.Select().From("artist").Where(
			db.Or(
				db.Or(
					db.Cond{"id": 1},
					db.Cond{"id": 2},
					db.Cond{"id IS": nil},
				),
				db.Or(
					db.Cond{"name": "John"},
					db.Cond{"name": "Peter"},
				),
			),
		).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE ((("id" = $1 OR "id" = $2 OR "id" = $3 OR "id" = $4) AND ("name" = $5 AND "last_name" = $6) AND "age" > $7))`,
		b.Select().From("artist").Where(
			db.And(
				db.Or(
					db.Cond{"id": 1},
					db.Cond{"id": 2},
					db.Cond{"id": 3},
				).Or(
					db.Cond{"id": 4},
				),
				db.Or(),
				db.And(
					db.Cond{"name": "John"},
					db.Cond{"last_name": "Smith"},
				),
				db.And(),
			).And(
				db.Cond{"age >": "20"},
			),
		).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist"`,
		b.Select().From("artist").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" ORDER BY "name" DESC`,
		b.Select().From("artist").OrderBy("name DESC").String(),
	)

	{
		sel := b.Select().From("artist").OrderBy(db.Raw("id = ?", 1), "name DESC")
		assert.Equal(
			`SELECT * FROM "artist" ORDER BY id = $1 , "name" DESC`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1},
			sel.Arguments(),
		)
	}

	{
		sel := b.Select().From("artist").OrderBy(db.Func("RAND"))
		assert.Equal(
			`SELECT * FROM "artist" ORDER BY RAND()`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}(nil),
			sel.Arguments(),
		)
	}

	assert.Equal(
		`SELECT * FROM "artist" ORDER BY RAND()`,
		b.Select().From("artist").OrderBy(db.Raw("RAND()")).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" ORDER BY "name" DESC`,
		b.Select().From("artist").OrderBy("-name").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" ORDER BY "name" ASC`,
		b.Select().From("artist").OrderBy("name").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" ORDER BY "name" ASC`,
		b.Select().From("artist").OrderBy("name ASC").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" LIMIT -1 OFFSET 5`,
		b.Select().From("artist").Limit(-1).Offset(5).String(),
	)

	assert.Equal(
		`SELECT "id" FROM "artist"`,
		b.Select("id").From("artist").String(),
	)

	assert.Equal(
		`SELECT "id", "name" FROM "artist"`,
		b.Select("id", "name").From("artist").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE ("name" = $1)`,
		b.SelectFrom("artist").Where("name", "Haruki").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE (name LIKE $1)`,
		b.SelectFrom("artist").Where("name LIKE ?", `%F%`).String(),
	)

	assert.Equal(
		`SELECT "id" FROM "artist" WHERE (name LIKE $1 OR name LIKE $2)`,
		b.Select("id").From("artist").Where(`name LIKE ? OR name LIKE ?`, `%Miya%`, `F%`).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE ("id" > $1)`,
		b.SelectFrom("artist").Where("id >", 2).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE (id <= 2 AND name != $1)`,
		b.SelectFrom("artist").Where("id <= 2 AND name != ?", "A").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE ("id" IN ($1, $2, $3, $4))`,
		b.SelectFrom("artist").Where("id IN", []int{1, 9, 8, 7}).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE (id IN ($1, $2, $3, $4) AND foo = $5 AND bar IN ($6, $7, $8))`,
		b.SelectFrom("artist").Where("id IN ? AND foo = ? AND bar IN ?", []int{1, 9, 8, 7}, 28, []int{1, 2, 3}).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE (name IS NOT NULL)`,
		b.SelectFrom("artist").Where("name IS NOT NULL").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" AS "a", "publication" AS "p" WHERE (p.author_id = a.id) LIMIT 1`,
		b.Select().From("artist a", "publication as p").Where("p.author_id = a.id").Limit(1).String(),
	)

	assert.Equal(
		`SELECT "id" FROM "artist" NATURAL JOIN "publication"`,
		b.Select("id").From("artist").Join("publication").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" AS "a" JOIN "publication" AS "p" ON (p.author_id = a.id) LIMIT 1`,
		b.SelectFrom("artist a").Join("publication p").On("p.author_id = a.id").Limit(1).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" AS "a" JOIN "publication" AS "p" ON (p.author_id = a.id) WHERE ("a"."id" = $1) LIMIT 1`,
		b.SelectFrom("artist a").Join("publication p").On("p.author_id = a.id").Where("a.id", 2).Limit(1).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" JOIN "publication" AS "p" ON (p.author_id = a.id) WHERE (a.id = 2) LIMIT 1`,
		b.SelectFrom("artist").Join("publication p").On("p.author_id = a.id").Where("a.id = 2").Limit(1).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" AS "a" JOIN "publication" AS "p" ON (p.title LIKE $1 OR p.title LIKE $2) WHERE (a.id = $3) LIMIT 1`,
		b.SelectFrom("artist a").Join("publication p").On("p.title LIKE ? OR p.title LIKE ?", "%Totoro%", "%Robot%").Where("a.id = ?", 2).Limit(1).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" AS "a" JOIN "publication" AS "p" ON (p.title LIKE $1 OR p.title LIKE $2) WHERE (a.id = $3) LIMIT 1`,
		b.SelectFrom("artist a").Join("publication p").On("p.title LIKE ? OR p.title LIKE ?", "%Totoro%", "%Robot%").Where("a.id = ?", 2).Where("a.id = ?", 3).Limit(1).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" AS "a" JOIN "publication" AS "p" ON (p.title LIKE $1 OR p.title LIKE $2) WHERE (a.id = $3 AND a.id = $4) LIMIT 1`,
		b.SelectFrom("artist a").Join("publication p").On("p.title LIKE ? OR p.title LIKE ?", "%Totoro%", "%Robot%").Where("a.id = ?", 2).And("a.id = ?", 3).Limit(1).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" AS "a" LEFT JOIN "publication" AS "p1" ON (p1.id = a.id) RIGHT JOIN "publication" AS "p2" ON (p2.id = a.id)`,
		b.SelectFrom("artist a").
			LeftJoin("publication p1").On("p1.id = a.id").
			RightJoin("publication p2").On("p2.id = a.id").
			String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" CROSS JOIN "publication"`,
		b.SelectFrom("artist").CrossJoin("publication").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" JOIN "publication" USING ("id")`,
		b.SelectFrom("artist").Join("publication").Using("id").String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE ("id" IS NULL)`,
		b.SelectFrom("artist").Where(db.Cond{"id": nil}).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE ("id" IN (NULL))`,
		b.SelectFrom("artist").Where(db.Cond{"id": []int64{}}).String(),
	)

	assert.Equal(
		`SELECT * FROM "artist" WHERE ("id" IN ($1))`,
		b.SelectFrom("artist").Where(db.Cond{"id": []int64{0}}).String(),
	)

	assert.Equal(
		`SELECT DATE()`,
		b.Select(db.Raw("DATE()")).String(),
	)

	{
		sel := b.Select(db.Raw("CONCAT(?, ?)", "foo", "bar"))
		assert.Equal(
			`SELECT CONCAT($1, $2)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{"foo", "bar"},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Cond{"bar": db.Raw("1")})
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("bar" = 1)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}(nil),
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Cond{db.Raw("1"): 1})
		assert.Equal(
			`SELECT * FROM "foo" WHERE (1 = $1)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Cond{db.Raw("1"): db.Raw("1")})
		assert.Equal(
			`SELECT * FROM "foo" WHERE (1 = 1)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}(nil),
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Raw("1 = 1"))
		assert.Equal(
			`SELECT * FROM "foo" WHERE (1 = 1)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}(nil),
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Cond{"bar": 1}, db.Cond{"baz": db.Raw("CONCAT(?, ?)", "foo", "bar")})
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("bar" = $1 AND "baz" = CONCAT($2, $3))`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1, "foo", "bar"},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Cond{"bar": 1}, db.Raw("? = ANY(col)", "name"))
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("bar" = $1 AND $2 = ANY(col))`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1, "name"},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Cond{"bar": 1}, db.Cond{"name": db.Raw("ANY(col)")})
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("bar" = $1 AND "name" = ANY(col))`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Cond{"bar": 1}, db.Cond{db.Raw("CONCAT(?, ?)", "a", "b"): db.Raw("ANY(col)")})
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("bar" = $1 AND CONCAT($2, $3) = ANY(col))`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1, "a", "b"},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where("bar", 2).And(db.Cond{"baz": 1})
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("bar" = $1 AND "baz" = $2)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{2, 1},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").And(db.Cond{"bar": 1})
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("bar" = $1)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where("bar", 2).And(db.Cond{"baz": 1})
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("bar" = $1 AND "baz" = $2)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{2, 1},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where("bar", 2).Where(db.Cond{"baz": 1})
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("baz" = $1)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Raw("bar->'baz' = ?", true))
		assert.Equal(
			`SELECT * FROM "foo" WHERE (bar->'baz' = $1)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{true},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where(db.Cond{}).And(db.Cond{})
		assert.Equal(
			`SELECT * FROM "foo"`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}(nil),
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where("bar = 1").And(db.Or(
			db.Raw("fieldA ILIKE ?", `%a%`),
			db.Raw("fieldB ILIKE ?", `%b%`),
		))
		assert.Equal(
			`SELECT * FROM "foo" WHERE (bar = 1 AND (fieldA ILIKE $1 OR fieldB ILIKE $2))`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{`%a%`, `%b%`},
			sel.Arguments(),
		)
	}

	{
		sel := b.SelectFrom("foo").Where("group_id", 1).And("user_id", 2)
		assert.Equal(
			`SELECT * FROM "foo" WHERE ("group_id" = $1 AND "user_id" = $2)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{1, 2},
			sel.Arguments(),
		)
	}

	{
		s := `SUM(CASE WHEN foo in ? THEN 1 ELSE 0 END) AS _sum`
		sel := b.Select("c1").Columns(db.Raw(s, []int{5, 4, 3, 2})).From("foo").Where("bar = ?", 1)
		assert.Equal(
			`SELECT "c1", SUM(CASE WHEN foo in ($1, $2, $3, $4) THEN 1 ELSE 0 END) AS _sum FROM "foo" WHERE (bar = $5)`,
			sel.String(),
		)
		assert.Equal(
			[]interface{}{5, 4, 3, 2, 1},
			sel.Arguments(),
		)
	}

	{
		s := `SUM(CASE WHEN foo in ? THEN 1 ELSE 0 END) AS _sum`
		sel := b.Select("c1").Columns(db.Raw(s, []int{5, 4, 3, 2})).From("foo").Where("bar = ?", 1)
		sel2 := b.SelectFrom(sel).As("subquery").Where(db.Cond{"foo": "bar"}).OrderBy("subquery.seq")
		assert.Equal(
			`SELECT * FROM (SELECT "c1", SUM(CASE WHEN foo in ($1, $2, $3, $4) THEN 1 ELSE 0 END) AS _sum FROM "foo" WHERE (bar = $5)) AS "subquery" WHERE ("foo" = $6) ORDER BY "subquery"."seq" ASC`,
			sel2.String(),
		)
		assert.Equal(
			[]interface{}{5, 4, 3, 2, 1, "bar"},
			sel2.Arguments(),
		)
	}

	{
		series := b.Select(
			db.Raw("start + interval ? - interval '1s' AS end", "1 day"),
		).From(
			b.Select(
				db.Raw("generate_series(?::timestamp, ?::timestamp, ?::interval) AS start", 1, 2, 3),
			),
		).As("series")

		assert.Equal(
			[]interface{}{"1 day", 1, 2, 3},
			series.Arguments(),
		)

		distinct := b.Select().Distinct(
			db.Raw(`ON(dt.email) SUBSTRING(email,(POSITION('@' in email) + 1),252) AS email_domain`),
			"dt.event_type AS event_type",
			db.Raw("count(dt.*) AS count"),
			"intervals.start AS start",
			"intervals.end AS start",
		).From("email_events AS dt").
			RightJoin("intervals").On("dt.ts BETWEEN intervals.stast AND intervals.END AND dt.hub_id = ? AND dt.object_id = ?", 67, 68).
			GroupBy("email_domain", "event_type", "start").
			OrderBy("email", "start", "event_type")

		sq, args := Preprocess(
			`WITH intervals AS ? ?`,
			[]interface{}{
				series,
				distinct,
			},
		)

		assert.Equal(
			stripWhitespace(`
				WITH intervals AS (SELECT start + interval ? - interval '1s' AS end FROM (SELECT generate_series(?::timestamp, ?::timestamp, ?::interval) AS start) AS "series")
					(SELECT DISTINCT ON(dt.email) SUBSTRING(email,(POSITION('@' in email) + 1),252) AS email_domain, "dt"."event_type" AS "event_type", count(dt.*) AS count, "intervals"."start" AS "start", "intervals"."end" AS "start"
						FROM "email_events" AS "dt"
						RIGHT JOIN "intervals" ON (dt.ts BETWEEN intervals.stast AND intervals.END AND dt.hub_id = ? AND dt.object_id = ?)
						GROUP BY "email_domain", "event_type", "start"
						ORDER BY "email" ASC, "start" ASC, "event_type" ASC)`),
			stripWhitespace(sq),
		)

		assert.Equal(
			[]interface{}{"1 day", 1, 2, 3, 67, 68},
			args,
		)
	}

	{
		sq := b.
			Select("user_id").
			From("user_access").
			Where(db.Cond{"hub_id": 3})

		// Don't reassign
		_ = sq.And(db.Cond{"role": []int{1, 2}})

		assert.Equal(
			`SELECT "user_id" FROM "user_access" WHERE ("hub_id" = $1)`,
			sq.String(),
		)

		assert.Equal(
			[]interface{}{3},
			sq.Arguments(),
		)

		// Reassign
		sq = sq.And(db.Cond{"role": []int{1, 2}})

		assert.Equal(
			`SELECT "user_id" FROM "user_access" WHERE ("hub_id" = $1 AND "role" IN ($2, $3))`,
			sq.String(),
		)

		assert.Equal(
			[]interface{}{3, 1, 2},
			sq.Arguments(),
		)

		cond := db.Or(
			db.Raw("a.id IN ?", sq),
		)

		cond = cond.Or(db.Cond{"ml.mailing_list_id": []int{4, 5, 6}})

		sel := b.
			Select(db.Raw("DISTINCT ON(a.id) a.id"), db.Raw("COALESCE(NULLIF(ml.name,''), a.name) as name"), "a.email").
			From("mailing_list_recipients ml").
			FullJoin("accounts a").On("a.id = ml.user_id").
			Where(cond)

		search := "word"
		sel = sel.And(db.Or(
			db.Raw("COALESCE(NULLIF(ml.name,''), a.name) ILIKE ?", fmt.Sprintf("%%%s%%", search)),
			db.Cond{"a.email ILIKE": fmt.Sprintf("%%%s%%", search)},
		))

		assert.Equal(
			`SELECT DISTINCT ON(a.id) a.id, COALESCE(NULLIF(ml.name,''), a.name) as name, "a"."email" FROM "mailing_list_recipients" AS "ml" FULL JOIN "accounts" AS "a" ON (a.id = ml.user_id) WHERE ((a.id IN (SELECT "user_id" FROM "user_access" WHERE ("hub_id" = $1 AND "role" IN ($2, $3))) OR "ml"."mailing_list_id" IN ($4, $5, $6)) AND (COALESCE(NULLIF(ml.name,''), a.name) ILIKE $7 OR "a"."email" ILIKE $8))`,
			sel.String(),
		)

		assert.Equal(
			[]interface{}{3, 1, 2, 4, 5, 6, `%word%`, `%word%`},
			sel.Arguments(),
		)

	}
}

func TestInsert(t *testing.T) {
	b := &sqlBuilder{t: newTemplateWithUtils(&testTemplate)}
	assert := assert.New(t)

	assert.Equal(
		`INSERT INTO "artist" VALUES ($1, $2), ($3, $4), ($5, $6)`,
		b.InsertInto("artist").
			Values(10, "Ryuichi Sakamoto").
			Values(11, "Alondra de la Parra").
			Values(12, "Haruki Murakami").
			String(),
	)

	assert.Equal(
		`INSERT INTO "artist" ("id", "name") VALUES ($1, $2)`,
		b.InsertInto("artist").Values(map[string]string{"id": "12", "name": "Chavela Vargas"}).String(),
	)

	assert.Equal(
		`INSERT INTO "artist" ("id", "name") VALUES ($1, $2) RETURNING "id"`,
		b.InsertInto("artist").Values(map[string]string{"id": "12", "name": "Chavela Vargas"}).Returning("id").String(),
	)

	assert.Equal(
		`INSERT INTO "artist" ("id", "name") VALUES ($1, $2) RETURNING "id"`,
		b.InsertInto("artist").Values(map[string]string{"id": "12", "name": "Chavela Vargas"}).Amend(func(query string) string {
			return query + ` RETURNING "id"`
		}).String(),
	)

	assert.Equal(
		`INSERT INTO "artist" ("id", "name") VALUES ($1, $2)`,
		b.InsertInto("artist").Values(map[string]interface{}{"name": "Chavela Vargas", "id": 12}).String(),
	)

	assert.Equal(
		`INSERT INTO "artist" ("id", "name") VALUES ($1, $2)`,
		b.InsertInto("artist").Values(struct {
			ID   int    `db:"id"`
			Name string `db:"name"`
		}{12, "Chavela Vargas"}).String(),
	)

	{
		type artistStruct struct {
			ID   int    `db:"id,omitempty"`
			Name string `db:"name,omitempty"`
		}

		assert.Equal(
			`INSERT INTO "artist" ("id", "name") VALUES ($1, $2), ($3, $4), ($5, $6)`,
			b.InsertInto("artist").
				Values(artistStruct{12, "Chavela Vargas"}).
				Values(artistStruct{13, "Alondra de la Parra"}).
				Values(artistStruct{14, "Haruki Murakami"}).
				String(),
		)
	}

	{
		type artistStruct struct {
			ID   int    `db:"id,omitempty"`
			Name string `db:"name,omitempty"`
		}

		q := b.InsertInto("artist").
			Values(artistStruct{0, ""}).
			Values(artistStruct{12, "Chavela Vargas"}).
			Values(artistStruct{0, "Alondra de la Parra"}).
			Values(artistStruct{14, ""}).
			Values(artistStruct{0, ""})

		assert.Equal(
			`INSERT INTO "artist" ("id", "name") VALUES (DEFAULT, DEFAULT), ($1, $2), (DEFAULT, $3), ($4, DEFAULT), (DEFAULT, DEFAULT)`,
			q.String(),
		)

		assert.Equal(
			[]interface{}{12, "Chavela Vargas", "Alondra de la Parra", 14},
			q.Arguments(),
		)
	}

	{
		type artistStruct struct {
			ID   int    `db:"id,omitempty"`
			Name string `db:"name,omitempty"`
		}

		assert.Equal(
			`INSERT INTO "artist" ("name") VALUES ($1)`,
			b.InsertInto("artist").
				Values(artistStruct{Name: "Chavela Vargas"}).
				String(),
		)

		assert.Equal(
			`INSERT INTO "artist" ("id") VALUES ($1)`,
			b.InsertInto("artist").
				Values(artistStruct{ID: 1}).
				String(),
		)
	}

	{
		type artistStruct struct {
			ID   int    `db:"id,omitempty"`
			Name string `db:"name,omitempty"`
		}

		{
			q := b.InsertInto("artist").Values(artistStruct{Name: "Chavela Vargas"})

			assert.Equal(
				`INSERT INTO "artist" ("name") VALUES ($1)`,
				q.String(),
			)
			assert.Equal(
				[]interface{}{"Chavela Vargas"},
				q.Arguments(),
			)
		}

		{
			q := b.InsertInto("artist").Values(artistStruct{Name: "Chavela Vargas"}).Values(artistStruct{Name: "Alondra de la Parra"})

			assert.Equal(
				`INSERT INTO "artist" ("id", "name") VALUES (DEFAULT, $1), (DEFAULT, $2)`,
				q.String(),
			)
			assert.Equal(
				[]interface{}{"Chavela Vargas", "Alondra de la Parra"},
				q.Arguments(),
			)
		}

		{
			q := b.InsertInto("artist").Values(artistStruct{ID: 1})

			assert.Equal(
				`INSERT INTO "artist" ("id") VALUES ($1)`,
				q.String(),
			)

			assert.Equal(
				[]interface{}{1},
				q.Arguments(),
			)
		}

		{
			q := b.InsertInto("artist").Values(artistStruct{ID: 1}).Values(artistStruct{ID: 2})

			assert.Equal(
				`INSERT INTO "artist" ("id", "name") VALUES ($1, DEFAULT), ($2, DEFAULT)`,
				q.String(),
			)

			assert.Equal(
				[]interface{}{1, 2},
				q.Arguments(),
			)
		}

	}

	{
		intRef := func(i int) *int {
			if i == 0 {
				return nil
			}
			return &i
		}

		strRef := func(s string) *string {
			if s == "" {
				return nil
			}
			return &s
		}

		type artistStruct struct {
			ID   *int    `db:"id,omitempty"`
			Name *string `db:"name,omitempty"`
		}

		q := b.InsertInto("artist").
			Values(artistStruct{intRef(0), strRef("")}).
			Values(artistStruct{intRef(12), strRef("Chavela Vargas")}).
			Values(artistStruct{intRef(0), strRef("Alondra de la Parra")}).
			Values(artistStruct{intRef(14), strRef("")}).
			Values(artistStruct{intRef(0), strRef("")})

		assert.Equal(
			`INSERT INTO "artist" ("id", "name") VALUES (DEFAULT, DEFAULT), ($1, $2), (DEFAULT, $3), ($4, DEFAULT), (DEFAULT, DEFAULT)`,
			q.String(),
		)

		assert.Equal(
			[]interface{}{intRef(12), strRef("Chavela Vargas"), strRef("Alondra de la Parra"), intRef(14)},
			q.Arguments(),
		)
	}

	assert.Equal(
		`INSERT INTO "artist" ("name", "id") VALUES ($1, $2)`,
		b.InsertInto("artist").Columns("name", "id").Values("Chavela Vargas", 12).String(),
	)

	assert.Equal(
		`INSERT INTO "artist" VALUES (default)`,
		b.InsertInto("artist").String(),
	)
}

func TestUpdate(t *testing.T) {
	b := &sqlBuilder{t: newTemplateWithUtils(&testTemplate)}
	assert := assert.New(t)

	assert.Equal(
		`UPDATE "artist" SET "name" = $1`,
		b.Update("artist").Set("name", "Artist").String(),
	)

	assert.Equal(
		`UPDATE "artist" SET "name" = $1 RETURNING "name"`,
		b.Update("artist").Set("name", "Artist").Amend(func(query string) string {
			return query + ` RETURNING "name"`
		}).String(),
	)

	{
		idSlice := []int64{8, 7, 6}
		q := b.Update("artist").Set(db.Cond{"some_column": 10}).Where(db.Cond{"id": 1}, db.Cond{"another_val": idSlice})
		assert.Equal(
			`UPDATE "artist" SET "some_column" = $1 WHERE ("id" = $2 AND "another_val" IN ($3, $4, $5))`,
			q.String(),
		)
		assert.Equal(
			[]interface{}{10, 1, int64(8), int64(7), int64(6)},
			q.Arguments(),
		)
	}

	{
		idSlice := []int64{}
		q := b.Update("artist").Set(db.Cond{"some_column": 10}).Where(db.Cond{"id": 1}, db.Cond{"another_val": idSlice})
		assert.Equal(
			`UPDATE "artist" SET "some_column" = $1 WHERE ("id" = $2 AND "another_val" IN (NULL))`,
			q.String(),
		)
		assert.Equal(
			[]interface{}{10, 1},
			q.Arguments(),
		)
	}

	{
		idSlice := []int64{}
		q := b.Update("artist").Where(db.Cond{"id": 1}, db.Cond{"another_val": idSlice}).Set(db.Cond{"some_column": 10})
		assert.Equal(
			`UPDATE "artist" SET "some_column" = $1 WHERE ("id" = $2 AND "another_val" IN (NULL))`,
			q.String(),
		)
		assert.Equal(
			[]interface{}{10, 1},
			q.Arguments(),
		)
	}

	assert.Equal(
		`UPDATE "artist" SET "name" = $1 WHERE ("id" < $2)`,
		b.Update("artist").Set("name = ?", "Artist").Where("id <", 5).String(),
	)

	assert.Equal(
		`UPDATE "artist" SET "name" = $1 WHERE ("id" < $2)`,
		b.Update("artist").Set(map[string]string{"name": "Artist"}).Where(db.Cond{"id <": 5}).String(),
	)

	assert.Equal(
		`UPDATE "artist" SET "name" = $1 WHERE ("id" < $2)`,
		b.Update("artist").Set(struct {
			Nombre string `db:"name"`
		}{"Artist"}).Where(db.Cond{"id <": 5}).String(),
	)

	assert.Equal(
		`UPDATE "artist" SET "name" = $1 WHERE ("id" < $2)`,
		b.Update("artist").Where(db.Cond{"id <": 5}).Set(struct {
			Nombre string `db:"name"`
		}{"Artist"}).String(),
	)

	assert.Equal(
		`UPDATE "artist" SET "name" = $1, "last_name" = $2 WHERE ("id" < $3)`,
		b.Update("artist").Set(struct {
			Nombre string `db:"name"`
		}{"Artist"}).Set(map[string]string{"last_name": "Foo"}).Where(db.Cond{"id <": 5}).String(),
	)

	assert.Equal(
		`UPDATE "artist" SET "name" = $1 || ' ' || $2 || id, "id" = id + $3 WHERE (id > $4)`,
		b.Update("artist").Set(
			"name = ? || ' ' || ? || id", "Artist", "#",
			"id = id + ?", 10,
		).Where("id > ?", 0).String(),
	)

	{
		q := b.Update("posts").Set("column = ?", "foo")

		assert.Equal(
			`UPDATE "posts" SET "column" = $1`,
			q.String(),
		)

		assert.Equal(
			[]interface{}{"foo"},
			q.Arguments(),
		)
	}

	{
		q := b.Update("posts").Set(db.Raw("column = ?", "foo"))

		assert.Equal(
			`UPDATE "posts" SET column = $1`,
			q.String(),
		)

		assert.Equal(
			[]interface{}{"foo"},
			q.Arguments(),
		)
	}

	{
		q := b.Update("posts").Set("foo = bar")

		assert.Equal(
			[]interface{}(nil),
			q.Arguments(),
		)

		assert.Equal(
			`UPDATE "posts" SET "foo" = bar`,
			q.String(),
		)
	}

	{
		q := b.Update("posts").Set(
			db.Cond{"tags": db.Raw("array_remove(tags, ?)", "foo")},
		).Where(db.Raw("hub_id = ? AND ? = ANY(tags) AND ? = ANY(tags)", 1, "bar", "baz"))

		assert.Equal(
			`UPDATE "posts" SET "tags" = array_remove(tags, $1) WHERE (hub_id = $2 AND $3 = ANY(tags) AND $4 = ANY(tags))`,
			q.String(),
		)

		assert.Equal(
			[]interface{}{"foo", 1, "bar", "baz"},
			q.Arguments(),
		)
	}
}

func TestDelete(t *testing.T) {
	bt := WithTemplate(&testTemplate)
	assert := assert.New(t)

	assert.Equal(
		`DELETE FROM "artist" WHERE (name = $1)`,
		bt.DeleteFrom("artist").Where("name = ?", "Chavela Vargas").String(),
	)

	assert.Equal(
		`DELETE FROM "artist" WHERE (name = $1) RETURNING 1`,
		bt.DeleteFrom("artist").Where("name = ?", "Chavela Vargas").Amend(func(query string) string {
			return fmt.Sprintf("%s RETURNING 1", query)
		}).String(),
	)

	assert.Equal(
		`DELETE FROM "artist" WHERE (id > 5)`,
		bt.DeleteFrom("artist").Where("id > 5").String(),
	)
}

func BenchmarkDelete1(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.DeleteFrom("artist").Where("name = ?", "Chavela Vargas").Limit(1).String()
	}
}

func BenchmarkDelete2(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.DeleteFrom("artist").Where("id > 5").String()
	}
}

func BenchmarkInsert1(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.InsertInto("artist").Values(10, "Ryuichi Sakamoto").Values(11, "Alondra de la Parra").Values(12, "Haruki Murakami").String()
	}
}

func BenchmarkInsert2(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.InsertInto("artist").Values(map[string]string{"id": "12", "name": "Chavela Vargas"}).String()
	}
}

func BenchmarkInsert3(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.InsertInto("artist").Values(map[string]string{"id": "12", "name": "Chavela Vargas"}).Returning("id").String()
	}
}

func BenchmarkInsert4(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.InsertInto("artist").Values(map[string]interface{}{"name": "Chavela Vargas", "id": 12}).String()
	}
}

func BenchmarkInsert5(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.InsertInto("artist").Values(struct {
			ID   int    `db:"id"`
			Name string `db:"name"`
		}{12, "Chavela Vargas"}).String()
	}
}

func BenchmarkSelect1(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.Select().From("artist").OrderBy("name DESC").String()
	}
}

func BenchmarkSelect2(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.Select("id").From("artist").Where(`name LIKE ? OR name LIKE ?`, `%Miya%`, `F%`).String()
	}
}

func BenchmarkSelect3(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.Select().From("artist a", "publication as p").Where("p.author_id = a.id").Limit(1).String()
	}
}

func BenchmarkSelect4(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.SelectFrom("artist").Join("publication p").On("p.author_id = a.id").Where("a.id = 2").Limit(1).String()
	}
}

func BenchmarkSelect5(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.SelectFrom("artist a").
			LeftJoin("publication p1").On("p1.id = a.id").
			RightJoin("publication p2").On("p2.id = a.id").
			String()
	}
}

func BenchmarkUpdate1(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.Update("artist").Set("name", "Artist").String()
	}
}

func BenchmarkUpdate2(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.Update("artist").Set("name = ?", "Artist").Where("id <", 5).String()
	}
}

func BenchmarkUpdate3(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.Update("artist").Set(struct {
			Nombre string `db:"name"`
		}{"Artist"}).Set(map[string]string{"last_name": "Foo"}).Where(db.Cond{"id <": 5}).String()
	}
}

func BenchmarkUpdate4(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.Update("artist").Set(map[string]string{"name": "Artist"}).Where(db.Cond{"id <": 5}).String()
	}
}

func BenchmarkUpdate5(b *testing.B) {
	bt := WithTemplate(&testTemplate)
	for n := 0; n < b.N; n++ {
		bt.Update("artist").Set(
			"name = ? || ' ' || ? || id", "Artist", "#",
			"id = id + ?", 10,
		).Where("id > ?", 0).String()
	}
}

func stripWhitespace(in string) string {
	q := reInvisibleChars.ReplaceAllString(in, ` `)
	return strings.TrimSpace(q)
}
