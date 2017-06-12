package exql

import (
	"testing"
)

func TestGroupBy(t *testing.T) {
	columns := GroupByColumns(
		&Column{Name: "id"},
		&Column{Name: "customer"},
		&Column{Name: "service_id"},
		&Column{Name: "role.name"},
		&Column{Name: "role.id"},
	)

	s := mustTrim(columns.Compile(defaultTemplate))
	e := `GROUP BY "id", "customer", "service_id", "role"."name", "role"."id"`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func BenchmarkGroupByColumns(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GroupByColumns(
			&Column{Name: "a"},
			&Column{Name: "b"},
			&Column{Name: "c"},
		)
	}
}

func BenchmarkGroupByHash(b *testing.B) {
	c := GroupByColumns(
		&Column{Name: "id"},
		&Column{Name: "customer"},
		&Column{Name: "service_id"},
		&Column{Name: "role.name"},
		&Column{Name: "role.id"},
	)
	for i := 0; i < b.N; i++ {
		c.Hash()
	}
}

func BenchmarkGroupByCompile(b *testing.B) {
	c := GroupByColumns(
		&Column{Name: "id"},
		&Column{Name: "customer"},
		&Column{Name: "service_id"},
		&Column{Name: "role.name"},
		&Column{Name: "role.id"},
	)
	for i := 0; i < b.N; i++ {
		c.Compile(defaultTemplate)
	}
}

func BenchmarkGroupByCompileNoCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := GroupByColumns(
			&Column{Name: "id"},
			&Column{Name: "customer"},
			&Column{Name: "service_id"},
			&Column{Name: "role.name"},
			&Column{Name: "role.id"},
		)
		c.Compile(defaultTemplate)
	}
}
