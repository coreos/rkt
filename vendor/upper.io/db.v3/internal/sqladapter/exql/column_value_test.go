package exql

import (
	"testing"
)

func TestColumnValueHash(t *testing.T) {
	var s, e string

	c := &ColumnValue{Column: ColumnWithName("id"), Operator: "=", Value: NewValue(1)}

	s = c.Hash()
	e = `*exql.ColumnValue:4950005282640920683`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestColumnValuesHash(t *testing.T) {
	var s, e string

	c := JoinColumnValues(
		&ColumnValue{Column: ColumnWithName("id"), Operator: "=", Value: NewValue(1)},
		&ColumnValue{Column: ColumnWithName("id"), Operator: "=", Value: NewValue(2)},
	)

	s = c.Hash()
	e = `*exql.ColumnValues:8728513848368010747`

	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestColumnValue(t *testing.T) {
	cv := &ColumnValue{Column: ColumnWithName("id"), Operator: "=", Value: NewValue(1)}

	s, err := cv.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `"id" = '1'`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}

	cv = &ColumnValue{Column: ColumnWithName("date"), Operator: "=", Value: NewValue(RawValue("NOW()"))}

	s, err = cv.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e = `"date" = NOW()`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func TestColumnValues(t *testing.T) {
	cvs := JoinColumnValues(
		&ColumnValue{Column: ColumnWithName("id"), Operator: ">", Value: NewValue(8)},
		&ColumnValue{Column: ColumnWithName("other.id"), Operator: "<", Value: NewValue(&Raw{Value: "100"})},
		&ColumnValue{Column: ColumnWithName("name"), Operator: "=", Value: NewValue("Haruki Murakami")},
		&ColumnValue{Column: ColumnWithName("created"), Operator: ">=", Value: NewValue(&Raw{Value: "NOW()"})},
		&ColumnValue{Column: ColumnWithName("modified"), Operator: "<=", Value: NewValue(&Raw{Value: "NOW()"})},
	)

	s, err := cvs.Compile(defaultTemplate)
	if err != nil {
		t.Fatal()
	}

	e := `"id" > '8', "other"."id" < 100, "name" = 'Haruki Murakami', "created" >= NOW(), "modified" <= NOW()`
	if s != e {
		t.Fatalf("Got: %s, Expecting: %s", s, e)
	}
}

func BenchmarkNewColumnValue(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = &ColumnValue{Column: ColumnWithName("a"), Operator: "=", Value: NewValue(Raw{Value: "7"})}
	}
}

func BenchmarkColumnValueHash(b *testing.B) {
	cv := &ColumnValue{Column: ColumnWithName("id"), Operator: "=", Value: NewValue(1)}
	for i := 0; i < b.N; i++ {
		cv.Hash()
	}
}

func BenchmarkColumnValueCompile(b *testing.B) {
	cv := &ColumnValue{Column: ColumnWithName("id"), Operator: "=", Value: NewValue(1)}
	for i := 0; i < b.N; i++ {
		cv.Compile(defaultTemplate)
	}
}

func BenchmarkColumnValueCompileNoCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cv := &ColumnValue{Column: ColumnWithName("id"), Operator: "=", Value: NewValue(1)}
		cv.Compile(defaultTemplate)
	}
}

func BenchmarkJoinColumnValues(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = JoinColumnValues(
			&ColumnValue{Column: ColumnWithName("id"), Operator: ">", Value: NewValue(8)},
			&ColumnValue{Column: ColumnWithName("other.id"), Operator: "<", Value: NewValue(Raw{Value: "100"})},
			&ColumnValue{Column: ColumnWithName("name"), Operator: "=", Value: NewValue("Haruki Murakami")},
			&ColumnValue{Column: ColumnWithName("created"), Operator: ">=", Value: NewValue(Raw{Value: "NOW()"})},
			&ColumnValue{Column: ColumnWithName("modified"), Operator: "<=", Value: NewValue(Raw{Value: "NOW()"})},
		)
	}
}

func BenchmarkColumnValuesHash(b *testing.B) {
	cvs := JoinColumnValues(
		&ColumnValue{Column: ColumnWithName("id"), Operator: ">", Value: NewValue(8)},
		&ColumnValue{Column: ColumnWithName("other.id"), Operator: "<", Value: NewValue(Raw{Value: "100"})},
		&ColumnValue{Column: ColumnWithName("name"), Operator: "=", Value: NewValue("Haruki Murakami")},
		&ColumnValue{Column: ColumnWithName("created"), Operator: ">=", Value: NewValue(Raw{Value: "NOW()"})},
		&ColumnValue{Column: ColumnWithName("modified"), Operator: "<=", Value: NewValue(Raw{Value: "NOW()"})},
	)
	for i := 0; i < b.N; i++ {
		cvs.Hash()
	}
}

func BenchmarkColumnValuesCompile(b *testing.B) {
	cvs := JoinColumnValues(
		&ColumnValue{Column: ColumnWithName("id"), Operator: ">", Value: NewValue(8)},
		&ColumnValue{Column: ColumnWithName("other.id"), Operator: "<", Value: NewValue(Raw{Value: "100"})},
		&ColumnValue{Column: ColumnWithName("name"), Operator: "=", Value: NewValue("Haruki Murakami")},
		&ColumnValue{Column: ColumnWithName("created"), Operator: ">=", Value: NewValue(Raw{Value: "NOW()"})},
		&ColumnValue{Column: ColumnWithName("modified"), Operator: "<=", Value: NewValue(Raw{Value: "NOW()"})},
	)
	for i := 0; i < b.N; i++ {
		cvs.Compile(defaultTemplate)
	}
}

func BenchmarkColumnValuesCompileNoCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cvs := JoinColumnValues(
			&ColumnValue{Column: ColumnWithName("id"), Operator: ">", Value: NewValue(8)},
			&ColumnValue{Column: ColumnWithName("other.id"), Operator: "<", Value: NewValue(Raw{Value: "100"})},
			&ColumnValue{Column: ColumnWithName("name"), Operator: "=", Value: NewValue("Haruki Murakami")},
			&ColumnValue{Column: ColumnWithName("created"), Operator: ">=", Value: NewValue(Raw{Value: "NOW()"})},
			&ColumnValue{Column: ColumnWithName("modified"), Operator: "<=", Value: NewValue(Raw{Value: "NOW()"})},
		)
		cvs.Compile(defaultTemplate)
	}
}
