package db

import (
	"testing"
)

func TestCond(t *testing.T) {
	c := Cond{}

	if !c.Empty() {
		t.Fatal("Cond is empty.")
	}

	c = Cond{"id": 1}
	if c.Empty() {
		t.Fatal("Cond is not empty.")
	}
}

func TestCondAnd(t *testing.T) {
	a := And()

	if !a.Empty() {
		t.Fatal("Cond is empty")
	}

	_ = a.And(Cond{"id": 1})

	if !a.Empty() {
		t.Fatal("Cond is still empty")
	}

	a = a.And(Cond{"name": "Ana"})

	if a.Empty() {
		t.Fatal("Cond is not empty anymore")
	}

	a = a.And().And()

	if a.Empty() {
		t.Fatal("Cond is not empty anymore")
	}
}

func TestCondOr(t *testing.T) {
	a := Or()

	if !a.Empty() {
		t.Fatal("Cond is empty")
	}

	_ = a.Or(Cond{"id": 1})

	if !a.Empty() {
		t.Fatal("Cond is empty")
	}

	a = a.Or(Cond{"name": "Ana"})

	if a.Empty() {
		t.Fatal("Cond is not empty")
	}

	a = a.Or().Or()
	if a.Empty() {
		t.Fatal("Cond is not empty")
	}
}
