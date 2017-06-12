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
	"errors"
)

// Error messages.
var (
	ErrNoMoreRows               = errors.New(`upper: no more rows in this result set`)
	ErrNotConnected             = errors.New(`upper: you're currently not connected`)
	ErrMissingDatabaseName      = errors.New(`upper: missing database name`)
	ErrMissingCollectionName    = errors.New(`upper: missing collection name`)
	ErrCollectionDoesNotExist   = errors.New(`upper: collection does not exist`)
	ErrSockerOrHost             = errors.New(`upper: you may connect either to a unix socket or a tcp address, but not both`)
	ErrQueryLimitParam          = errors.New(`upper: a query can accept only one limit parameter`)
	ErrQuerySortParam           = errors.New(`upper: a query can accept only one order by parameter`)
	ErrQueryOffsetParam         = errors.New(`upper: a query can accept only one offset parameter`)
	ErrMissingConditions        = errors.New(`upper: missing selector conditions`)
	ErrUnsupported              = errors.New(`upper: this action is currently unsupported on this database`)
	ErrUndefined                = errors.New(`upper: this value is undefined`)
	ErrQueryIsPending           = errors.New(`upper: can't execute this instruction while the result set is still open`)
	ErrUnsupportedDestination   = errors.New(`upper: unsupported destination type`)
	ErrUnsupportedType          = errors.New(`upper: this type does not support marshaling`)
	ErrUnsupportedValue         = errors.New(`upper: this value does not support unmarshaling`)
	ErrUnknownConditionType     = errors.New(`upper: arguments of type %T can't be used as constraints`)
	ErrTooManyClients           = errors.New(`upper: can't connect to database server: too many clients`)
	ErrGivingUpTryingToConnect  = errors.New(`upper: giving up trying to connect: too many clients`)
	ErrMissingConnURL           = errors.New(`upper: missing DSN`)
	ErrNotImplemented           = errors.New(`upper: call not implemented`)
	ErrAlreadyWithinTransaction = errors.New(`upper: already within a transaction`)
)
