package model

import (
	"github.com/shopspring/decimal"
)

type Portfolio struct {
	TotalBalance decimal.Decimal
	Name         string
	TotalWeight  decimal.Decimal
	CurPage      int
	HasNextPage  bool
	Stocks       []Stock
}

type PortfolioSummary struct {
	TotalBalance        decimal.Decimal
	TotalWeight         decimal.Decimal
	BalanceOutsideIndex decimal.Decimal
}
