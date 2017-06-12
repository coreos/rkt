package exql

import (
	"testing"
)

func TestColumns(t *testing.T) {
	columns := JoinColumns(
		&Column{Name: "id"},
		&Column{Name: "customer"},
		&Column{Name: "service_id"},
		&Column{Name: "role.name"},
		&Column{Name: "role.id"},
	)

	s, err := columns.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `"id", "customer", "service_id", "role"."name", "role"."id"`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func BenchmarkJoinColumns(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = JoinColumns(
			&Column{Name: "a"},
			&Column{Name: "b"},
			&Column{Name: "c"},
		)
	}
}

func BenchmarkColumnsHash(b *testing.B) {
	c := JoinColumns(
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

func BenchmarkColumnsCompile(b *testing.B) {
	c := JoinColumns(
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

func BenchmarkColumnsCompileNoCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := JoinColumns(
			&Column{Name: "id"},
			&Column{Name: "customer"},
			&Column{Name: "service_id"},
			&Column{Name: "role.name"},
			&Column{Name: "role.id"},
		)
		c.Compile(defaultTemplate)
	}
}
