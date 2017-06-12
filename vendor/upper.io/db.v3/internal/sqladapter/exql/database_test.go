package exql

import (
	"fmt"
	"testing"
)

func TestDatabaseHash(t *testing.T) {
	var s, e string

	column := Database{Name: "users"}

	s = column.Hash()
	e = `*exql.Database:16777957551305673389`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestDatabaseCompile(t *testing.T) {
	column := Database{Name: "name"}

	s, err := column.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `"name"`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func BenchmarkDatabaseHash(b *testing.B) {
	c := Database{Name: "name"}
	for i := 0; i < b.N; i++ {
		c.Hash()
	}
}

func BenchmarkDatabaseCompile(b *testing.B) {
	c := Database{Name: "name"}
	for i := 0; i < b.N; i++ {
		c.Compile(defaultTemplate)
	}
}

func BenchmarkDatabaseCompileNoCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := Database{Name: "name"}
		c.Compile(defaultTemplate)
	}
}

func BenchmarkDatabaseCompileNoCache2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := Database{Name: fmt.Sprintf("name: %v", i)}
		c.Compile(defaultTemplate)
	}
}
