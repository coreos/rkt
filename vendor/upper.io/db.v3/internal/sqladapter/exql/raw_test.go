package exql

import (
	"testing"
)

func TestRawString(t *testing.T) {
	raw := &Raw{Value: "foo"}

	s, err := raw.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `foo`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestRawCompile(t *testing.T) {
	raw := &Raw{Value: "foo"}

	s, err := raw.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `foo`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestRawHash(t *testing.T) {
	var s, e string

	raw := &Raw{Value: "foo"}

	s = raw.Hash()
	e = `*exql.Raw:5772950988983410957`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func BenchmarkRawCreate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Raw{Value: "foo"}
	}
}

func BenchmarkRawString(b *testing.B) {
	raw := &Raw{Value: "foo"}
	for i := 0; i < b.N; i++ {
		raw.String()
	}
}

func BenchmarkRawCompile(b *testing.B) {
	raw := &Raw{Value: "foo"}
	for i := 0; i < b.N; i++ {
		raw.Compile(defaultTemplate)
	}
}

func BenchmarkRawHash(b *testing.B) {
	raw := &Raw{Value: "foo"}
	for i := 0; i < b.N; i++ {
		raw.Hash()
	}
}
