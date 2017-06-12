package sqlbuilder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"upper.io/db.v3"
)

func TestPlaceholderSimple(t *testing.T) {
	{
		ret, _ := Preprocess("?", []interface{}{1})
		assert.Equal(t, "?", ret)
	}
	{
		ret, _ := Preprocess("?", nil)
		assert.Equal(t, "?", ret)
	}
}

func TestPlaceholderMany(t *testing.T) {
	{
		ret, _ := Preprocess("?, ?, ?", []interface{}{1, 2, 3})
		assert.Equal(t, "?, ?, ?", ret)
	}
}

func TestPlaceholderArray(t *testing.T) {
	{
		ret, _ := Preprocess("?, ?, ?", []interface{}{1, 2, []interface{}{3, 4, 5}})
		assert.Equal(t, "?, ?, (?, ?, ?)", ret)
	}

	{
		ret, _ := Preprocess("?, ?, ?", []interface{}{[]interface{}{1, 2, 3}, 4, 5})
		assert.Equal(t, "(?, ?, ?), ?, ?", ret)
	}

	{
		ret, _ := Preprocess("?, ?, ?", []interface{}{1, []interface{}{2, 3, 4}, 5})
		assert.Equal(t, "?, (?, ?, ?), ?", ret)
	}

	{
		ret, _ := Preprocess("???", []interface{}{1, []interface{}{2, 3, 4}, 5})
		assert.Equal(t, "?(?, ?, ?)?", ret)
	}

	{
		ret, _ := Preprocess("??", []interface{}{[]interface{}{1, 2, 3}, []interface{}{}, []interface{}{4, 5}, []interface{}{}})
		assert.Equal(t, "(?, ?, ?)(NULL)", ret)
	}
}

func TestPlaceholderArguments(t *testing.T) {
	{
		_, args := Preprocess("?, ?, ?", []interface{}{1, 2, []interface{}{3, 4, 5}})
		assert.Equal(t, []interface{}{1, 2, 3, 4, 5}, args)
	}

	{
		_, args := Preprocess("?, ?, ?", []interface{}{1, []interface{}{2, 3, 4}, 5})
		assert.Equal(t, []interface{}{1, 2, 3, 4, 5}, args)
	}

	{
		_, args := Preprocess("?, ?, ?", []interface{}{[]interface{}{1, 2, 3}, 4, 5})
		assert.Equal(t, []interface{}{1, 2, 3, 4, 5}, args)
	}

	{
		_, args := Preprocess("?, ?", []interface{}{[]interface{}{1, 2, 3}, []interface{}{4, 5}})
		assert.Equal(t, []interface{}{1, 2, 3, 4, 5}, args)
	}
}

func TestPlaceholderReplace(t *testing.T) {
	{
		ret, args := Preprocess("?, ?, ?", []interface{}{1, db.Raw("foo"), 3})
		assert.Equal(t, "?, foo, ?", ret)
		assert.Equal(t, []interface{}{1, 3}, args)
	}
}
