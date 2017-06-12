package sqlbuilder

import (
	"errors"
)

// Common error messages.
var (
	ErrExpectingPointer                    = errors.New(`Argument must be an address.`)
	ErrExpectingSlicePointer               = errors.New(`Argument must be a slice address.`)
	ErrExpectingSliceMapStruct             = errors.New(`Argument must be a slice address of maps or structs.`)
	ErrExpectingMapOrStruct                = errors.New(`Argument must be either a map or a struct.`)
	ErrExpectingPointerToEitherMapOrStruct = errors.New(`Expecting a pointer to either a map or a struct.`)
)
