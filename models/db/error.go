package db

import "errors"

// ErrNotExist is wrapped by "not found" model errors.
var ErrNotExist = errors.New("resource does not exist")

// ErrInvalidArgument is wrapped by validation errors.
var ErrInvalidArgument = errors.New("invalid argument")
