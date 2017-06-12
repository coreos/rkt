package sqladapter

import (
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
)

var (
	activeStatements int64
)

// Stmt represents a *sql.Stmt that is cached and provides the
// OnPurge method to allow it to clean after itself.
type Stmt struct {
	*sql.Stmt

	query string
	mu    sync.Mutex

	count int64
	dead  bool
}

// NewStatement creates an returns an opened statement
func NewStatement(stmt *sql.Stmt, query string) *Stmt {
	s := &Stmt{
		Stmt:  stmt,
		query: query,
	}
	atomic.AddInt64(&activeStatements, 1)
	return s
}

// Open marks the statement as in-use
func (c *Stmt) Open() (*Stmt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dead {
		return nil, errors.New("statement is dead")
	}

	c.count++
	return c, nil
}

// Close closes the underlying statement if no other go-routine is using it.
func (c *Stmt) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count--

	return c.checkClose()
}

func (c *Stmt) checkClose() error {
	if c.dead && c.count == 0 {
		// Statement is dead and we can close it for real.
		err := c.Stmt.Close()
		if err != nil {
			return err
		}
		// Reduce active statements counter.
		atomic.AddInt64(&activeStatements, -1)
	}
	return nil
}

// OnPurge marks the statement as ready to be cleaned up.
func (c *Stmt) OnPurge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dead = true
	c.checkClose()
}

// NumActiveStatements returns the global number of prepared statements in use
// at any point.
func NumActiveStatements() int64 {
	return atomic.LoadInt64(&activeStatements)
}
