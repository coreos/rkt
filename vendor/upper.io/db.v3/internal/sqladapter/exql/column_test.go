package exql

import (
	"testing"
)

func TestColumnHash(t *testing.T) {
	var s, e string

	column := Column{Name: "role.name"}

	s = column.Hash()
	e = "*exql.Column:5663680925324531495"

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestColumnString(t *testing.T) {

	column := Column{Name: "role.name"}

	s, err := column.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `"role"."name"`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestColumnAs(t *testing.T) {
	column := Column{Name: "role.name as foo"}

	s, err := column.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `"role"."name" AS "foo"`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestColumnImplicitAs(t *testing.T) {
	column := Column{Name: "role.name foo"}

	s, err := column.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `"role"."name" AS "foo"`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestColumnRaw(t *testing.T) {
	column := Column{Name: Raw{Value: "role.name As foo"}}

	s, err := column.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `role.name As foo`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func BenchmarkColumnWithName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ColumnWithName("a")
	}
}

func BenchmarkColumnHash(b *testing.B) {
	c := Column{Name: "name"}
	for i := 0; i < b.N; i++ {
		c.Hash()
	}
}

func BenchmarkColumnCompile(b *testing.B) {
	c := Column{Name: "name"}
	for i := 0; i < b.N; i++ {
		c.Compile(defaultTemplate)
	}
}

func BenchmarkColumnCompileNoCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := Column{Name: "name"}
		c.Compile(defaultTemplate)
	}
}

func BenchmarkColumnWithDotCompile(b *testing.B) {
	c := Column{Name: "role.name"}
	for i := 0; i < b.N; i++ {
		c.Compile(defaultTemplate)
	}
}

func BenchmarkColumnWithImplicitAsKeywordCompile(b *testing.B) {
	c := Column{Name: "role.name foo"}
	for i := 0; i < b.N; i++ {
		c.Compile(defaultTemplate)
	}
}

func BenchmarkColumnWithAsKeywordCompile(b *testing.B) {
	c := Column{Name: "role.name AS foo"}
	for i := 0; i < b.N; i++ {
		c.Compile(defaultTemplate)
	}
}
