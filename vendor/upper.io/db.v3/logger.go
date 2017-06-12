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

package db

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

const (
	fmtLogSessID       = `Session ID:     %05d`
	fmtLogTxID         = `Transaction ID: %05d`
	fmtLogQuery        = `Query:          %s`
	fmtLogArgs         = `Arguments:      %#v`
	fmtLogRowsAffected = `Rows affected:  %d`
	fmtLogLastInsertID = `Last insert ID: %d`
	fmtLogError        = `Error:          %v`
	fmtLogTimeTaken    = `Time taken:     %0.5fs`
)

var (
	reInvisibleChars       = regexp.MustCompile(`[\s\r\n\t]+`)
	reColumnCompareExclude = regexp.MustCompile(`[^a-zA-Z0-9]`)
)

// QueryStatus represents the status of a query after being executed.
type QueryStatus struct {
	SessID uint64
	TxID   uint64

	RowsAffected *int64
	LastInsertID *int64

	Query string
	Args  []interface{}

	Err error

	Start time.Time
	End   time.Time
}

// String returns a formatted log message.
func (q *QueryStatus) String() string {
	lines := make([]string, 0, 8)

	if q.SessID > 0 {
		lines = append(lines, fmt.Sprintf(fmtLogSessID, q.SessID))
	}

	if q.TxID > 0 {
		lines = append(lines, fmt.Sprintf(fmtLogTxID, q.TxID))
	}

	if query := q.Query; query != "" {
		query = reInvisibleChars.ReplaceAllString(query, ` `)
		query = strings.TrimSpace(query)
		lines = append(lines, fmt.Sprintf(fmtLogQuery, query))
	}

	if len(q.Args) > 0 {
		lines = append(lines, fmt.Sprintf(fmtLogArgs, q.Args))
	}

	if q.RowsAffected != nil {
		lines = append(lines, fmt.Sprintf(fmtLogRowsAffected, *q.RowsAffected))
	}
	if q.LastInsertID != nil {
		lines = append(lines, fmt.Sprintf(fmtLogLastInsertID, *q.LastInsertID))
	}

	if q.Err != nil {
		lines = append(lines, fmt.Sprintf(fmtLogError, q.Err))
	}

	lines = append(lines, fmt.Sprintf(fmtLogTimeTaken, float64(q.End.UnixNano()-q.Start.UnixNano())/float64(1e9)))

	return strings.Join(lines, "\n")
}

// EnvEnableDebug can be used by adapters to determine if the user has enabled
// debugging.
//
// If the user sets the `UPPERIO_DB_DEBUG` environment variable to a
// non-empty value, all generated statements will be printed at runtime to
// the standard logger.
//
// Example:
//
//	UPPERIO_DB_DEBUG=1 go test
//
//	UPPERIO_DB_DEBUG=1 ./go-program
const (
	EnvEnableDebug = `UPPERIO_DB_DEBUG`
)

// Logger represents a logging collector. You can pass a logging collector to
// db.DefaultSettings.SetLogger(myCollector) to make it collect db.QueryStatus messages
// after executing a query.
type Logger interface {
	Log(*QueryStatus)
}

type defaultLogger struct {
}

func (lg *defaultLogger) Log(m *QueryStatus) {
	log.Printf("\n\t%s\n\n", strings.Replace(m.String(), "\n", "\n\t", -1))
}

var _ = Logger(&defaultLogger{})

func init() {
	if envEnabled(EnvEnableDebug) {
		DefaultSettings.SetLogging(true)
	}
}
