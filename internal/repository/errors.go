// Package repository provides data persistence implementations.
package repository

import "errors"

var (
	// ErrAlreadyExists indicates a unique constraint conflict.
	ErrAlreadyExists = errors.New("already exists")
	// ErrNotFound indicates that a document was not found.
	ErrNotFound = errors.New("not found")
)
