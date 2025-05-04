package model

import (
	"github.com/govalues/decimal"
)

type Portfolio struct {
	TotalBalance decimal.Decimal
	Name         string
	TotalWeight       decimal.Decimal
	CurPage      int
	HasNextPage  bool
	Stocks       []Stock
}
