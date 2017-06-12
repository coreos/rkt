// +build generated

package ADAPTER

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"upper.io/db.v3"
	"upper.io/db.v3/lib/sqlbuilder"
)

type customLogger struct {
}

func (*customLogger) Log(q *db.QueryStatus) {
	switch q.Err {
	case nil, db.ErrNoMoreRows:
		return // Don't log successful queries.
	}
	// Alert of any other error.
	log.Printf("Unexpected database error: %v\n%s", q.Err, q.String())
}

type artistType struct {
	ID   int64  `db:"id,omitempty"`
	Name string `db:"name"`
}

type itemWithCompoundKey struct {
	Code    string `db:"code"`
	UserID  string `db:"user_id"`
	SomeVal string `db:"some_val"`
}

type customType struct {
	Val []byte
}

type artistWithCustomType struct {
	Custom customType `db:"name"`
}

func (f customType) String() string {
	return fmt.Sprintf("foo: %s", string(f.Val))
}

func (f customType) MarshalDB() (interface{}, error) {
	return f.String(), nil
}

func (f *customType) UnmarshalDB(in interface{}) error {
	switch t := in.(type) {
	case []byte:
		f.Val = t
	case string:
		f.Val = []byte(t)
	}
	return nil
}

var (
	_ = db.Marshaler(&customType{})
	_ = db.Unmarshaler(&customType{})
)

func TestMain(m *testing.M) {
	flag.Parse()

	if err := tearUp(); err != nil {
		log.Fatal("tearUp", err)
	}

	os.Exit(m.Run())
}

func mustOpen() sqlbuilder.Database {
	sess, err := Open(settings)
	if err != nil {
		panic(err.Error())
	}
	return sess
}

func TestOpenMustSucceed(t *testing.T) {
	sess, err := Open(settings)
	assert.NoError(t, err)
	assert.NotNil(t, sess)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestPreparedStatementsCache(t *testing.T) {
	sess := mustOpen()

	sess.SetPreparedStatementCache(true)
	defer sess.SetPreparedStatementCache(false)

	var tMu sync.Mutex
	tFatal := func(err error) {
		tMu.Lock()
		defer tMu.Unlock()
		t.Fatal(err)
	}

	// This limit was chosen because, by default, MySQL accepts 16k statements
	// and dies. See https://github.com/upper/db/issues/287
	limit := 20000
	var wg sync.WaitGroup

	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// This query is different with each iteration and thus generates a new
			// prepared statement everytime it's called.
			res := sess.Collection("artist").Find().Select(db.Raw(fmt.Sprintf("count(%d)", i)))
			var count map[string]uint64
			err := res.One(&count)
			if err != nil {
				tFatal(err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent Insert can open many connections on MySQL / PostgreSQL, this
	// sets a limit on them.
	sess.SetMaxOpenConns(100)

	switch Adapter {
	case "ql":
		limit = 1000
	case "sqlite":
		// TODO: We'll probably be able to workaround this with a mutex on inserts.
		t.Skip(`Skipped due to a "database is locked" problem with concurrent transactions. See https://github.com/mattn/go-sqlite3/issues/274`)
	}

	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// The same prepared query on every iteration.
			_, err := sess.Collection("artist").Insert(artistType{
				Name: fmt.Sprintf("artist-%d", i),
			})
			if err != nil {
				tFatal(err)
			}
		}(i)
	}
	wg.Wait()

	// Insert returning creates a transaction.
	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// The same prepared query on every iteration.
			artist := artistType{
				Name: fmt.Sprintf("artist-%d", i),
			}
			err := sess.Collection("artist").InsertReturning(&artist)
			if err != nil {
				tFatal(err)
			}
		}(i)
	}
	wg.Wait()

	// Removing the limit.
	sess.SetMaxOpenConns(0)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestTruncateAllCollections(t *testing.T) {
	sess := mustOpen()

	collections, err := sess.Collections()
	assert.NoError(t, err)
	assert.True(t, len(collections) > 0)

	for _, name := range collections {
		col := sess.Collection(name)

		if col.Exists() {
			if err = col.Truncate(); err != nil {
				assert.NoError(t, err)
			}
		}
	}

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestCustomQueryLogger(t *testing.T) {
	sess := mustOpen()

	sess.SetLogger(&customLogger{})
	defer func() {
		sess.SetLogger(nil)
	}()

	_, err := sess.Collection("artist").Find().Count()
	assert.Equal(t, nil, err)

	_, err = sess.Collection("artist_x").Find().Count()
	assert.NotEqual(t, nil, err)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestExpectCursorError(t *testing.T) {
	sess := mustOpen()

	artist := sess.Collection("artist")

	res := artist.Find(-1)
	c, err := res.Count()
	assert.Equal(t, uint64(0), c)
	assert.NoError(t, err)

	var item map[string]interface{}
	err = res.One(&item)
	assert.Error(t, err)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestInsertDefault(t *testing.T) {
	if Adapter == "ql" {
		t.Skip("Currently not supported.")
	}

	sess := mustOpen()

	artist := sess.Collection("artist")

	err := artist.Truncate()
	assert.NoError(t, err)

	id, err := artist.Insert(&artistType{})
	assert.NoError(t, err)
	assert.NotNil(t, id)

	err = artist.Truncate()
	assert.NoError(t, err)

	id, err = artist.Insert(nil)
	assert.NoError(t, err)
	assert.NotNil(t, id)
}

func TestInsertReturning(t *testing.T) {
	sess := mustOpen()

	artist := sess.Collection("artist")

	err := artist.Truncate()
	assert.NoError(t, err)

	itemMap := map[string]string{
		"name": "Ozzie",
	}
	assert.Zero(t, itemMap["id"], "Must be zero before inserting")
	err = artist.InsertReturning(&itemMap)
	assert.NoError(t, err)
	assert.NotZero(t, itemMap["id"], "Must not be zero after inserting")

	itemStruct := struct {
		ID   int    `db:"id,omitempty"`
		Name string `db:"name"`
	}{
		0,
		"Flea",
	}
	assert.Zero(t, itemStruct.ID, "Must be zero before inserting")
	err = artist.InsertReturning(&itemStruct)
	assert.NoError(t, err)
	assert.NotZero(t, itemStruct.ID, "Must not be zero after inserting")

	count, err := artist.Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), count, "Expecting 2 elements")

	itemStruct2 := struct {
		ID   int    `db:"id,omitempty"`
		Name string `db:"name"`
	}{
		0,
		"Slash",
	}
	assert.Zero(t, itemStruct2.ID, "Must be zero before inserting")
	err = artist.InsertReturning(itemStruct2)
	assert.Error(t, err, "Should not happen, using a pointer should be enforced")
	assert.Zero(t, itemStruct2.ID, "Must still be zero because there was no insertion")

	itemMap2 := map[string]string{
		"name": "Janus",
	}
	assert.Zero(t, itemMap2["id"], "Must be zero before inserting")
	err = artist.InsertReturning(itemMap2)
	assert.Error(t, err, "Should not happen, using a pointer should be enforced")
	assert.Zero(t, itemMap2["id"], "Must still be zero because there was no insertion")

	// Counting elements, must be exactly 2 elements.
	count, err = artist.Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), count, "Expecting 2 elements")

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestInsertReturningWithinTransaction(t *testing.T) {
	sess := mustOpen()

	err := sess.Collection("artist").Truncate()
	assert.NoError(t, err)

	tx, err := sess.NewTx(nil)
	assert.NoError(t, err)
	defer tx.Close()

	artist := tx.Collection("artist")

	itemMap := map[string]string{
		"name": "Ozzie",
	}
	assert.Zero(t, itemMap["id"], "Must be zero before inserting")
	err = artist.InsertReturning(&itemMap)
	assert.NoError(t, err)
	assert.NotZero(t, itemMap["id"], "Must not be zero after inserting")

	itemStruct := struct {
		ID   int    `db:"id,omitempty"`
		Name string `db:"name"`
	}{
		0,
		"Flea",
	}
	assert.Zero(t, itemStruct.ID, "Must be zero before inserting")
	err = artist.InsertReturning(&itemStruct)
	assert.NoError(t, err)
	assert.NotZero(t, itemStruct.ID, "Must not be zero after inserting")

	count, err := artist.Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), count, "Expecting 2 elements")

	itemStruct2 := struct {
		ID   int    `db:"id,omitempty"`
		Name string `db:"name"`
	}{
		0,
		"Slash",
	}
	assert.Zero(t, itemStruct2.ID, "Must be zero before inserting")
	err = artist.InsertReturning(itemStruct2)
	assert.Error(t, err, "Should not happen, using a pointer should be enforced")
	assert.Zero(t, itemStruct2.ID, "Must still be zero because there was no insertion")

	itemMap2 := map[string]string{
		"name": "Janus",
	}
	assert.Zero(t, itemMap2["id"], "Must be zero before inserting")
	err = artist.InsertReturning(itemMap2)
	assert.Error(t, err, "Should not happen, using a pointer should be enforced")
	assert.Zero(t, itemMap2["id"], "Must still be zero because there was no insertion")

	// Counting elements, must be exactly 2 elements.
	count, err = artist.Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), count, "Expecting 2 elements")

	// Rolling back everything
	err = tx.Rollback()
	assert.NoError(t, err)

	// Expecting no elements.
	count, err = sess.Collection("artist").Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), count, "Expecting 0 elements, everything was rolled back!")

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestInsertIntoArtistsTable(t *testing.T) {
	sess := mustOpen()

	artist := sess.Collection("artist")

	err := artist.Truncate()
	assert.NoError(t, err)

	itemMap := map[string]string{
		"name": "Ozzie",
	}

	id, err := artist.Insert(itemMap)
	assert.NoError(t, err)
	assert.NotNil(t, id)

	if pk, ok := id.(int64); !ok || pk == 0 {
		t.Fatalf("Expecting an ID.")
	}

	// Attempt to append a struct.
	itemStruct := struct {
		Name string `db:"name"`
	}{
		"Flea",
	}

	id, err = artist.Insert(itemStruct)
	assert.NoError(t, err)
	assert.NotNil(t, id)

	if pk, ok := id.(int64); !ok || pk == 0 {
		t.Fatalf("Expecting an ID.")
	}

	// Attempt to append a tagged struct.
	itemStruct2 := struct {
		ArtistName string `db:"name"`
	}{
		"Slash",
	}

	id, err = artist.Insert(itemStruct2)
	assert.NoError(t, err)
	assert.NotNil(t, id)

	if pk, ok := id.(int64); !ok || pk == 0 {
		t.Fatalf("Expecting an ID.")
	}

	// Attempt to append and update a private key
	itemStruct3 := artistType{
		Name: "Janus",
	}

	id, err = artist.Insert(&itemStruct3)
	assert.NoError(t, err)
	if Adapter != "ql" {
		assert.NotZero(t, id) // QL always inserts an ID.
	}

	// Counting elements, must be exactly 4 elements.
	count, err := artist.Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(4), count)

	count, err = artist.Find(db.Cond{"name": "Ozzie"}).Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), count)

	count, err = artist.Find("name", "Ozzie").And("name", "Flea").Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), count)

	count, err = artist.Find(db.Or(db.Cond{"name": "Ozzie"}, db.Cond{"name": "Flea"})).Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), count)

	count, err = artist.Find(db.And(db.Cond{"name": "Ozzie"}, db.Cond{"name": "Flea"})).Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), count)

	count, err = artist.Find(db.Cond{"name": "Ozzie"}).And(db.Cond{"name": "Flea"}).Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), count)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestQueryNonExistentCollection(t *testing.T) {
	sess := mustOpen()

	count, err := sess.Collection("doesnotexist").Find().Count()
	assert.Error(t, err)
	assert.Zero(t, count)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestGetOneResult(t *testing.T) {
	sess := mustOpen()

	artist := sess.Collection("artist")

	// Fetching one struct.
	var someArtist artistType
	err := artist.Find().Limit(1).One(&someArtist)
	assert.NoError(t, err)

	assert.NotZero(t, someArtist.Name)
	if Adapter != "ql" {
		assert.NotZero(t, someArtist.ID)
	}

	// Fetching a pointer to a pointer.
	var someArtistObj *artistType
	err = artist.Find().Limit(1).One(&someArtistObj)
	assert.NoError(t, err)
	assert.NotZero(t, someArtist.Name)
	if Adapter != "ql" {
		assert.NotZero(t, someArtist.ID)
	}

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestGetResultsOneByOne(t *testing.T) {
	sess := mustOpen()

	artist := sess.Collection("artist")

	rowMap := map[string]interface{}{}

	res := artist.Find()

	if Adapter == "ql" {
		res = res.Select("id() as id", "name")
	}

	err := res.Err()
	assert.NoError(t, err)

	for res.Next(&rowMap) {
		assert.NotZero(t, rowMap["id"])
		assert.NotZero(t, rowMap["name"])
	}
	err = res.Err()
	assert.NoError(t, err)

	err = res.Close()
	assert.NoError(t, err)

	// Dumping into a tagged struct.
	rowStruct2 := struct {
		Value1 int64  `db:"id"`
		Value2 string `db:"name"`
	}{}

	res = artist.Find()

	if Adapter == "ql" {
		res = res.Select("id() as id", "name")
	}

	for res.Next(&rowStruct2) {
		assert.NotZero(t, rowStruct2.Value1)
		assert.NotZero(t, rowStruct2.Value2)
	}
	err = res.Err()
	assert.NoError(t, err)

	err = res.Close()
	assert.NoError(t, err)

	// Dumping into a slice of maps.
	allRowsMap := []map[string]interface{}{}

	res = artist.Find()
	if Adapter == "ql" {
		res.Select("id() as id")
	}

	err = res.All(&allRowsMap)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(allRowsMap))

	for _, singleRowMap := range allRowsMap {
		if fmt.Sprintf("%d", singleRowMap["id"]) == "0" {
			t.Fatalf("Expecting a not null ID.")
		}
	}

	// Dumping into a slice of structs.
	allRowsStruct := []struct {
		ID   int64  `db:"id,omitempty"`
		Name string `db:"name"`
	}{}

	res = artist.Find()
	if Adapter == "ql" {
		res.Select("id() as id")
	}

	if err = res.All(&allRowsStruct); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 4, len(allRowsStruct))

	for _, singleRowStruct := range allRowsStruct {
		assert.NotZero(t, singleRowStruct.ID)
	}

	// Dumping into a slice of tagged structs.
	allRowsStruct2 := []struct {
		Value1 int64  `db:"id"`
		Value2 string `db:"name"`
	}{}

	res = artist.Find()
	if Adapter == "ql" {
		res.Select("id() as id", "name")
	}

	err = res.All(&allRowsStruct2)
	assert.NoError(t, err)

	assert.Equal(t, 4, len(allRowsStruct2))

	for _, singleRowStruct := range allRowsStruct2 {
		assert.NotZero(t, singleRowStruct.Value1)
	}

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestGetAllResults(t *testing.T) {
	sess := mustOpen()

	artist := sess.Collection("artist")

	// Fetching all artists into struct
	artists := []artistType{}

	res := artist.Find()
	if Adapter == "ql" {
		res.Select("id() as id", "name")
	}

	err := res.All(&artists)
	assert.NoError(t, err)
	assert.NotZero(t, len(artists))

	assert.NotZero(t, artists[0].Name)
	assert.NotZero(t, artists[0].ID)

	// Fetching all artists into struct objects
	artistObjs := []*artistType{}
	res = artist.Find()
	if Adapter == "ql" {
		res.Select("id() as id", "name")
	}
	err = res.All(&artistObjs)
	assert.NoError(t, err)
	assert.NotZero(t, len(artistObjs))
	assert.NotZero(t, artistObjs[0].Name)
	assert.NotZero(t, artistObjs[0].ID)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestInlineStructs(t *testing.T) {
	type reviewTypeDetails struct {
		Name     string    `db:"name"`
		Comments string    `db:"comments"`
		Created  time.Time `db:"created"`
	}

	type reviewType struct {
		ID            int64             `db:"id,omitempty"`
		PublicationID int64             `db:"publication_id"`
		Details       reviewTypeDetails `db:",inline"`
	}

	sess := mustOpen()

	review := sess.Collection("review")

	err := review.Truncate()
	assert.NoError(t, err)

	rec := reviewType{
		PublicationID: 123,
		Details: reviewTypeDetails{
			Name:     "..name..",
			Comments: "..comments..",
		},
	}

	if Adapter == "postgresql" {
		rec.Details.Created = time.Date(2016, time.January, 1, 2, 3, 4, 0, time.FixedZone("", 0))
	} else {
		rec.Details.Created = time.Date(2016, time.January, 1, 2, 3, 4, 0, time.UTC)
	}

	id, err := review.Insert(rec)
	assert.NoError(t, err)
	assert.NotZero(t, id.(int64))

	rec.ID = id.(int64)

	var recChk reviewType
	res := review.Find()
	if Adapter == "ql" {
		res.Select("id() as id", "publication_id", "comments", "name", "created")
	}
	err = res.One(&recChk)
	assert.NoError(t, err)

	assert.Equal(t, rec, recChk)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestUpdate(t *testing.T) {
	sess := mustOpen()

	artist := sess.Collection("artist")

	// Defining destination struct
	value := struct {
		ID   int64  `db:"id,omitempty"`
		Name string `db:"name"`
	}{}

	// Getting the first artist.
	cond := db.Cond{"id !=": 0}
	if Adapter == "ql" {
		cond = db.Cond{"id() !=": 0}
	}
	res := artist.Find(cond).Limit(1)

	err := res.One(&value)
	assert.NoError(t, err)

	res = artist.Find(value.ID)

	// Updating set with a map
	rowMap := map[string]interface{}{
		"name": strings.ToUpper(value.Name),
	}

	err = res.Update(rowMap)
	assert.NoError(t, err)

	// Pulling it again.
	err = res.One(&value)
	assert.NoError(t, err)

	// Verifying.
	assert.Equal(t, value.Name, rowMap["name"])

	if Adapter != "ql" {

		// Updating using raw
		if err = res.Update(map[string]interface{}{"name": db.Raw("LOWER(name)")}); err != nil {
			t.Fatal(err)
		}

		// Pulling it again.
		err = res.One(&value)
		assert.NoError(t, err)

		// Verifying.
		assert.Equal(t, value.Name, strings.ToLower(rowMap["name"].(string)))

		// Updating using raw
		if err = res.Update(struct {
			Name db.RawValue `db:"name"`
		}{db.Raw(`UPPER(name)`)}); err != nil {
			t.Fatal(err)
		}

		// Pulling it again.
		err = res.One(&value)
		assert.NoError(t, err)

		// Verifying.
		assert.Equal(t, value.Name, strings.ToUpper(rowMap["name"].(string)))

		// Updating using raw
		if err = res.Update(struct {
			Name db.Function `db:"name"`
		}{db.Func("LOWER", db.Raw("name"))}); err != nil {
			t.Fatal(err)
		}

		// Pulling it again.
		err = res.One(&value)
		assert.NoError(t, err)

		// Verifying.
		assert.Equal(t, value.Name, strings.ToLower(rowMap["name"].(string)))
	}

	// Updating set with a struct
	rowStruct := struct {
		Name string `db:"name"`
	}{strings.ToLower(value.Name)}

	err = res.Update(rowStruct)
	assert.NoError(t, err)

	// Pulling it again.
	err = res.One(&value)
	assert.NoError(t, err)

	// Verifying
	assert.Equal(t, value.Name, rowStruct.Name)

	// Updating set with a tagged struct
	rowStruct2 := struct {
		Value1 string `db:"name"`
	}{"john"}

	err = res.Update(rowStruct2)
	assert.NoError(t, err)

	// Pulling it again.
	err = res.One(&value)
	assert.NoError(t, err)

	// Verifying
	assert.Equal(t, value.Name, rowStruct2.Value1)

	// Updating set with a tagged object
	rowStruct3 := &struct {
		Value1 string `db:"name"`
	}{"anderson"}

	err = res.Update(rowStruct3)
	assert.NoError(t, err)

	// Pulling it again.
	err = res.One(&value)
	assert.NoError(t, err)

	// Verifying
	assert.Equal(t, value.Name, rowStruct3.Value1)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestFunction(t *testing.T) {
	sess := mustOpen()

	rowStruct := struct {
		ID   int64
		Name string
	}{}

	artist := sess.Collection("artist")
	cond := db.Cond{"id NOT IN": []int{0, -1}}
	if Adapter == "ql" {
		cond = db.Cond{"id() NOT IN": []int{0, -1}}
	}
	res := artist.Find(cond)

	err := res.One(&rowStruct)
	assert.NoError(t, err)

	total, err := res.Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(4), total)

	// Testing conditions
	cond = db.Cond{"id NOT IN": []interface{}{0, -1}}
	if Adapter == "ql" {
		cond = db.Cond{"id() NOT IN": []interface{}{0, -1}}
	}
	res = artist.Find(cond)

	err = res.One(&rowStruct)
	assert.NoError(t, err)

	total, err = res.Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(4), total)

	// Testing DISTINCT (function)
	res = artist.Find().Select(
		db.Func("DISTINCT", "name"),
	)

	var rowMap map[string]interface{}
	err = res.One(&rowMap)
	assert.NoError(t, err)

	total, err = res.Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(4), total)

	// Testing DISTINCT (raw)
	res = artist.Find().Select(
		db.Raw("DISTINCT(name)"),
	)

	err = res.One(&rowMap)
	assert.NoError(t, err)

	total, err = res.Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(4), total)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestNullableFields(t *testing.T) {
	sess := mustOpen()

	type testType struct {
		ID              int64           `db:"id,omitempty"`
		NullStringTest  sql.NullString  `db:"_string"`
		NullInt64Test   sql.NullInt64   `db:"_int64"`
		NullFloat64Test sql.NullFloat64 `db:"_float64"`
		NullBoolTest    sql.NullBool    `db:"_bool"`
	}

	col := sess.Collection(`data_types`)

	err := col.Truncate()
	assert.NoError(t, err)

	// Testing insertion of invalid nulls.
	test := testType{
		NullStringTest:  sql.NullString{"", false},
		NullInt64Test:   sql.NullInt64{0, false},
		NullFloat64Test: sql.NullFloat64{0.0, false},
		NullBoolTest:    sql.NullBool{false, false},
	}

	id, err := col.Insert(testType{})
	assert.NoError(t, err)

	// Testing fetching of invalid nulls.
	err = col.Find(id).One(&test)
	assert.NoError(t, err)

	assert.False(t, test.NullInt64Test.Valid)
	assert.False(t, test.NullFloat64Test.Valid)
	assert.False(t, test.NullBoolTest.Valid)

	// Testing insertion of valid nulls.
	test = testType{
		NullStringTest:  sql.NullString{"", true},
		NullInt64Test:   sql.NullInt64{0, true},
		NullFloat64Test: sql.NullFloat64{0.0, true},
		NullBoolTest:    sql.NullBool{false, true},
	}

	id, err = col.Insert(test)
	assert.NoError(t, err)

	// Testing fetching of valid nulls.
	err = col.Find(id).One(&test)
	assert.NoError(t, err)

	assert.True(t, test.NullInt64Test.Valid)
	assert.True(t, test.NullBoolTest.Valid)
	assert.True(t, test.NullStringTest.Valid)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestGroup(t *testing.T) {
	sess := mustOpen()

	type statsType struct {
		Numeric int `db:"numeric"`
		Value   int `db:"value"`
	}

	stats := sess.Collection("stats_test")

	err := stats.Truncate()
	assert.NoError(t, err)

	// Adding row append.
	for i := 0; i < 100; i++ {
		numeric, value := rand.Intn(5), rand.Intn(100)
		_, err := stats.Insert(statsType{numeric, value})
		assert.NoError(t, err)
	}

	// Testing GROUP BY
	res := stats.Find().Select(
		"numeric",
		db.Raw("count(1) AS counter"),
		db.Raw("sum(value) AS total"),
	).Group("numeric")

	var results []map[string]interface{}

	err = res.All(&results)
	assert.NoError(t, err)

	assert.Equal(t, 5, len(results))

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestDelete(t *testing.T) {
	sess := mustOpen()

	artist := sess.Collection("artist")
	res := artist.Find()

	total, err := res.Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(4), total)

	err = res.Delete()
	assert.NoError(t, err)

	total, err = res.Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), total)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestCompositeKeys(t *testing.T) {
	if Adapter == "ql" {
		t.Skip("Currently not supported.")
	}

	sess := mustOpen()

	compositeKeys := sess.Collection("composite_keys")

	n := rand.Intn(100000)

	item := itemWithCompoundKey{
		"ABCDEF",
		strconv.Itoa(n),
		"Some value",
	}

	id, err := compositeKeys.Insert(&item)
	assert.NoError(t, err)
	assert.NotZero(t, id)

	var item2 itemWithCompoundKey
	assert.NotEqual(t, item2.SomeVal, item.SomeVal)

	// Finding by ID
	err = compositeKeys.Find(id).One(&item2)
	assert.NoError(t, err)

	assert.Equal(t, item2.SomeVal, item.SomeVal)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

// Attempts to test database transactions.
func TestTransactionsAndRollback(t *testing.T) {

	if Adapter == "ql" {
		t.Skip("Currently not supported.")
	}

	sess := mustOpen()

	// Simple transaction that should not fail.
	tx, err := sess.NewTx(nil)
	assert.NoError(t, err)

	artist := tx.Collection("artist")
	err = artist.Truncate()
	assert.NoError(t, err)

	_, err = artist.Insert(artistType{1, "First"})
	assert.NoError(t, err)

	err = tx.Commit()
	assert.NoError(t, err)

	// An attempt to use the same transaction must fail.
	err = tx.Commit()
	assert.Error(t, err)

	err = tx.Close()
	assert.NoError(t, err)

	err = tx.Close()
	assert.NoError(t, err)

	// Use another transaction.
	tx, err = sess.NewTx(nil)
	assert.NoError(t, err)

	artist = tx.Collection("artist")

	_, err = artist.Insert(artistType{2, "Second"})
	assert.NoError(t, err)

	// Won't fail.
	_, err = artist.Insert(artistType{3, "Third"})
	assert.NoError(t, err)

	// Will fail.
	_, err = artist.Insert(artistType{1, "Duplicated"})
	assert.Error(t, err)

	err = tx.Rollback()
	assert.NoError(t, err)

	err = tx.Commit()
	assert.Error(t, err, "Already rolled back.")

	// Let's verify we still have one element.
	artist = sess.Collection("artist")

	count, err := artist.Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), count)

	err = tx.Close()
	assert.NoError(t, err)

	// Attempt to add some rows.
	tx, err = sess.NewTx(nil)
	assert.NoError(t, err)

	artist = tx.Collection("artist")

	// Won't fail.
	_, err = artist.Insert(artistType{2, "Second"})
	assert.NoError(t, err)

	// Won't fail.
	_, err = artist.Insert(artistType{3, "Third"})
	assert.NoError(t, err)

	// Then rollback for no reason.
	err = tx.Rollback()
	assert.NoError(t, err)

	err = tx.Commit()
	assert.Error(t, err, "Already rolled back.")

	// Let's verify we still have one element.
	artist = sess.Collection("artist")

	count, err = artist.Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), count)

	err = tx.Close()
	assert.NoError(t, err)

	// Attempt to add some rows.
	tx, err = sess.NewTx(nil)
	assert.NoError(t, err)

	artist = tx.Collection("artist")

	// Won't fail.
	_, err = artist.Insert(artistType{2, "Second"})
	assert.NoError(t, err)

	// Won't fail.
	_, err = artist.Insert(artistType{3, "Third"})
	assert.NoError(t, err)

	err = tx.Commit()
	assert.NoError(t, err)

	err = tx.Rollback()
	assert.Error(t, err, "Already committed")

	// Let's verify we have 3 rows.
	artist = sess.Collection("artist")

	count, err = artist.Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), count)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestDataTypes(t *testing.T) {
	if Adapter == "ql" {
		t.Skip("Currently not supported.")
	}

	type testValuesStruct struct {
		Uint   uint   `db:"_uint"`
		Uint8  uint8  `db:"_uint8"`
		Uint16 uint16 `db:"_uint16"`
		Uint32 uint32 `db:"_uint32"`
		Uint64 uint64 `db:"_uint64"`

		Int   int   `db:"_int"`
		Int8  int8  `db:"_int8"`
		Int16 int16 `db:"_int16"`
		Int32 int32 `db:"_int32"`
		Int64 int64 `db:"_int64"`

		Float32 float32 `db:"_float32"`
		Float64 float64 `db:"_float64"`

		Bool   bool   `db:"_bool"`
		String string `db:"_string"`

		Date  time.Time  `db:"_date"`
		DateN *time.Time `db:"_nildate"`
		DateP *time.Time `db:"_ptrdate"`
		DateD *time.Time `db:"_defaultdate,omitempty"`
		Time  int64      `db:"_time"`
	}

	sess := mustOpen()

	// Getting a pointer to the "data_types" collection.
	dataTypes := sess.Collection("data_types")

	// Removing all data.
	err := dataTypes.Truncate()
	assert.NoError(t, err)

	// Inserting our test subject.
	loc, err := time.LoadLocation(testTimeZone)
	assert.NoError(t, err)

	ts := time.Date(2011, 7, 28, 1, 2, 3, 0, loc) // timestamp with time zone

	var tnz time.Time
	if Adapter == "postgresql" {
		tnz = time.Date(2012, 7, 28, 1, 2, 3, 0, time.FixedZone("", 0)) // timestamp without time zone
	} else {
		tnz = time.Date(2012, 7, 28, 1, 2, 3, 0, time.UTC) // timestamp without time zone
	}

	testValues := testValuesStruct{
		1, 1, 1, 1, 1,
		-1, -1, -1, -1, -1,

		1.337, 1.337,

		true,
		"Hello world!",

		ts,
		nil,
		&tnz,
		nil,
		int64(time.Second * time.Duration(7331)),
	}
	id, err := dataTypes.Insert(testValues)
	assert.NoError(t, err)
	assert.NotNil(t, id)

	// Defining our set.
	cond := db.Cond{"id": id}
	if Adapter == "ql" {
		cond = db.Cond{"id()": id}
	}
	res := dataTypes.Find(cond)

	count, err := res.Count()
	assert.NoError(t, err)
	assert.NotZero(t, count)

	// Trying to dump the subject into an empty structure of the same type.
	var item testValuesStruct

	err = res.One(&item)
	assert.NoError(t, err)
	assert.NotNil(t, item.DateD)

	// Copy the default date (this value is set by the database)
	testValues.DateD = item.DateD
	item.Date = item.Date.In(loc)

	// The original value and the test subject must match.
	assert.Equal(t, testValues, item)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestUpdateWithNullColumn(t *testing.T) {
	sess := mustOpen()

	artist := sess.Collection("artist")
	err := artist.Truncate()
	assert.NoError(t, err)

	type Artist struct {
		ID   int64   `db:"id,omitempty"`
		Name *string `db:"name"`
	}

	name := "José"
	id, err := artist.Insert(Artist{0, &name})
	assert.NoError(t, err)

	var item Artist
	err = artist.Find(id).One(&item)
	assert.NoError(t, err)

	assert.NotEqual(t, nil, item.Name)
	assert.Equal(t, name, *item.Name)

	artist.Find(id).Update(Artist{Name: nil})
	assert.NoError(t, err)

	var item2 Artist
	err = artist.Find(id).One(&item2)
	assert.NoError(t, err)

	assert.Equal(t, (*string)(nil), item2.Name)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestBatchInsert(t *testing.T) {
	sess := mustOpen()

	for batchSize := 0; batchSize < 17; batchSize++ {
		err := sess.Collection("artist").Truncate()
		assert.NoError(t, err)

		q := sess.InsertInto("artist").Columns("name")

		if Adapter == "postgresql" {
			q = q.Amend(func(query string) string {
				return query + ` ON CONFLICT DO NOTHING`
			})
		}

		batch := q.Batch(batchSize)

		totalItems := int(rand.Int31n(21))

		go func() {
			defer batch.Done()
			for i := 0; i < totalItems; i++ {
				batch.Values(fmt.Sprintf("artist-%d", i))
			}
		}()

		err = batch.Wait()
		assert.NoError(t, err)
		assert.NoError(t, batch.Err())

		c, err := sess.Collection("artist").Find().Count()
		assert.NoError(t, err)
		assert.Equal(t, uint64(totalItems), c)

		for i := 0; i < totalItems; i++ {
			c, err := sess.Collection("artist").Find(db.Cond{"name": fmt.Sprintf("artist-%d", i)}).Count()
			assert.NoError(t, err)
			assert.Equal(t, uint64(1), c)
		}
	}

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestBatchInsertNoColumns(t *testing.T) {
	sess := mustOpen()

	for batchSize := 0; batchSize < 17; batchSize++ {
		err := sess.Collection("artist").Truncate()
		assert.NoError(t, err)

		batch := sess.InsertInto("artist").Batch(batchSize)

		totalItems := int(rand.Int31n(21))

		go func() {
			defer batch.Done()
			for i := 0; i < totalItems; i++ {
				value := struct {
					Name string `db:"name"`
				}{fmt.Sprintf("artist-%d", i)}
				batch.Values(value)
			}
		}()

		err = batch.Wait()
		assert.NoError(t, err)
		assert.NoError(t, batch.Err())

		c, err := sess.Collection("artist").Find().Count()
		assert.NoError(t, err)
		assert.Equal(t, uint64(totalItems), c)

		for i := 0; i < totalItems; i++ {
			c, err := sess.Collection("artist").Find(db.Cond{"name": fmt.Sprintf("artist-%d", i)}).Count()
			assert.NoError(t, err)
			assert.Equal(t, uint64(1), c)
		}
	}

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestBatchInsertReturningKeys(t *testing.T) {
	if Adapter != "postgresql" {
		t.Skip("Currently not supported.")
	}

	sess := mustOpen()

	err := sess.Collection("artist").Truncate()
	assert.NoError(t, err)

	batchSize, totalItems := 7, 12

	batch := sess.InsertInto("artist").Columns("name").Returning("id").Batch(batchSize)

	go func() {
		defer batch.Done()
		for i := 0; i < totalItems; i++ {
			batch.Values(fmt.Sprintf("artist-%d", i))
		}
	}()

	var keyMap []struct {
		ID int `db:"id"`
	}
	for batch.NextResult(&keyMap) {
		// Each insertion must produce new keys.
		assert.True(t, len(keyMap) > 0)
		assert.True(t, len(keyMap) <= batchSize)

		// Find the elements we've just inserted
		keys := make([]int, 0, len(keyMap))
		for i := range keyMap {
			keys = append(keys, keyMap[i].ID)
		}

		// Make sure count matches.
		c, err := sess.Collection("artist").Find(db.Cond{"id": keys}).Count()
		assert.NoError(t, err)
		assert.Equal(t, uint64(len(keyMap)), c)
	}
	assert.NoError(t, batch.Err())

	// Count all new elements
	c, err := sess.Collection("artist").Find().Count()
	assert.NoError(t, err)
	assert.Equal(t, uint64(totalItems), c)

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestSQLBuilder(t *testing.T) {
	sess := mustOpen()

	var all []map[string]interface{}

	err := sess.Collection("artist").Truncate()
	assert.NoError(t, err)

	_, err = sess.InsertInto("artist").Values(struct {
		Name string `db:"name"`
	}{"Rinko Kikuchi"}).Exec()
	assert.NoError(t, err)

	// Using explicit iterator.
	iter := sess.SelectFrom("artist").Iterator()
	err = iter.All(&all)

	assert.NoError(t, err)
	assert.NotZero(t, all)

	// Using explicit iterator to fetch one item.
	var item map[string]interface{}
	iter = sess.SelectFrom("artist").Iterator()
	err = iter.One(&item)

	assert.NoError(t, err)
	assert.NotZero(t, item)

	// Using explicit iterator and NextScan.
	iter = sess.SelectFrom("artist").Iterator()
	var id int
	var name string

	if Adapter == "ql" {
		err = iter.NextScan(&name)
		id = 1
	} else {
		err = iter.NextScan(&id, &name)
	}

	assert.NoError(t, err)
	assert.NotZero(t, id)
	assert.NotEmpty(t, name)
	assert.NoError(t, iter.Close())

	err = iter.NextScan(&id, &name)
	assert.Error(t, err)

	// Using explicit iterator and ScanOne.
	iter = sess.SelectFrom("artist").Iterator()
	id, name = 0, ""
	if Adapter == "ql" {
		err = iter.ScanOne(&name)
		id = 1
	} else {
		err = iter.ScanOne(&id, &name)
	}

	assert.NoError(t, err)
	assert.NotZero(t, id)
	assert.NotEmpty(t, name)

	err = iter.ScanOne(&id, &name)
	assert.Error(t, err)

	// Using explicit iterator and Next.
	iter = sess.SelectFrom("artist").Iterator()

	var artist map[string]interface{}
	for iter.Next(&artist) {
		if Adapter != "ql" {
			assert.NotZero(t, artist["id"])
		}
		assert.NotEmpty(t, artist["name"])
	}
	// We should not have any error after finishing successfully exiting a Next() loop.
	assert.Empty(t, iter.Err())

	for i := 0; i < 5; i++ {
		// But we'll get errors if we attempt to continue using Next().
		assert.False(t, iter.Next(&artist))
		assert.Error(t, iter.Err())
	}

	// Using implicit iterator.
	q := sess.SelectFrom("artist")
	err = q.All(&all)

	assert.NoError(t, err)
	assert.NotZero(t, all)

	tx, err := sess.NewTx(nil)
	assert.NoError(t, err)
	assert.NotZero(t, tx)
	defer tx.Close()

	q = tx.SelectFrom("artist")
	assert.NotZero(t, iter)

	err = q.All(&all)
	assert.NoError(t, err)
	assert.NotZero(t, all)

	assert.NoError(t, tx.Commit())

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestExhaustConnectionPool(t *testing.T) {
	if Adapter == "ql" {
		t.Skip("Currently not supported.")
	}

	var tMu sync.Mutex

	tFatal := func(err error) {
		tMu.Lock()
		defer tMu.Unlock()
		t.Fatal(err)
	}

	tLogf := func(format string, args ...interface{}) {
		tMu.Lock()
		defer tMu.Unlock()
		t.Logf(format, args...)
	}

	sess := mustOpen()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		tLogf("Tx %d: Pending", i)

		wg.Add(1)
		go func(wg *sync.WaitGroup, i int) {
			defer wg.Done()

			// Requesting a new transaction session.
			start := time.Now()
			tLogf("Tx: %d: NewTx", i)
			tx, err := sess.NewTx(nil)
			if err != nil {
				tFatal(err)
			}
			tLogf("Tx %d: OK (time to connect: %v)", i, time.Now().Sub(start))

			// Let's suppose that we do a bunch of complex stuff and that the
			// transaction lasts 3 seconds.
			time.Sleep(time.Second * 3)

			switch i % 7 {
			case 0:
				var account map[string]interface{}
				if err := tx.Collection("artist").Find().One(&account); err != nil {
					tFatal(err)
				}
				if err := tx.Commit(); err != nil {
					tFatal(err)
				}
				tLogf("Tx %d: Committed", i)
			case 1:
				if _, err := tx.DeleteFrom("artist").Exec(); err != nil {
					tFatal(err)
				}
				if err := tx.Rollback(); err != nil {
					tFatal(err)
				}
				tLogf("Tx %d: Rolled back", i)
			case 2:
				if err := tx.Close(); err != nil {
					tFatal(err)
				}
				tLogf("Tx %d: Closed", i)
			case 3:
				var account map[string]interface{}
				if err := tx.Collection("artist").Find().One(&account); err != nil {
					tFatal(err)
				}
				if err := tx.Commit(); err != nil {
					tFatal(err)
				}
				if err := tx.Close(); err != nil {
					tFatal(err)
				}
				tLogf("Tx %d: Committed and closed", i)
			case 4:
				if err := tx.Rollback(); err != nil {
					tFatal(err)
				}
				if err := tx.Close(); err != nil {
					tFatal(err)
				}
				tLogf("Tx %d: Rolled back and closed", i)
			case 5:
				if err := tx.Close(); err != nil {
					tFatal(err)
				}
				if err := tx.Commit(); err == nil {
					tFatal(fmt.Errorf("Error expected"))
				}
				tLogf("Tx %d: Closed and committed", i)
			case 6:
				if err := tx.Close(); err != nil {
					tFatal(err)
				}
				if err := tx.Rollback(); err == nil {
					tFatal(fmt.Errorf("Error expected"))
				}
				tLogf("Tx %d: Closed and rolled back", i)
			}
		}(&wg, i)
	}

	wg.Wait()

	assert.NoError(t, cleanUpCheck(sess))
	assert.NoError(t, sess.Close())
}

func TestCustomType(t *testing.T) {
	// See https://github.com/upper/db/issues/332
	sess := mustOpen()

	artist := sess.Collection("artist")

	err := artist.Truncate()
	assert.NoError(t, err)

	id, err := artist.Insert(artistWithCustomType{
		Custom: customType{Val: []byte("some name")},
	})
	assert.NoError(t, err)
	assert.NotNil(t, id)

	var bar artistWithCustomType
	err = artist.Find(id).One(&bar)
	assert.NoError(t, err)

	assert.Equal(t, "foo: some name", string(bar.Custom.Val))
}
