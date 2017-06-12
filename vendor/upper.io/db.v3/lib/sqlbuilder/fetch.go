// Copyright (c) 2012-present The upper.io/db authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package sqlbuilder

import (
	"database/sql"
	"encoding/json"
	"reflect"

	"upper.io/db.v3"
	"upper.io/db.v3/lib/reflectx"
)

var mapper = reflectx.NewMapper("db")

// fetchRow receives a *sql.Rows value and tries to map all the rows into a
// single struct given by the pointer `dst`.
func fetchRow(rows *sql.Rows, dst interface{}) error {
	var columns []string
	var err error

	dstv := reflect.ValueOf(dst)

	if dstv.IsNil() || dstv.Kind() != reflect.Ptr {
		return ErrExpectingPointer
	}

	itemV := dstv.Elem()

	if columns, err = rows.Columns(); err != nil {
		return err
	}

	reset(dst)

	next := rows.Next()

	if next == false {
		if err = rows.Err(); err != nil {
			return err
		}
		return db.ErrNoMoreRows
	}

	itemT := itemV.Type()
	item, err := fetchResult(itemT, rows, columns)

	if err != nil {
		return err
	}

	if itemT.Kind() == reflect.Ptr {
		itemV.Set(item)
	} else {
		itemV.Set(reflect.Indirect(item))
	}

	return nil
}

// fetchRows receives a *sql.Rows value and tries to map all the rows into a
// slice of structs given by the pointer `dst`.
func fetchRows(rows *sql.Rows, dst interface{}) error {
	var err error

	defer rows.Close()

	// Destination.
	dstv := reflect.ValueOf(dst)

	if dstv.IsNil() || dstv.Kind() != reflect.Ptr {
		return ErrExpectingPointer
	}

	if dstv.Elem().Kind() != reflect.Slice {
		return ErrExpectingSlicePointer
	}

	if dstv.Kind() != reflect.Ptr || dstv.Elem().Kind() != reflect.Slice || dstv.IsNil() {
		return ErrExpectingSliceMapStruct
	}

	var columns []string
	if columns, err = rows.Columns(); err != nil {
		return err
	}

	slicev := dstv.Elem()
	itemT := slicev.Type().Elem()

	reset(dst)

	for rows.Next() {
		item, err := fetchResult(itemT, rows, columns)
		if err != nil {
			return err
		}
		if itemT.Kind() == reflect.Ptr {
			slicev = reflect.Append(slicev, item)
		} else {
			slicev = reflect.Append(slicev, reflect.Indirect(item))
		}
	}

	dstv.Elem().Set(slicev)

	return nil
}

func fetchResult(itemT reflect.Type, rows *sql.Rows, columns []string) (reflect.Value, error) {
	var item reflect.Value
	var err error

	objT := itemT

	switch objT.Kind() {
	case reflect.Map:
		item = reflect.MakeMap(objT)
	case reflect.Struct:
		item = reflect.New(objT)
	case reflect.Ptr:
		objT = itemT.Elem()
		if objT.Kind() != reflect.Struct {
			return item, ErrExpectingMapOrStruct
		}
		item = reflect.New(objT)
	default:
		return item, ErrExpectingMapOrStruct
	}

	switch objT.Kind() {

	case reflect.Struct:

		values := make([]interface{}, len(columns))
		typeMap := mapper.TypeMap(itemT)
		fieldMap := typeMap.Names
		wrappedValues := map[*reflectx.FieldInfo]interface{}{}

		for i, k := range columns {
			fi, ok := fieldMap[k]
			if !ok {
				values[i] = new(interface{})
				continue
			}

			// TODO: refactor into a nice pattern
			if _, ok := fi.Options["stringarray"]; ok {
				values[i] = &[]byte{}
				wrappedValues[fi] = values[i]
			} else if _, ok := fi.Options["int64array"]; ok {
				values[i] = &[]byte{}
				wrappedValues[fi] = values[i]
			} else if _, ok := fi.Options["jsonb"]; ok {
				values[i] = &[]byte{}
				wrappedValues[fi] = values[i]
			} else {
				f := reflectx.FieldByIndexes(item, fi.Index)
				values[i] = f.Addr().Interface()
			}
			if u, ok := values[i].(db.Unmarshaler); ok {
				values[i] = scanner{u}
			}
		}

		// Scanner - for reads
		// Valuer  - for writes

		// OptionTypes
		// - before/after scan
		// - before/after valuer..

		if err = rows.Scan(values...); err != nil {
			return item, err
		}

		// TODO: move this stuff out of here.. find a nice pattern
		for fi, v := range wrappedValues {
			var opt string
			if _, ok := fi.Options["stringarray"]; ok {
				opt = "stringarray"
			} else if _, ok := fi.Options["int64array"]; ok {
				opt = "int64array"
			} else if _, ok := fi.Options["jsonb"]; ok {
				opt = "jsonb"
			}

			b := v.(*[]byte)

			f := reflectx.FieldByIndexesReadOnly(item, fi.Index)

			switch opt {
			case "stringarray":
				v := stringArray{}
				err := v.Scan(*b)
				if err != nil {
					return item, err
				}
				f.Set(reflect.ValueOf(v))
			case "int64array":
				v := int64Array{}
				err := v.Scan(*b)
				if err != nil {
					return item, err
				}
				f.Set(reflect.ValueOf(v))
			case "jsonb":
				if len(*b) == 0 {
					continue
				}

				var vv reflect.Value
				t := reflect.PtrTo(f.Type())

				switch t.Kind() {
				case reflect.Map:
					vv = reflect.MakeMap(t)
				case reflect.Slice:
					vv = reflect.MakeSlice(t, 0, 0)
				default:
					vv = reflect.New(t)
				}

				err := json.Unmarshal(*b, vv.Interface())
				if err != nil {
					return item, err
				}

				vv = vv.Elem().Elem()

				if !vv.IsValid() || (vv.Kind() == reflect.Ptr && vv.IsNil()) {
					continue
				}

				f.Set(vv)
			}
		}

	case reflect.Map:

		columns, err := rows.Columns()
		if err != nil {
			return item, err
		}

		values := make([]interface{}, len(columns))
		for i := range values {
			if itemT.Elem().Kind() == reflect.Interface {
				values[i] = new(interface{})
			} else {
				values[i] = reflect.New(itemT.Elem()).Interface()
			}
		}

		if err = rows.Scan(values...); err != nil {
			return item, err
		}

		for i, column := range columns {
			item.SetMapIndex(reflect.ValueOf(column), reflect.Indirect(reflect.ValueOf(values[i])))
		}

	}

	return item, nil
}

func reset(data interface{}) error {
	// Resetting element.
	v := reflect.ValueOf(data).Elem()
	t := v.Type()
	z := reflect.Zero(t)
	v.Set(z)
	return nil
}
