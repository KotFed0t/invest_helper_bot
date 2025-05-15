package model

import (
	"github.com/shopspring/decimal"
)

type PortfolioPage struct {
	PortfolioSummary
	CurPage    int
	TotalPages int
	Stocks     []Stock
}

type PortfolioSummary struct {
	PortfolioName         string
	TotalBalance          decimal.Decimal
	TotalWeight           decimal.Decimal
	BalanceOutsideIndex   decimal.Decimal
	StocksCount           int
	StocksOutsideIndexCnt int
}

type Portfolio struct {
	ID   int64
	Name string
}
