// Package service contains business logic for EventHub use cases.
package service

import "errors"

var (
	// ErrInvalidField indicates an invalid request field value.
	ErrInvalidField = errors.New("invalid field")
	// ErrInvalidParameter indicates an invalid query parameter value.
	ErrInvalidParameter = errors.New("invalid parameter")
	// ErrAlreadyExists indicates entity conflict.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInvalidCredentials indicates login/password mismatch.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrNotFound indicates entity absence.
	ErrNotFound = errors.New("not found")
)
