package service

import "errors"

var (
	ErrNotFound = errors.New("error not found")
	ErrStockNotActive = errors.New("error stock is not active")
)