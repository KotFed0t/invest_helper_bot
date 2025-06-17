package repository

import "errors"

var (
	ErrAlreadyExists = errors.New("error already exists")
	ErrNotFound      = errors.New("error not found")
)
