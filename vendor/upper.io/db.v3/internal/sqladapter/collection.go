package sqladapter

import (
	"fmt"
	"reflect"

	"upper.io/db.v3"
	"upper.io/db.v3/internal/sqladapter/exql"
	"upper.io/db.v3/lib/reflectx"
)

var mapper = reflectx.NewMapper("db")

// Collection represents a SQL table.
type Collection interface {
	PartialCollection
	BaseCollection
}

// PartialCollection defines methods that must be implemented by the adapter.
type PartialCollection interface {
	// Database returns the parent database.
	Database() Database

	// Name returns the name of the table.
	Name() string

	// Insert inserts a new item into the collection.
	Insert(interface{}) (interface{}, error)
}

// BaseCollection provides logic for methods that can be shared across all SQL
// adapters.
type BaseCollection interface {
	// Exists returns true if the collection exists.
	Exists() bool

	// Find creates and returns a new result set.
	Find(conds ...interface{}) db.Result

	// Truncate removes all items on the collection.
	Truncate() error

	// InsertReturning inserts a new item and updates it with the
	// actual values from the database.
	InsertReturning(interface{}) error

	// UpdateReturning updates an item and returns the actual values from the
	// database.
	UpdateReturning(interface{}) error

	// PrimaryKeys returns the table's primary keys.
	PrimaryKeys() []string
}

type condsFilter interface {
	FilterConds(...interface{}) []interface{}
}

// collection is the implementation of Collection.
type collection struct {
	BaseCollection
	PartialCollection

	pk  []string
	err error
}

var (
	_ = Collection(&collection{})
)

// NewBaseCollection returns a collection with basic methods.
func NewBaseCollection(p PartialCollection) BaseCollection {
	c := &collection{PartialCollection: p}
	c.pk, c.err = c.Database().PrimaryKeys(c.Name())
	return c
}

// PrimaryKeys returns the collection's primary keys, if any.
func (c *collection) PrimaryKeys() []string {
	return c.pk
}

func (c *collection) filterConds(conds ...interface{}) []interface{} {
	if tr, ok := c.PartialCollection.(condsFilter); ok {
		return tr.FilterConds(conds...)
	}
	if len(conds) == 1 && len(c.pk) == 1 {
		if id := conds[0]; IsKeyValue(id) {
			conds[0] = db.Cond{c.pk[0]: id}
		}
	}
	return conds
}

// Find creates a result set with the given conditions.
func (c *collection) Find(conds ...interface{}) db.Result {
	if c.err != nil {
		res := &Result{}
		res.setErr(c.err)
		return res
	}
	return NewResult(
		c.Database(),
		c.Name(),
		c.filterConds(conds...),
	)
}

// Exists returns true if the collection exists.
func (c *collection) Exists() bool {
	if err := c.Database().TableExists(c.Name()); err != nil {
		return false
	}
	return true
}

// InsertReturning inserts an item and updates the given variable reference.
func (c *collection) InsertReturning(item interface{}) error {
	if item == nil || reflect.TypeOf(item).Kind() != reflect.Ptr {
		return fmt.Errorf("Expecting a pointer but got %T", item)
	}

	// Grab primary keys
	pks := c.PrimaryKeys()
	if len(pks) == 0 {
		return fmt.Errorf("InsertReturning: Cannot update an item without primary keys")
	}

	var tx DatabaseTx
	inTx := false

	if currTx := c.Database().Transaction(); currTx != nil {
		tx = NewDatabaseTx(c.Database())
		inTx = true
	} else {
		// Not within a transaction, let's create one.
		var err error
		tx, err = c.Database().NewDatabaseTx(c.Database().Context())
		if err != nil {
			return err
		}
		defer tx.(Database).Close()
	}

	// Allocate a clone of item.
	newItem := reflect.New(reflect.ValueOf(item).Elem().Type()).Interface()
	var newItemFieldMap map[string]reflect.Value

	itemValue := reflect.ValueOf(item)

	col := tx.(Database).Collection(c.Name())

	// Insert item as is and grab the returning ID.
	id, err := col.Insert(item)
	if err != nil {
		goto cancel
	}
	if id == nil {
		err = fmt.Errorf("InsertReturning: Could not get a valid ID after inserting. Does the %q table have a primary key?", c.Name())
		goto cancel
	}

	// Fetch the row that was just interted into newItem
	if err = col.Find(id).One(newItem); err != nil {
		goto cancel
	}

	switch reflect.ValueOf(newItem).Elem().Kind() {
	case reflect.Struct:
		// Get valid fields from newItem to overwrite those that are on item.
		newItemFieldMap = mapper.ValidFieldMap(reflect.ValueOf(newItem))
		for fieldName := range newItemFieldMap {
			mapper.FieldByName(itemValue, fieldName).Set(newItemFieldMap[fieldName])
		}
	case reflect.Map:
		newItemV := reflect.ValueOf(newItem).Elem()
		itemV := reflect.ValueOf(item)
		if itemV.Kind() == reflect.Ptr {
			itemV = itemV.Elem()
		}
		for _, keyV := range newItemV.MapKeys() {
			itemV.SetMapIndex(keyV, newItemV.MapIndex(keyV))
		}
	default:
		panic("default")
	}

	if !inTx {
		// This is only executed if t.Database() was **not** a transaction and if
		// sess was created with sess.NewTransaction().
		return tx.Commit()
	}
	return err

cancel:
	// This goto label should only be used when we got an error within a
	// transaction and we don't want to continue.

	if !inTx {
		// This is only executed if t.Database() was **not** a transaction and if
		// sess was created with sess.NewTransaction().
		tx.Rollback()
	}
	return err
}

func (c *collection) UpdateReturning(item interface{}) error {
	if item == nil || reflect.TypeOf(item).Kind() != reflect.Ptr {
		return fmt.Errorf("Expecting a pointer but got %T", item)
	}

	// Grab primary keys
	pks := c.PrimaryKeys()
	if len(pks) == 0 {
		return fmt.Errorf("InsertReturning: Cannot update an item without primary keys")
	}

	var tx DatabaseTx
	inTx := false

	if currTx := c.Database().Transaction(); currTx != nil {
		tx = NewDatabaseTx(c.Database())
		inTx = true
	} else {
		// Not within a transaction, let's create one.
		var err error
		tx, err = c.Database().NewDatabaseTx(c.Database().Context())
		if err != nil {
			return err
		}
		defer tx.(Database).Close()
	}

	// Allocate a clone of item.
	defaultItem := reflect.New(reflect.ValueOf(item).Elem().Type()).Interface()
	var defaultItemFieldMap map[string]reflect.Value

	itemValue := reflect.ValueOf(item)

	conds := db.Cond{}
	for _, pk := range pks {
		conds[pk] = mapper.FieldByName(itemValue, pk).Interface()
	}

	col := tx.(Database).Collection(c.Name())

	err := col.Find(conds).Update(item)
	if err != nil {
		goto cancel
	}

	if err = col.Find(conds).One(defaultItem); err != nil {
		goto cancel
	}

	switch reflect.ValueOf(defaultItem).Elem().Kind() {
	case reflect.Struct:
		// Get valid fields from defaultItem to overwrite those that are on item.
		defaultItemFieldMap = mapper.ValidFieldMap(reflect.ValueOf(defaultItem))
		for fieldName := range defaultItemFieldMap {
			mapper.FieldByName(itemValue, fieldName).Set(defaultItemFieldMap[fieldName])
		}
	case reflect.Map:
		defaultItemV := reflect.ValueOf(defaultItem).Elem()
		itemV := reflect.ValueOf(item)
		if itemV.Kind() == reflect.Ptr {
			itemV = itemV.Elem()
		}
		for _, keyV := range defaultItemV.MapKeys() {
			itemV.SetMapIndex(keyV, defaultItemV.MapIndex(keyV))
		}
	default:
		panic("default")
	}

	if !inTx {
		// This is only executed if t.Database() was **not** a transaction and if
		// sess was created with sess.NewTransaction().
		return tx.Commit()
	}
	return err

cancel:
	// This goto label should only be used when we got an error within a
	// transaction and we don't want to continue.

	if !inTx {
		// This is only executed if t.Database() was **not** a transaction and if
		// sess was created with sess.NewTransaction().
		tx.Rollback()
	}
	return err
}

// Truncate deletes all rows from the table.
func (c *collection) Truncate() error {
	stmt := exql.Statement{
		Type:  exql.Truncate,
		Table: exql.TableWithName(c.Name()),
	}
	if _, err := c.Database().Exec(&stmt); err != nil {
		return err
	}
	return nil
}
