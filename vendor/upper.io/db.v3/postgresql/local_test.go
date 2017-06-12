package postgresql

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"upper.io/db.v3"
)

func TestStringAndInt64Array(t *testing.T) {
	sess := mustOpen()
	driver := sess.Driver().(*sql.DB)

	defer func() {
		driver.Exec(`DROP TABLE IF EXISTS array_types`)
		sess.Close()
	}()

	if _, err := driver.Exec(`
		CREATE TABLE array_types (
			id serial primary key,
			integers bigint[] DEFAULT NULL,
			strings varchar(64)[]
		)`); err != nil {
		assert.NoError(t, err)
	}

	arrayTypes := sess.Collection("array_types")
	err := arrayTypes.Truncate()
	assert.NoError(t, err)

	type arrayType struct {
		ID       int64    `db:"id,pk"`
		Integers []int64  `db:"integers,int64array"`
		Strings  []string `db:"strings,stringarray"`
	}

	tt := []arrayType{
		// Test nil arrays.
		arrayType{
			ID:       1,
			Integers: nil,
			Strings:  nil,
		},

		// Test empty arrays.
		arrayType{
			ID:       2,
			Integers: []int64{},
			Strings:  []string{},
		},

		// Test non-empty arrays.
		arrayType{
			ID:       3,
			Integers: []int64{1, 2, 3},
			Strings:  []string{"1", "2", "3"},
		},
	}

	for _, item := range tt {
		id, err := arrayTypes.Insert(item)
		assert.NoError(t, err)

		if pk, ok := id.(int64); !ok || pk == 0 {
			t.Fatalf("Expecting an ID.")
		}

		var itemCheck arrayType
		err = arrayTypes.Find(db.Cond{"id": id}).One(&itemCheck)
		assert.NoError(t, err)
		assert.Len(t, itemCheck.Integers, len(item.Integers))
		assert.Len(t, itemCheck.Strings, len(item.Strings))

		assert.Equal(t, item, itemCheck)
	}
}

func TestIssue210(t *testing.T) {
	list := []string{
		`DROP TABLE IF EXISTS testing123`,
		`DROP TABLE IF EXISTS hello`,
		`CREATE TABLE IF NOT EXISTS testing123 (
			ID INT PRIMARY KEY     NOT NULL,
			NAME           TEXT    NOT NULL
		)
		`,
		`CREATE TABLE IF NOT EXISTS hello (
			ID INT PRIMARY KEY     NOT NULL,
			NAME           TEXT    NOT NULL
		)`,
	}

	sess := mustOpen()
	defer sess.Close()

	tx, err := sess.NewTx(nil)
	assert.NoError(t, err)

	for i := range list {
		_, err = tx.Exec(list[i])
		assert.NoError(t, err)
	}

	err = tx.Commit()
	assert.NoError(t, err)

	_, err = sess.Collection("testing123").Find().Count()
	assert.NoError(t, err)

	_, err = sess.Collection("hello").Find().Count()
	assert.NoError(t, err)
}

func TestPreparedStatements(t *testing.T) {
	sess := mustOpen()
	defer sess.Close()

	var val int

	{
		stmt, err := sess.Prepare(`SELECT 1`)
		assert.NoError(t, err)
		assert.NotNil(t, stmt)

		q, err := stmt.Query()
		assert.NoError(t, err)
		assert.NotNil(t, q)
		assert.True(t, q.Next())

		err = q.Scan(&val)
		assert.NoError(t, err)

		err = q.Close()
		assert.NoError(t, err)

		assert.Equal(t, 1, val)

		err = stmt.Close()
		assert.NoError(t, err)
	}

	{
		tx, err := sess.NewTx(nil)
		assert.NoError(t, err)

		stmt, err := tx.Prepare(`SELECT 2`)
		assert.NoError(t, err)
		assert.NotNil(t, stmt)

		q, err := stmt.Query()
		assert.NoError(t, err)
		assert.NotNil(t, q)
		assert.True(t, q.Next())

		err = q.Scan(&val)
		assert.NoError(t, err)

		err = q.Close()
		assert.NoError(t, err)

		assert.Equal(t, 2, val)

		err = stmt.Close()
		assert.NoError(t, err)

		err = tx.Commit()
		assert.NoError(t, err)
	}

	{
		stmt, err := sess.Select(3).Prepare()
		assert.NoError(t, err)
		assert.NotNil(t, stmt)

		q, err := stmt.Query()
		assert.NoError(t, err)
		assert.NotNil(t, q)
		assert.True(t, q.Next())

		err = q.Scan(&val)
		assert.NoError(t, err)

		err = q.Close()
		assert.NoError(t, err)

		assert.Equal(t, 3, val)

		err = stmt.Close()
		assert.NoError(t, err)
	}
}

func TestNonTrivialSubqueries(t *testing.T) {
	sess := mustOpen()
	defer sess.Close()

	{
		q, err := sess.Query(`WITH test AS (?) ?`,
			sess.Select("id AS foo").From("artist"),
			sess.Select("foo").From("test").Where("foo > ?", 0),
		)

		assert.NoError(t, err)
		assert.NotNil(t, q)

		assert.True(t, q.Next())

		var number int
		assert.NoError(t, q.Scan(&number))

		assert.Equal(t, 1, number)
		assert.NoError(t, q.Close())
	}

	{
		row, err := sess.QueryRow(`WITH test AS (?) ?`,
			sess.Select("id AS foo").From("artist"),
			sess.Select("foo").From("test").Where("foo > ?", 0),
		)

		assert.NoError(t, err)
		assert.NotNil(t, row)

		var number int
		assert.NoError(t, row.Scan(&number))

		assert.Equal(t, 1, number)
	}

	{
		res, err := sess.Exec(`UPDATE artist a1 SET id = ?`,
			sess.Select(db.Raw("id + 1")).From("artist a2").Where("a2.id = a1.id"),
		)

		assert.NoError(t, err)
		assert.NotNil(t, res)
	}

	{
		q, err := sess.Query(db.Raw(`WITH test AS (?) ?`,
			sess.Select("id AS foo").From("artist"),
			sess.Select("foo").From("test").Where("foo > ?", 0),
		))

		assert.NoError(t, err)
		assert.NotNil(t, q)

		assert.True(t, q.Next())

		var number int
		assert.NoError(t, q.Scan(&number))

		assert.Equal(t, 2, number)
		assert.NoError(t, q.Close())
	}

	{
		row, err := sess.QueryRow(db.Raw(`WITH test AS (?) ?`,
			sess.Select("id AS foo").From("artist"),
			sess.Select("foo").From("test").Where("foo > ?", 0),
		))

		assert.NoError(t, err)
		assert.NotNil(t, row)

		var number int
		assert.NoError(t, row.Scan(&number))

		assert.Equal(t, 2, number)
	}

	{
		res, err := sess.Exec(db.Raw(`UPDATE artist a1 SET id = ?`,
			sess.Select(db.Raw("id + 1")).From("artist a2").Where("a2.id = a1.id"),
		))

		assert.NoError(t, err)
		assert.NotNil(t, res)
	}
}
