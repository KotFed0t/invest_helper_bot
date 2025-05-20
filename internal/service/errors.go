package service

import "errors"

var (
	ErrNotFound = errors.New("error not found")
	ErrStockNotActive = errors.New("error stock is not active")
	ErrActualStockInfoUnavailable = errors.New("error actual stock info unavailable")
)