package sqlbuilder

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"

	"upper.io/db.v3"
	"upper.io/db.v3/internal/sqladapter/exql"
)

var (
	sqlNull            = exql.RawValue(`NULL`)
	sqlDefault         = exql.RawValue(`DEFAULT`)
	sqlIsOperator      = `IS`
	sqlInOperator      = `IN`
	sqlDefaultOperator = `=`
)

type templateWithUtils struct {
	*exql.Template
}

func newTemplateWithUtils(template *exql.Template) *templateWithUtils {
	return &templateWithUtils{template}
}

func expandQuery(in string, args []interface{}, fn func(interface{}) (string, []interface{})) (string, []interface{}) {
	argn := 0
	argx := make([]interface{}, 0, len(args))
	for i := 0; i < len(in); i++ {
		if in[i] != '?' {
			continue
		}
		if len(args) > argn {
			k, values := fn(args[argn])
			k, values = expandQuery(k, values, fn)

			if k != "" {
				in = in[:i] + k + in[i+1:]
				i += len(k) - 1
			}
			if len(values) > 0 {
				argx = append(argx, values...)
			}
			argn++
		}
	}
	if len(argx) < len(args) {
		argx = append(argx, args[argn:]...)
	}
	return in, argx
}

func (tu *templateWithUtils) PlaceholderValue(in interface{}) (exql.Fragment, []interface{}) {
	switch t := in.(type) {
	case db.RawValue:
		return exql.RawValue(t.String()), t.Arguments()
	case db.Function:
		fnName := t.Name()
		fnArgs := []interface{}{}

		args, _ := toInterfaceArguments(t.Arguments())
		fragments := []string{}
		for i := range args {
			frag, args := tu.PlaceholderValue(args[i])
			fragment, err := frag.Compile(tu.Template)
			if err == nil {
				fragments = append(fragments, fragment)
				fnArgs = append(fnArgs, args...)
			}
		}
		return exql.RawValue(fnName + `(` + strings.Join(fragments, `, `) + `)`), fnArgs
	default:
		// Value must be escaped.
		return sqlPlaceholder, []interface{}{in}
	}
}

// toInterfaceArguments converts the given value into an array of interfaces.
func toInterfaceArguments(value interface{}) (args []interface{}, isSlice bool) {
	v := reflect.ValueOf(value)

	if value == nil {
		return nil, false
	}

	switch t := value.(type) {
	case driver.Valuer:
		return []interface{}{t}, false
	}

	if v.Type().Kind() == reflect.Slice {
		var i, total int

		// Byte slice gets transformed into a string.
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return []interface{}{string(value.([]byte))}, false
		}

		total = v.Len()
		args = make([]interface{}, total)
		for i = 0; i < total; i++ {
			args[i] = v.Index(i).Interface()
		}
		return args, true
	}

	return []interface{}{value}, false
}

// toColumnsValuesAndArguments maps the given columnNames and columnValues into
// expr's Columns and Values, it also extracts and returns query arguments.
func toColumnsValuesAndArguments(columnNames []string, columnValues []interface{}) (*exql.Columns, *exql.Values, []interface{}, error) {
	var arguments []interface{}

	columns := new(exql.Columns)

	columns.Columns = make([]exql.Fragment, 0, len(columnNames))
	for i := range columnNames {
		columns.Columns = append(columns.Columns, exql.ColumnWithName(columnNames[i]))
	}

	values := new(exql.Values)

	arguments = make([]interface{}, 0, len(columnValues))
	values.Values = make([]exql.Fragment, 0, len(columnValues))

	for i := range columnValues {
		switch v := columnValues[i].(type) {
		case *exql.Raw, exql.Raw:
			values.Values = append(values.Values, sqlDefault)
		case *exql.Value:
			// Adding value.
			values.Values = append(values.Values, v)
		case exql.Value:
			// Adding value.
			values.Values = append(values.Values, &v)
		default:
			// Adding both value and placeholder.
			values.Values = append(values.Values, sqlPlaceholder)
			arguments = append(arguments, v)
		}
	}

	return columns, values, arguments, nil
}

func preprocessFn(arg interface{}) (string, []interface{}) {
	values, isSlice := toInterfaceArguments(arg)

	if isSlice {
		if len(values) == 0 {
			return `(NULL)`, nil
		}
		return `(?` + strings.Repeat(`, ?`, len(values)-1) + `)`, values
	}

	if len(values) == 1 {
		switch t := arg.(type) {
		case db.RawValue:
			return Preprocess(t.Raw(), t.Arguments())
		case compilable:
			c, err := t.Compile()
			if err == nil {
				return `(` + c + `)`, t.Arguments()
			}
			panic(err.Error())
		}
	} else if len(values) == 0 {
		return `NULL`, nil
	}

	return "", []interface{}{arg}
}

// Preprocess expands arguments that needs to be expanded and compiles a query
// into a single string.
func Preprocess(in string, args []interface{}) (string, []interface{}) {
	return expandQuery(in, args, preprocessFn)
}

// toWhereWithArguments converts the given parameters into a exql.Where
// value.
func (tu *templateWithUtils) toWhereWithArguments(term interface{}) (where exql.Where, args []interface{}) {
	args = []interface{}{}

	switch t := term.(type) {
	case []interface{}:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				if strings.ContainsAny(s, "?") || len(t) == 1 {
					s, args = Preprocess(s, t[1:])
					where.Conditions = []exql.Fragment{exql.RawValue(s)}
				} else {
					var val interface{}
					key := s
					if len(t) > 2 {
						val = t[1:]
					} else {
						val = t[1]
					}
					cv, v := tu.toColumnValues(db.NewConstraint(key, val))
					args = append(args, v...)
					for i := range cv.ColumnValues {
						where.Conditions = append(where.Conditions, cv.ColumnValues[i])
					}
				}
				return
			}
		}
		for i := range t {
			w, v := tu.toWhereWithArguments(t[i])
			if len(w.Conditions) == 0 {
				continue
			}
			args = append(args, v...)
			where.Conditions = append(where.Conditions, w.Conditions...)
		}
		return
	case db.RawValue:
		r, v := Preprocess(t.Raw(), t.Arguments())
		where.Conditions = []exql.Fragment{exql.RawValue(r)}
		args = append(args, v...)
		return
	case db.Constraints:
		for _, c := range t.Constraints() {
			w, v := tu.toWhereWithArguments(c)
			if len(w.Conditions) == 0 {
				continue
			}
			args = append(args, v...)
			where.Conditions = append(where.Conditions, w.Conditions...)
		}
		return
	case db.Compound:
		var cond exql.Where

		for _, c := range t.Sentences() {
			w, v := tu.toWhereWithArguments(c)
			if len(w.Conditions) == 0 {
				continue
			}
			args = append(args, v...)
			cond.Conditions = append(cond.Conditions, w.Conditions...)
		}

		if len(cond.Conditions) > 0 {
			var frag exql.Fragment
			switch t.Operator() {
			case db.OperatorNone, db.OperatorAnd:
				q := exql.And(cond)
				frag = &q
			case db.OperatorOr:
				q := exql.Or(cond)
				frag = &q
			default:
				panic(fmt.Sprintf("Unknown type %T", t))
			}
			where.Conditions = append(where.Conditions, frag)
		}

		return
	case db.Constraint:
		cv, v := tu.toColumnValues(t)
		args = append(args, v...)
		where.Conditions = append(where.Conditions, cv.ColumnValues...)
		return where, args
	}

	panic(fmt.Sprintf("Unknown condition type %T", term))
}

func (tu *templateWithUtils) toColumnValues(term interface{}) (cv exql.ColumnValues, args []interface{}) {
	args = []interface{}{}

	switch t := term.(type) {
	case []interface{}:
		l := len(t)
		for i := 0; i < l; i++ {
			column, isString := t[i].(string)

			if !isString {
				p, q := tu.toColumnValues(t[i])
				cv.ColumnValues = append(cv.ColumnValues, p.ColumnValues...)
				args = append(args, q...)
				continue
			}

			if !strings.ContainsAny(column, "=") {
				column = fmt.Sprintf("%s = ?", column)
			}

			chunks := strings.SplitN(column, "=", 2)

			column = chunks[0]
			format := strings.TrimSpace(chunks[1])

			columnValue := exql.ColumnValue{
				Column:   exql.ColumnWithName(column),
				Operator: "=",
				Value:    exql.RawValue(format),
			}

			ps := strings.Count(format, "?")
			if i+ps < l {
				for j := 0; j < ps; j++ {
					args = append(args, t[i+j+1])
				}
				i = i + ps
			} else {
				panic(fmt.Sprintf("Format string %q has more placeholders than given arguments.", format))
			}

			cv.ColumnValues = append(cv.ColumnValues, &columnValue)
		}
		return cv, args
	case db.Constraint:
		columnValue := exql.ColumnValue{}

		// Guessing operator from input, or using a default one.
		if column, ok := t.Key().(string); ok {
			chunks := strings.SplitN(strings.TrimSpace(column), ` `, 2)
			columnValue.Column = exql.ColumnWithName(chunks[0])
			if len(chunks) > 1 {
				columnValue.Operator = chunks[1]
			}
		} else {
			if rawValue, ok := t.Key().(db.RawValue); ok {
				columnValue.Column = exql.RawValue(rawValue.Raw())
				args = append(args, rawValue.Arguments()...)
			} else {
				columnValue.Column = exql.RawValue(fmt.Sprintf("%v", t.Key()))
			}
		}

		switch value := t.Value().(type) {
		case db.Function:
			fnName, fnArgs := value.Name(), value.Arguments()
			if len(fnArgs) == 0 {
				// A function with no arguments.
				fnName = fnName + "()"
			} else {
				// A function with one or more arguments.
				fnName = fnName + "(?" + strings.Repeat("?, ", len(fnArgs)-1) + ")"
			}
			fnName, fnArgs = Preprocess(fnName, fnArgs)
			columnValue.Value = exql.RawValue(fnName)
			args = append(args, fnArgs...)
		case db.RawValue:
			q, a := Preprocess(value.Raw(), value.Arguments())
			columnValue.Value = exql.RawValue(q)
			args = append(args, a...)
		default:
			v, isSlice := toInterfaceArguments(value)

			if isSlice {
				if columnValue.Operator == "" {
					columnValue.Operator = sqlInOperator
				}
				if len(v) > 0 {
					// Array value given.
					columnValue.Value = exql.RawValue(fmt.Sprintf(`(?%s)`, strings.Repeat(`, ?`, len(v)-1)))
				} else {
					// Single value given.
					columnValue.Value = exql.RawValue(`(NULL)`)
				}
				args = append(args, v...)
			} else {
				if v == nil {
					// Nil value given.
					columnValue.Value = sqlNull
					if columnValue.Operator == "" {
						columnValue.Operator = sqlIsOperator
					}
				} else {
					columnValue.Value = sqlPlaceholder
					args = append(args, v...)
				}
			}
		}

		// Using guessed operator if no operator was given.
		if columnValue.Operator == "" {
			if tu.DefaultOperator != "" {
				columnValue.Operator = tu.DefaultOperator
			} else {
				columnValue.Operator = sqlDefaultOperator
			}
		}

		// Using guessed operator if no operator was given.
		cv.ColumnValues = append(cv.ColumnValues, &columnValue)

		return cv, args
	case db.RawValue:
		columnValue := exql.ColumnValue{}
		p, q := Preprocess(t.Raw(), t.Arguments())

		columnValue.Column = exql.RawValue(p)
		args = append(args, q...)

		cv.ColumnValues = append(cv.ColumnValues, &columnValue)
		return cv, args
	case db.Constraints:
		for _, c := range t.Constraints() {
			p, q := tu.toColumnValues(c)
			cv.ColumnValues = append(cv.ColumnValues, p.ColumnValues...)
			args = append(args, q...)
		}
		return cv, args
	}

	panic(fmt.Sprintf("Unknown term type %T.", term))
}
