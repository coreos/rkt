package sqladapter

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"upper.io/db.v3"
	"upper.io/db.v3/internal/cache"
	"upper.io/db.v3/internal/sqladapter/compat"
	"upper.io/db.v3/internal/sqladapter/exql"
	"upper.io/db.v3/lib/sqlbuilder"
)

var (
	lastSessID uint64
	lastTxID   uint64
)

// hasCleanUp is implemented by structs that have a clean up routine that needs
// to be called before Close().
type hasCleanUp interface {
	CleanUp() error
}

// hasStatementExec allows the adapter to have its own exec statement.
type hasStatementExec interface {
	StatementExec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// Database represents a SQL database.
type Database interface {
	PartialDatabase
	BaseDatabase
}

// PartialDatabase defines methods to be implemented by SQL database adapters.
type PartialDatabase interface {
	sqlbuilder.SQLBuilder

	// Collections returns a list of non-system tables from the database.
	Collections() ([]string, error)

	// Open opens a new connection
	Open(db.ConnectionURL) error

	// TableExists returns an error if the given table does not exist.
	TableExists(name string) error

	// LookupName returns the name of the database.
	LookupName() (string, error)

	// PrimaryKeys returns all primary keys on the table.
	PrimaryKeys(name string) ([]string, error)

	// NewCollection allocates a new collection by name.
	NewCollection(name string) db.Collection

	// CompileStatement transforms an internal statement into a format
	// database/sql can understand.
	CompileStatement(stmt *exql.Statement, args []interface{}) (string, []interface{})

	// ConnectionURL returns the database's connection URL, if any.
	ConnectionURL() db.ConnectionURL

	// Err wraps specific database errors (given in string form) and transforms them
	// into error values.
	Err(in error) (out error)

	// NewDatabaseTx begins a transaction block and returns a new
	// session backed by it.
	NewDatabaseTx(ctx context.Context) (DatabaseTx, error)
}

// BaseDatabase provides logic for methods that can be shared across all SQL
// adapters.
type BaseDatabase interface {
	db.Settings

	// Name returns the name of the database.
	Name() string

	// Close closes the database session
	Close() error

	// Ping checks if the database server is reachable.
	Ping() error

	// ClearCache clears all caches the session is using
	ClearCache()

	// Collection returns a new collection.
	Collection(string) db.Collection

	// Driver returns the underlying driver the session is using
	Driver() interface{}

	// WaitForConnection attempts to run the given connection function a fixed
	// number of times before failing.
	WaitForConnection(func() error) error

	// BindSession sets the *sql.DB the session will use.
	BindSession(*sql.DB) error

	// Session returns the *sql.DB the session is using.
	Session() *sql.DB

	// BindTx binds a transaction to the current session.
	BindTx(context.Context, *sql.Tx) error

	// Returns the current transaction the session is using.
	Transaction() BaseTx

	// NewClone clones the database using the given PartialDatabase as base.
	NewClone(PartialDatabase, bool) (BaseDatabase, error)

	// Context returns the default context the session is using.
	Context() context.Context

	// SetContext sets a default context for the session.
	SetContext(context.Context)
}

// NewBaseDatabase provides a BaseDatabase given a PartialDatabase
func NewBaseDatabase(p PartialDatabase) BaseDatabase {
	d := &database{
		Settings:          db.NewSettings(),
		PartialDatabase:   p,
		cachedCollections: cache.NewCache(),
		cachedStatements:  cache.NewCache(),
	}
	return d
}

// database is the actual implementation of Database and joins methods from
// BaseDatabase and PartialDatabase
type database struct {
	PartialDatabase
	baseTx BaseTx

	db.Settings

	ctx context.Context

	collectionMu sync.Mutex
	mu           sync.Mutex

	name   string
	sess   *sql.DB
	sessMu sync.Mutex

	psMu sync.Mutex

	sessID uint64
	txID   uint64

	cachedStatements  *cache.Cache
	cachedCollections *cache.Cache

	template *exql.Template
}

var (
	_ = db.Database(&database{})
)

// Session returns the underlying *sql.DB
func (d *database) Session() *sql.DB {
	return d.sess
}

// SetContext sets the session's default context.
func (d *database) SetContext(ctx context.Context) {
	d.mu.Lock()
	d.ctx = ctx
	d.mu.Unlock()
}

// Context returns the session's default context.
func (d *database) Context() context.Context {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ctx == nil {
		return context.Background()
	}
	return d.ctx
}

// BindTx binds a *sql.Tx into *database
func (d *database) BindTx(ctx context.Context, t *sql.Tx) error {
	d.sessMu.Lock()
	defer d.sessMu.Unlock()

	d.baseTx = newBaseTx(t)
	if err := d.Ping(); err != nil {
		return err
	}

	d.SetContext(ctx)
	d.txID = newBaseTxID()
	return nil
}

// Tx returns a BaseTx, which, if not nil, means that this session is within a
// transaction
func (d *database) Transaction() BaseTx {
	return d.baseTx
}

// Name returns the database named
func (d *database) Name() string {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.name == "" {
		d.name, _ = d.PartialDatabase.LookupName()
	}

	return d.name
}

// BindSession binds a *sql.DB into *database
func (d *database) BindSession(sess *sql.DB) error {
	d.sessMu.Lock()
	d.sess = sess
	d.sessMu.Unlock()

	if err := d.Ping(); err != nil {
		return err
	}

	d.sessID = newSessionID()
	name, err := d.PartialDatabase.LookupName()
	if err != nil {
		return err
	}

	d.name = name

	return nil
}

// Ping checks whether a connection to the database is still alive by pinging
// it
func (d *database) Ping() error {
	if d.sess != nil {
		return d.sess.Ping()
	}
	return nil
}

// SetConnMaxLifetime sets the maximum amount of time a connection may be
// reused.
func (d *database) SetConnMaxLifetime(t time.Duration) {
	d.Settings.SetConnMaxLifetime(t)
	if sess := d.Session(); sess != nil {
		sess.SetConnMaxLifetime(d.Settings.ConnMaxLifetime())
	}
}

// SetMaxIdleConns sets the maximum number of connections in the idle
// connection pool.
func (d *database) SetMaxIdleConns(n int) {
	d.Settings.SetMaxIdleConns(n)
	if sess := d.Session(); sess != nil {
		sess.SetMaxIdleConns(d.MaxIdleConns())
	}
}

// SetMaxOpenConns sets the maximum number of open connections to the
// database.
func (d *database) SetMaxOpenConns(n int) {
	d.Settings.SetMaxOpenConns(n)
	if sess := d.Session(); sess != nil {
		sess.SetMaxOpenConns(d.MaxOpenConns())
	}
}

// ClearCache removes all caches.
func (d *database) ClearCache() {
	d.collectionMu.Lock()
	defer d.collectionMu.Unlock()
	d.cachedCollections.Clear()
	d.cachedStatements.Clear()
	if d.template != nil {
		d.template.Cache.Clear()
	}
}

// NewClone binds a clone that is linked to the current
// session. This is commonly done before creating a transaction
// session.
func (d *database) NewClone(p PartialDatabase, checkConn bool) (BaseDatabase, error) {
	nd := NewBaseDatabase(p).(*database)
	nd.name = d.name
	nd.sess = d.sess
	if checkConn {
		if err := nd.Ping(); err != nil {
			return nil, err
		}
	}
	nd.sessID = newSessionID()
	return nd, nil
}

// Close terminates the current database session
func (d *database) Close() error {
	defer func() {
		d.sessMu.Lock()
		d.sess = nil
		d.baseTx = nil
		d.sessMu.Unlock()
	}()
	if d.sess != nil {
		if cleaner, ok := d.PartialDatabase.(hasCleanUp); ok {
			cleaner.CleanUp()
		}

		d.cachedCollections.Clear()
		d.cachedStatements.Clear() // Closes prepared statements as well.

		tx := d.Transaction()
		if tx == nil {
			// Not within a transaction.
			return d.sess.Close()
		}

		if !tx.Committed() {
			tx.Rollback()
			return nil
		}
	}
	return nil
}

// Collection returns a db.Collection given a name. Results are cached.
func (d *database) Collection(name string) db.Collection {
	d.collectionMu.Lock()
	defer d.collectionMu.Unlock()

	h := cache.String(name)

	ccol, ok := d.cachedCollections.ReadRaw(h)
	if ok {
		return ccol.(db.Collection)
	}

	col := d.PartialDatabase.NewCollection(name)
	d.cachedCollections.Write(h, col)

	return col
}

// StatementPrepare creates a prepared statement.
func (d *database) StatementPrepare(ctx context.Context, stmt *exql.Statement) (sqlStmt *sql.Stmt, err error) {
	var query string

	if d.Settings.LoggingEnabled() {
		defer func(start time.Time) {
			d.Logger().Log(&db.QueryStatus{
				TxID:   d.txID,
				SessID: d.sessID,
				Query:  query,
				Err:    err,
				Start:  start,
				End:    time.Now(),
			})
		}(time.Now())
	}

	tx := d.Transaction()

	query, _ = d.compileStatement(stmt, nil)
	if tx != nil {
		sqlStmt, err = compat.PrepareContext(tx.(*baseTx), ctx, query)
		return
	}

	sqlStmt, err = compat.PrepareContext(d.sess, ctx, query)
	return
}

// StatementExec compiles and executes a statement that does not return any
// rows.
func (d *database) StatementExec(ctx context.Context, stmt *exql.Statement, args ...interface{}) (res sql.Result, err error) {
	var query string

	if d.Settings.LoggingEnabled() {
		defer func(start time.Time) {

			status := db.QueryStatus{
				TxID:   d.txID,
				SessID: d.sessID,
				Query:  query,
				Args:   args,
				Err:    err,
				Start:  start,
				End:    time.Now(),
			}

			if res != nil {
				if rowsAffected, err := res.RowsAffected(); err == nil {
					status.RowsAffected = &rowsAffected
				}

				if lastInsertID, err := res.LastInsertId(); err == nil {
					status.LastInsertID = &lastInsertID
				}
			}

			d.Logger().Log(&status)
		}(time.Now())
	}

	if execer, ok := d.PartialDatabase.(hasStatementExec); ok {
		query, args = d.compileStatement(stmt, args)
		res, err = execer.StatementExec(ctx, query, args...)
		return
	}

	tx := d.Transaction()

	if d.Settings.PreparedStatementCacheEnabled() && tx == nil {
		var p *Stmt
		if p, query, args, err = d.prepareStatement(ctx, stmt, args); err != nil {
			return nil, err
		}
		defer p.Close()

		res, err = compat.PreparedExecContext(p, ctx, args)
		return
	}

	query, args = d.compileStatement(stmt, args)
	if tx != nil {
		res, err = compat.ExecContext(tx.(*baseTx), ctx, query, args)
		return
	}

	res, err = compat.ExecContext(d.sess, ctx, query, args)
	return
}

// StatementQuery compiles and executes a statement that returns rows.
func (d *database) StatementQuery(ctx context.Context, stmt *exql.Statement, args ...interface{}) (rows *sql.Rows, err error) {
	var query string

	if d.Settings.LoggingEnabled() {
		defer func(start time.Time) {
			d.Logger().Log(&db.QueryStatus{
				TxID:   d.txID,
				SessID: d.sessID,
				Query:  query,
				Args:   args,
				Err:    err,
				Start:  start,
				End:    time.Now(),
			})
		}(time.Now())
	}

	tx := d.Transaction()

	if d.Settings.PreparedStatementCacheEnabled() && tx == nil {
		var p *Stmt
		if p, query, args, err = d.prepareStatement(ctx, stmt, args); err != nil {
			return nil, err
		}
		defer p.Close()

		rows, err = compat.PreparedQueryContext(p, ctx, args)
		return
	}

	query, args = d.compileStatement(stmt, args)
	if tx != nil {
		rows, err = compat.QueryContext(tx.(*baseTx), ctx, query, args)
		return
	}

	rows, err = compat.QueryContext(d.sess, ctx, query, args)
	return

}

// StatementQueryRow compiles and executes a statement that returns at most one
// row.
func (d *database) StatementQueryRow(ctx context.Context, stmt *exql.Statement, args ...interface{}) (row *sql.Row, err error) {
	var query string

	if d.Settings.LoggingEnabled() {
		defer func(start time.Time) {
			d.Logger().Log(&db.QueryStatus{
				TxID:   d.txID,
				SessID: d.sessID,
				Query:  query,
				Args:   args,
				Err:    err,
				Start:  start,
				End:    time.Now(),
			})
		}(time.Now())
	}

	tx := d.Transaction()

	if d.Settings.PreparedStatementCacheEnabled() && tx == nil {
		var p *Stmt
		if p, query, args, err = d.prepareStatement(ctx, stmt, args); err != nil {
			return nil, err
		}
		defer p.Close()

		row = compat.PreparedQueryRowContext(p, ctx, args)
		return
	}

	query, args = d.compileStatement(stmt, args)
	if tx != nil {
		row = compat.QueryRowContext(tx.(*baseTx), ctx, query, args)
		return
	}

	row = compat.QueryRowContext(d.sess, ctx, query, args)
	return
}

// Driver returns the underlying *sql.DB or *sql.Tx instance.
func (d *database) Driver() interface{} {
	if tx := d.Transaction(); tx != nil {
		// A transaction
		return tx.(*baseTx).Tx
	}
	return d.sess
}

// compileStatement compiles the given statement into a string.
func (d *database) compileStatement(stmt *exql.Statement, args []interface{}) (string, []interface{}) {
	return d.PartialDatabase.CompileStatement(stmt, args)
}

// prepareStatement compiles a query and tries to use previously generated
// statement.
func (d *database) prepareStatement(ctx context.Context, stmt *exql.Statement, args []interface{}) (*Stmt, string, []interface{}, error) {
	d.sessMu.Lock()
	defer d.sessMu.Unlock()

	sess, tx := d.sess, d.Transaction()
	if sess == nil && tx == nil {
		return nil, "", nil, db.ErrNotConnected
	}

	pc, ok := d.cachedStatements.ReadRaw(stmt)
	if ok {
		// The statement was cached.
		ps, err := pc.(*Stmt).Open()
		if err == nil {
			_, args = d.compileStatement(stmt, args)
			return ps, ps.query, args, nil
		}
	}

	query, args := d.compileStatement(stmt, args)
	sqlStmt, err := func(query *string) (*sql.Stmt, error) {
		if tx != nil {
			return compat.PrepareContext(tx.(*baseTx), ctx, *query)
		}
		return compat.PrepareContext(sess, ctx, *query)
	}(&query)
	if err != nil {
		return nil, "", nil, err
	}

	p, err := NewStatement(sqlStmt, query).Open()
	if err != nil {
		return nil, query, args, err
	}
	d.cachedStatements.Write(stmt, p)
	return p, p.query, args, nil
}

var waitForConnMu sync.Mutex

// WaitForConnection tries to execute the given connectFn function, if
// connectFn returns an error, then WaitForConnection will keep trying until
// connectFn returns nil. Maximum waiting time is 5s after having acquired the
// lock.
func (d *database) WaitForConnection(connectFn func() error) error {
	// This lock ensures first-come, first-served and prevents opening too many
	// file descriptors.
	waitForConnMu.Lock()
	defer waitForConnMu.Unlock()

	// Minimum waiting time.
	waitTime := time.Millisecond * 10

	// Waitig 5 seconds for a successful connection.
	for timeStart := time.Now(); time.Now().Sub(timeStart) < time.Second*5; {
		err := connectFn()
		if err == nil {
			return nil // Connected!
		}

		// Only attempt to reconnect if the error is too many clients.
		if d.PartialDatabase.Err(err) == db.ErrTooManyClients {
			// Sleep and try again if, and only if, the server replied with a "too
			// many clients" error.
			time.Sleep(waitTime)
			if waitTime < time.Millisecond*500 {
				// Wait a bit more next time.
				waitTime = waitTime * 2
			}
			continue
		}

		// Return any other error immediately.
		return err
	}

	return db.ErrGivingUpTryingToConnect
}

// ReplaceWithDollarSign turns a SQL statament with '?' placeholders into
// dollar placeholders, like $1, $2, ..., $n
func ReplaceWithDollarSign(in string) string {
	buf := []byte(in)
	out := make([]byte, 0, len(buf))

	i, j, k, t := 0, 1, 0, len(buf)

	for i < t {
		if buf[i] == '?' {
			out = append(out, buf[k:i]...)
			out = append(out, []byte("$"+strconv.Itoa(j))...)
			k = i + 1
			j++
		}
		i++
	}
	out = append(out, buf[k:i]...)

	return string(out)
}

func newSessionID() uint64 {
	if atomic.LoadUint64(&lastSessID) == math.MaxUint64 {
		atomic.StoreUint64(&lastSessID, 0)
		return 0
	}
	return atomic.AddUint64(&lastSessID, 1)
}

func newBaseTxID() uint64 {
	if atomic.LoadUint64(&lastTxID) == math.MaxUint64 {
		atomic.StoreUint64(&lastTxID, 0)
		return 0
	}
	return atomic.AddUint64(&lastTxID, 1)
}
