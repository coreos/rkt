package sqlbuilder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"upper.io/db.v3"
	"upper.io/db.v3/internal/immutable"
	"upper.io/db.v3/internal/sqladapter/exql"
)

type selectorQuery struct {
	table     *exql.Columns
	tableArgs []interface{}

	as string

	distinct bool

	where     *exql.Where
	whereArgs []interface{}

	groupBy     *exql.GroupBy
	groupByArgs []interface{}

	orderBy     *exql.OrderBy
	orderByArgs []interface{}

	limit  exql.Limit
	offset exql.Offset

	columns     *exql.Columns
	columnsArgs []interface{}

	joins     []*exql.Join
	joinsArgs []interface{}

	amendFn func(string) string
}

func (sq *selectorQuery) and(b *sqlBuilder, terms ...interface{}) error {
	where, whereArgs := b.t.toWhereWithArguments(terms)

	if sq.where == nil {
		sq.where, sq.whereArgs = &exql.Where{}, []interface{}{}
	}
	sq.where.Append(&where)
	sq.whereArgs = append(sq.whereArgs, whereArgs...)

	return nil
}

func (sq *selectorQuery) arguments() []interface{} {
	return joinArguments(
		sq.columnsArgs,
		sq.tableArgs,
		sq.joinsArgs,
		sq.whereArgs,
		sq.groupByArgs,
		sq.orderByArgs,
	)
}

func (sq *selectorQuery) statement() *exql.Statement {
	stmt := &exql.Statement{
		Type:     exql.Select,
		Table:    sq.table,
		Columns:  sq.columns,
		Distinct: sq.distinct,
		Limit:    sq.limit,
		Offset:   sq.offset,
		Where:    sq.where,
		OrderBy:  sq.orderBy,
		GroupBy:  sq.groupBy,
	}

	if len(sq.joins) > 0 {
		stmt.Joins = exql.JoinConditions(sq.joins...)
	}

	stmt.SetAmendment(sq.amendFn)

	return stmt
}

func (sq *selectorQuery) pushJoin(t string, tables []interface{}) error {
	tableNames := make([]string, len(tables))
	for i := range tables {
		tableNames[i] = fmt.Sprintf("%s", tables[i])
	}

	if sq.joins == nil {
		sq.joins = []*exql.Join{}
	}
	sq.joins = append(sq.joins,
		&exql.Join{
			Type:  t,
			Table: exql.TableWithName(strings.Join(tableNames, ", ")),
		},
	)

	return nil
}

type selector struct {
	builder *sqlBuilder

	fn   func(*selectorQuery) error
	prev *selector
}

var _ = immutable.Immutable(&inserter{})

func (sel *selector) SQLBuilder() *sqlBuilder {
	if sel.prev == nil {
		return sel.builder
	}
	return sel.prev.SQLBuilder()
}

func (sel *selector) String() string {
	s, err := sel.Compile()
	if err != nil {
		panic(err.Error())
	}
	return prepareQueryForDisplay(s)
}

func (sel *selector) frame(fn func(*selectorQuery) error) *selector {
	return &selector{prev: sel, fn: fn}
}

func (sel *selector) From(tables ...interface{}) Selector {
	return sel.frame(
		func(sq *selectorQuery) error {
			f, args, err := columnFragments(tables)
			if err != nil {
				return err
			}
			sq.table = exql.JoinColumns(f...)
			sq.tableArgs = args
			return nil
		},
	)
}

func (sel *selector) Columns(columns ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		return sq.pushColumns(columns...)
	})
}

func (sq *selectorQuery) pushColumns(columns ...interface{}) error {
	f, args, err := columnFragments(columns)
	if err != nil {
		return err
	}

	c := exql.JoinColumns(f...)

	if sq.columns != nil {
		sq.columns.Append(c)
	} else {
		sq.columns = c
	}

	sq.columnsArgs = append(sq.columnsArgs, args...)
	return nil
}

func (sel *selector) Distinct(exps ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		sq.distinct = true
		return sq.pushColumns(exps...)
	})
}

func (sel *selector) Where(terms ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		sq.where, sq.whereArgs = &exql.Where{}, []interface{}{}
		return sq.and(sel.SQLBuilder(), terms...)
	})
}

func (sel *selector) And(terms ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		return sq.and(sel.SQLBuilder(), terms...)
	})
}

func (sel *selector) Amend(fn func(string) string) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		sq.amendFn = fn
		return nil
	})
}

func (sel *selector) Arguments() []interface{} {
	sq, err := sel.build()
	if err != nil {
		return nil
	}
	return sq.arguments()
}

func (sel *selector) GroupBy(columns ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		fragments, args, err := columnFragments(columns)
		if err != nil {
			return err
		}

		if fragments != nil {
			sq.groupBy = exql.GroupByColumns(fragments...)
		}
		sq.groupByArgs = args

		return nil
	})
}

func (sel *selector) OrderBy(columns ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		var sortColumns exql.SortColumns

		for i := range columns {
			var sort *exql.SortColumn

			switch value := columns[i].(type) {
			case db.RawValue:
				query, args := Preprocess(value.Raw(), value.Arguments())
				sort = &exql.SortColumn{
					Column: exql.RawValue(query),
				}
				sq.orderByArgs = append(sq.orderByArgs, args...)
			case db.Function:
				fnName, fnArgs := value.Name(), value.Arguments()
				if len(fnArgs) == 0 {
					fnName = fnName + "()"
				} else {
					fnName = fnName + "(?" + strings.Repeat("?, ", len(fnArgs)-1) + ")"
				}
				fnName, fnArgs = Preprocess(fnName, fnArgs)
				sort = &exql.SortColumn{
					Column: exql.RawValue(fnName),
				}
				sq.orderByArgs = append(sq.orderByArgs, fnArgs...)
			case string:
				if strings.HasPrefix(value, "-") {
					sort = &exql.SortColumn{
						Column: exql.ColumnWithName(value[1:]),
						Order:  exql.Descendent,
					}
				} else {
					chunks := strings.SplitN(value, " ", 2)

					order := exql.Ascendent
					if len(chunks) > 1 && strings.ToUpper(chunks[1]) == "DESC" {
						order = exql.Descendent
					}

					sort = &exql.SortColumn{
						Column: exql.ColumnWithName(chunks[0]),
						Order:  order,
					}
				}
			default:
				return fmt.Errorf("Can't sort by type %T", value)
			}
			sortColumns.Columns = append(sortColumns.Columns, sort)
		}

		sq.orderBy = &exql.OrderBy{
			SortColumns: &sortColumns,
		}
		return nil
	})
}

func (sel *selector) Using(columns ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {

		joins := len(sq.joins)

		if joins == 0 {
			return errors.New(`Cannot use Using() without a preceding Join() expression.`)
		}

		lastJoin := sq.joins[joins-1]
		if lastJoin.On != nil {
			return errors.New(`Cannot use Using() and On() with the same Join() expression.`)
		}

		fragments, args, err := columnFragments(columns)
		if err != nil {
			return err
		}

		sq.joinsArgs = append(sq.joinsArgs, args...)
		lastJoin.Using = exql.UsingColumns(fragments...)

		return nil
	})
}

func (sel *selector) FullJoin(tables ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		return sq.pushJoin("FULL", tables)
	})
}

func (sel *selector) CrossJoin(tables ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		return sq.pushJoin("CROSS", tables)
	})
}

func (sel *selector) RightJoin(tables ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		return sq.pushJoin("RIGHT", tables)
	})
}

func (sel *selector) LeftJoin(tables ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		return sq.pushJoin("LEFT", tables)
	})
}

func (sel *selector) Join(tables ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		return sq.pushJoin("", tables)
	})
}

func (sel *selector) On(terms ...interface{}) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		joins := len(sq.joins)

		if joins == 0 {
			return errors.New(`Cannot use On() without a preceding Join() expression.`)
		}

		lastJoin := sq.joins[joins-1]
		if lastJoin.On != nil {
			return errors.New(`Cannot use Using() and On() with the same Join() expression.`)
		}

		w, a := sel.SQLBuilder().t.toWhereWithArguments(terms)
		o := exql.On(w)

		lastJoin.On = &o

		sq.joinsArgs = append(sq.joinsArgs, a...)

		return nil
	})
}

func (sel *selector) Limit(n int) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		sq.limit = exql.Limit(n)
		return nil
	})
}

func (sel *selector) Offset(n int) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		sq.offset = exql.Offset(n)
		return nil
	})
}

func (sel *selector) template() *exql.Template {
	return sel.SQLBuilder().t.Template
}

func (sel *selector) As(alias string) Selector {
	return sel.frame(func(sq *selectorQuery) error {
		if sq.table == nil {
			return errors.New("Cannot use As() without a preceding From() expression")
		}
		last := len(sq.table.Columns) - 1
		if raw, ok := sq.table.Columns[last].(*exql.Raw); ok {
			compiled, err := exql.ColumnWithName(alias).Compile(sel.template())
			if err != nil {
				return err
			}
			sq.table.Columns[last] = exql.RawValue("(" + raw.Value + ") AS " + compiled)
		}
		return nil
	})
}

func (sel *selector) statement() *exql.Statement {
	sq, _ := sel.build()
	return sq.statement()
}

func (sel *selector) QueryRow() (*sql.Row, error) {
	return sel.QueryRowContext(sel.SQLBuilder().sess.Context())
}

func (sel *selector) QueryRowContext(ctx context.Context) (*sql.Row, error) {
	sq, err := sel.build()
	if err != nil {
		return nil, err
	}

	return sel.SQLBuilder().sess.StatementQueryRow(ctx, sq.statement(), sq.arguments()...)
}

func (sel *selector) Prepare() (*sql.Stmt, error) {
	return sel.PrepareContext(sel.SQLBuilder().sess.Context())
}

func (sel *selector) PrepareContext(ctx context.Context) (*sql.Stmt, error) {
	sq, err := sel.build()
	if err != nil {
		return nil, err
	}
	return sel.SQLBuilder().sess.StatementPrepare(ctx, sq.statement())
}

func (sel *selector) Query() (*sql.Rows, error) {
	return sel.QueryContext(sel.SQLBuilder().sess.Context())
}

func (sel *selector) QueryContext(ctx context.Context) (*sql.Rows, error) {
	sq, err := sel.build()
	if err != nil {
		return nil, err
	}
	return sel.SQLBuilder().sess.StatementQuery(ctx, sq.statement(), sq.arguments()...)
}

func (sel *selector) Iterator() Iterator {
	return sel.IteratorContext(sel.SQLBuilder().sess.Context())
}

func (sel *selector) IteratorContext(ctx context.Context) Iterator {
	sq, err := sel.build()
	if err != nil {
		return &iterator{nil, err}
	}

	rows, err := sel.SQLBuilder().sess.StatementQuery(ctx, sq.statement(), sq.arguments()...)
	return &iterator{rows, err}
}

func (sel *selector) All(destSlice interface{}) error {
	return sel.Iterator().All(destSlice)
}

func (sel *selector) One(dest interface{}) error {
	return sel.Iterator().One(dest)
}

func (sel *selector) build() (*selectorQuery, error) {
	sq, err := immutable.FastForward(sel)
	if err != nil {
		return nil, err
	}
	return sq.(*selectorQuery), nil
}

func (sel *selector) Compile() (string, error) {
	return sel.statement().Compile(sel.template())
}

func (sel *selector) Prev() immutable.Immutable {
	if sel == nil {
		return nil
	}
	return sel.prev
}

func (sel *selector) Fn(in interface{}) error {
	if sel.fn == nil {
		return nil
	}
	return sel.fn(in.(*selectorQuery))
}

func (sel *selector) Base() interface{} {
	return &selectorQuery{}
}
