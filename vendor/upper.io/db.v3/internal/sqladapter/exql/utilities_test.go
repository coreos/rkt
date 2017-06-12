package exql

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

const (
	blankSymbol         = ' '
	stringWithCommas    = "Hello,,World!,Enjoy"
	stringWithSpaces    = " Hello  World! Enjoy"
	stringWithASKeyword = "table.Name AS myTableAlias"
)

var (
	bytesWithLeadingBlanks  = []byte("               Hello world!             ")
	stringWithLeadingBlanks = string(bytesWithLeadingBlanks)
)

func TestUtilIsBlankSymbol(t *testing.T) {
	if isBlankSymbol(' ') == false {
		t.Fail()
	}
	if isBlankSymbol('\n') == false {
		t.Fail()
	}
	if isBlankSymbol('\t') == false {
		t.Fail()
	}
	if isBlankSymbol('\r') == false {
		t.Fail()
	}
	if isBlankSymbol('x') == true {
		t.Fail()
	}
}

func TestUtilTrimBytes(t *testing.T) {
	var trimmed []byte

	trimmed = trimBytes([]byte("  \t\nHello World!     \n"))
	if string(trimmed) != "Hello World!" {
		t.Fatalf("Got: %s\n", string(trimmed))
	}

	trimmed = trimBytes([]byte("Nope"))
	if string(trimmed) != "Nope" {
		t.Fatalf("Got: %s\n", string(trimmed))
	}

	trimmed = trimBytes([]byte(""))
	if string(trimmed) != "" {
		t.Fatalf("Got: %s\n", string(trimmed))
	}

	trimmed = trimBytes([]byte(" "))
	if string(trimmed) != "" {
		t.Fatalf("Got: %s\n", string(trimmed))
	}

	trimmed = trimBytes(nil)
	if string(trimmed) != "" {
		t.Fatalf("Got: %s\n", string(trimmed))
	}
}

func TestUtilSeparateByComma(t *testing.T) {
	chunks := separateByComma("Hello,,World!,Enjoy")

	if len(chunks) != 4 {
		t.Fatal()
	}

	if chunks[0] != "Hello" {
		t.Fatal()
	}
	if chunks[1] != "" {
		t.Fatal()
	}
	if chunks[2] != "World!" {
		t.Fatal()
	}
	if chunks[3] != "Enjoy" {
		t.Fatal()
	}
}

func TestUtilSeparateBySpace(t *testing.T) {
	chunks := separateBySpace("       Hello        World!        Enjoy")

	if len(chunks) != 3 {
		t.Fatal()
	}

	if chunks[0] != "Hello" {
		t.Fatal()
	}
	if chunks[1] != "World!" {
		t.Fatal()
	}
	if chunks[2] != "Enjoy" {
		t.Fatal()
	}
}

func TestUtilSeparateByAS(t *testing.T) {
	var chunks []string

	var tests = []string{
		`table.Name AS myTableAlias`,
		`table.Name     AS         myTableAlias`,
		"table.Name\tAS\r\nmyTableAlias",
	}

	for _, test := range tests {
		chunks = separateByAS(test)

		if len(chunks) != 2 {
			t.Fatalf(`Expecting 2 results.`)
		}

		if chunks[0] != "table.Name" {
			t.Fatal(`Expecting first result to be "table.Name".`)
		}
		if chunks[1] != "myTableAlias" {
			t.Fatal(`Expecting second result to be myTableAlias.`)
		}
	}

	// Single character.
	chunks = separateByAS("a")

	if len(chunks) != 1 {
		t.Fatalf(`Expecting 1 results.`)
	}

	if chunks[0] != "a" {
		t.Fatal(`Expecting first result to be "a".`)
	}

	// Empty name
	chunks = separateByAS("")

	if len(chunks) != 1 {
		t.Fatalf(`Expecting 1 results.`)
	}

	if chunks[0] != "" {
		t.Fatal(`Expecting first result to be "".`)
	}

	// Single name
	chunks = separateByAS("  A Single Table ")

	if len(chunks) != 1 {
		t.Fatalf(`Expecting 1 results.`)
	}

	if chunks[0] != "A Single Table" {
		t.Fatal(`Expecting first result to be "ASingleTable".`)
	}

	// Minimal expression.
	chunks = separateByAS("a AS b")

	if len(chunks) != 2 {
		t.Fatalf(`Expecting 2 results.`)
	}

	if chunks[0] != "a" {
		t.Fatal(`Expecting first result to be "a".`)
	}

	if chunks[1] != "b" {
		t.Fatal(`Expecting first result to be "b".`)
	}

	// Minimal expression with spaces.
	chunks = separateByAS("   a    AS    b ")

	if len(chunks) != 2 {
		t.Fatalf(`Expecting 2 results.`)
	}

	if chunks[0] != "a" {
		t.Fatal(`Expecting first result to be "a".`)
	}

	if chunks[1] != "b" {
		t.Fatal(`Expecting first result to be "b".`)
	}

	// Minimal expression + 1 with spaces.
	chunks = separateByAS("   a    AS    bb ")

	if len(chunks) != 2 {
		t.Fatalf(`Expecting 2 results.`)
	}

	if chunks[0] != "a" {
		t.Fatal(`Expecting first result to be "a".`)
	}

	if chunks[1] != "bb" {
		t.Fatal(`Expecting first result to be "bb".`)
	}
}

func BenchmarkUtilIsBlankSymbol(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = isBlankSymbol(blankSymbol)
	}
}

func BenchmarkUtilStdlibIsBlankSymbol(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = unicode.IsSpace(blankSymbol)
	}
}

func BenchmarkUtilTrimBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = trimBytes(bytesWithLeadingBlanks)
	}
}
func BenchmarkUtilStdlibBytesTrimSpace(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = bytes.TrimSpace(bytesWithLeadingBlanks)
	}
}

func BenchmarkUtilTrimString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = trimString(stringWithLeadingBlanks)
	}
}

func BenchmarkUtilStdlibStringsTrimSpace(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strings.TrimSpace(stringWithLeadingBlanks)
	}
}

func BenchmarkUtilSeparateByComma(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = separateByComma(stringWithCommas)
	}
}

func BenchmarkUtilRegExpSeparateByComma(b *testing.B) {
	sep := regexp.MustCompile(`\s*?,\s*?`)
	for i := 0; i < b.N; i++ {
		_ = sep.Split(stringWithCommas, -1)
	}
}

func BenchmarkUtilSeparateBySpace(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = separateBySpace(stringWithSpaces)
	}
}

func BenchmarkUtilRegExpSeparateBySpace(b *testing.B) {
	sep := regexp.MustCompile(`\s+`)
	for i := 0; i < b.N; i++ {
		_ = sep.Split(stringWithSpaces, -1)
	}
}

func BenchmarkUtilSeparateByAS(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = separateByAS(stringWithASKeyword)
	}
}

func BenchmarkUtilRegExpSeparateByAS(b *testing.B) {
	sep := regexp.MustCompile(`(?i:\s+AS\s+)`)
	for i := 0; i < b.N; i++ {
		_ = sep.Split(stringWithASKeyword, -1)
	}
}
