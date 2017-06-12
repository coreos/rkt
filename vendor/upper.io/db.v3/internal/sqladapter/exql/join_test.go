package exql

import (
	"fmt"
	"testing"
)

func TestOnAndRawOrAnd(t *testing.T) {
	var s, e string

	on := OnConditions(
		JoinWithAnd(
			&ColumnValue{Column: &Column{Name: "id"}, Operator: ">", Value: NewValue(&Raw{Value: "8"})},
			&ColumnValue{Column: &Column{Name: "id"}, Operator: "<", Value: NewValue(&Raw{Value: "99"})},
		),
		&ColumnValue{Column: &Column{Name: "name"}, Operator: "=", Value: NewValue("John")},
		&Raw{Value: "city_id = 728"},
		JoinWithOr(
			&ColumnValue{Column: &Column{Name: "last_name"}, Operator: "=", Value: NewValue("Smith")},
			&ColumnValue{Column: &Column{Name: "last_name"}, Operator: "=", Value: NewValue("Reyes")},
		),
		JoinWithAnd(
			&ColumnValue{Column: &Column{Name: "age"}, Operator: ">", Value: NewValue(&Raw{Value: "18"})},
			&ColumnValue{Column: &Column{Name: "age"}, Operator: "<", Value: NewValue(&Raw{Value: "41"})},
		),
	)

	s = mustTrim(on.Compile(defaultTemplate))
	e = `ON (("id" > 8 AND "id" < 99) AND "name" = 'John' AND city_id = 728 AND ("last_name" = 'Smith' OR "last_name" = 'Reyes') AND ("age" > 18 AND "age" < 41))`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestUsing(t *testing.T) {
	var s, e string

	using := UsingColumns(
		&Column{Name: "country"},
		&Column{Name: "state"},
	)

	s = mustTrim(using.Compile(defaultTemplate))
	e = `USING ("country", "state")`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestJoinOn(t *testing.T) {
	var s, e string

	join := JoinConditions(
		&Join{
			Table: TableWithName("countries c"),
			On: OnConditions(
				&ColumnValue{
					Column:   &Column{Name: "p.country_id"},
					Operator: "=",
					Value:    NewValue(&Column{Name: "a.id"}),
				},
				&ColumnValue{
					Column:   &Column{Name: "p.country_code"},
					Operator: "=",
					Value:    NewValue(&Column{Name: "a.code"}),
				},
			),
		},
	)

	s = mustTrim(join.Compile(defaultTemplate))
	e = `JOIN "countries" AS "c" ON ("p"."country_id" = "a"."id" AND "p"."country_code" = "a"."code")`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestInnerJoinOn(t *testing.T) {
	var s, e string

	join := JoinConditions(&Join{
		Type:  "INNER",
		Table: TableWithName("countries c"),
		On: OnConditions(
			&ColumnValue{
				Column:   &Column{Name: "p.country_id"},
				Operator: "=",
				Value:    NewValue(ColumnWithName("a.id")),
			},
			&ColumnValue{
				Column:   &Column{Name: "p.country_code"},
				Operator: "=",
				Value:    NewValue(ColumnWithName("a.code")),
			},
		),
	})

	s = mustTrim(join.Compile(defaultTemplate))
	e = `INNER JOIN "countries" AS "c" ON ("p"."country_id" = "a"."id" AND "p"."country_code" = "a"."code")`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestLeftJoinUsing(t *testing.T) {
	var s, e string

	join := JoinConditions(&Join{
		Type:  "LEFT",
		Table: TableWithName("countries"),
		Using: UsingColumns(ColumnWithName("name")),
	})

	s = mustTrim(join.Compile(defaultTemplate))
	e = `LEFT JOIN "countries" USING ("name")`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestNaturalJoinOn(t *testing.T) {
	var s, e string

	join := JoinConditions(&Join{
		Table: TableWithName("countries"),
	})

	s = mustTrim(join.Compile(defaultTemplate))
	e = `NATURAL JOIN "countries"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestNaturalInnerJoinOn(t *testing.T) {
	var s, e string

	join := JoinConditions(&Join{
		Type:  "INNER",
		Table: TableWithName("countries"),
	})

	s = mustTrim(join.Compile(defaultTemplate))
	e = `NATURAL INNER JOIN "countries"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestCrossJoin(t *testing.T) {
	var s, e string

	join := JoinConditions(&Join{
		Type:  "CROSS",
		Table: TableWithName("countries"),
	})

	s = mustTrim(join.Compile(defaultTemplate))
	e = `CROSS JOIN "countries"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestMultipleJoins(t *testing.T) {
	var s, e string

	join := JoinConditions(&Join{
		Type:  "LEFT",
		Table: TableWithName("countries"),
	}, &Join{
		Table: TableWithName("cities"),
	})

	s = mustTrim(join.Compile(defaultTemplate))
	e = `NATURAL LEFT JOIN "countries" NATURAL JOIN "cities"`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func BenchmarkJoin(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = JoinConditions(&Join{
			Table: TableWithName("countries c"),
			On: OnConditions(
				&ColumnValue{
					Column:   &Column{Name: "p.country_id"},
					Operator: "=",
					Value:    NewValue(&Column{Name: "a.id"}),
				},
				&ColumnValue{
					Column:   &Column{Name: "p.country_code"},
					Operator: "=",
					Value:    NewValue(&Column{Name: "a.code"}),
				},
			),
		})
	}
}

func BenchmarkCompileJoin(b *testing.B) {
	j := JoinConditions(&Join{
		Table: TableWithName("countries c"),
		On: OnConditions(
			&ColumnValue{
				Column:   &Column{Name: "p.country_id"},
				Operator: "=",
				Value:    NewValue(&Column{Name: "a.id"}),
			},
			&ColumnValue{
				Column:   &Column{Name: "p.country_code"},
				Operator: "=",
				Value:    NewValue(&Column{Name: "a.code"}),
			},
		),
	})
	for i := 0; i < b.N; i++ {
		j.Compile(defaultTemplate)
	}
}

func BenchmarkCompileJoinNoCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		j := JoinConditions(&Join{
			Table: TableWithName("countries c"),
			On: OnConditions(
				&ColumnValue{
					Column:   &Column{Name: "p.country_id"},
					Operator: "=",
					Value:    NewValue(&Column{Name: "a.id"}),
				},
				&ColumnValue{
					Column:   &Column{Name: "p.country_code"},
					Operator: "=",
					Value:    NewValue(&Column{Name: "a.code"}),
				},
			),
		})
		j.Compile(defaultTemplate)
	}
}

func BenchmarkCompileJoinNoCache2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		j := JoinConditions(&Join{
			Table: TableWithName(fmt.Sprintf("countries c", i)),
			On: OnConditions(
				&ColumnValue{
					Column:   &Column{Name: "p.country_id"},
					Operator: "=",
					Value:    NewValue(&Column{Name: "a.id"}),
				},
				&ColumnValue{
					Column:   &Column{Name: "p.country_code"},
					Operator: "=",
					Value:    NewValue(&Column{Name: "a.code"}),
				},
			),
		})
		j.Compile(defaultTemplate)
	}
}
